package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/guillermoBallester/isthmus/internal/adapter/mcp"
	"github.com/guillermoBallester/isthmus/internal/adapter/postgres"
	"github.com/guillermoBallester/isthmus/internal/audit"
	"github.com/guillermoBallester/isthmus/internal/config"
	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"
	"github.com/guillermoBallester/isthmus/internal/policy"
	"github.com/jackc/pgx/v5/pgxpool"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	overrides, err := parseFlags(os.Args[1:])
	if err != nil {
		return err
	}

	cfg, err := config.Load(overrides)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := newLogger(cfg)
	logger.Info("starting isthmus",
		slog.String("version", version),
		slog.String("log_level", cfg.LogLevel.String()),
		slog.Bool("read_only", cfg.ReadOnly),
		slog.Int("max_rows", cfg.MaxRows),
		slog.String("query_timeout", cfg.QueryTimeout.String()),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pool, err := connectDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("database pool connected", slog.String("db.system", "postgresql"))

	if cfg.DryRun {
		printResolvedConfig(cfg)
		return nil
	}

	explorer, err := buildExplorer(pool, cfg, logger)
	if err != nil {
		return err
	}
	executor := buildExecutor(pool, cfg, logger)
	profiler := postgres.NewProfiler(pool, cfg.Schemas)

	auditor, closeAuditor, err := buildAuditor(cfg, logger)
	if err != nil {
		return err
	}
	defer closeAuditor()

	return serve(ctx, version, explorer, executor, profiler, auditor, logger)
}

func newLogger(cfg *config.Config) *slog.Logger {
	// Logs go to stderr — stdout is reserved for the MCP stdio transport.
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
}

func connectDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return pool, nil
}

func buildExplorer(pool *pgxpool.Pool, cfg *config.Config, logger *slog.Logger) (port.SchemaExplorer, error) {
	var explorer port.SchemaExplorer = postgres.NewExplorer(pool, cfg.Schemas)

	if cfg.PolicyFile != "" {
		pol, err := policy.LoadFromFile(cfg.PolicyFile)
		if err != nil {
			return nil, fmt.Errorf("loading policy: %w", err)
		}
		explorer = policy.NewPolicyExplorer(explorer, pol)
		logger.Info("policy loaded", slog.String("file", cfg.PolicyFile))
	}

	return explorer, nil
}

func buildExecutor(pool *pgxpool.Pool, cfg *config.Config, logger *slog.Logger) port.QueryExecutor {
	var executor port.QueryExecutor = postgres.NewExecutor(pool, cfg.ReadOnly, cfg.MaxRows, cfg.QueryTimeout)

	if cfg.ExplainOnly {
		executor = postgres.NewExplainOnlyExecutor(executor)
		logger.Info("explain-only mode enabled")
	}

	return executor
}

func buildAuditor(cfg *config.Config, logger *slog.Logger) (port.QueryAuditor, func(), error) {
	if cfg.AuditLog == "" {
		return audit.NoopAuditor{}, func() {}, nil
	}

	fa, err := audit.NewFileAuditor(cfg.AuditLog)
	if err != nil {
		return nil, nil, fmt.Errorf("opening audit log %q: %w", cfg.AuditLog, err)
	}
	logger.Info("audit logging enabled", slog.String("file", cfg.AuditLog))

	closeFn := func() {
		if err := fa.Close(); err != nil {
			logger.Error("closing audit log", slog.String("error", err.Error()))
		}
	}

	return fa, closeFn, nil
}

func serve(ctx context.Context, ver string, explorer port.SchemaExplorer, executor port.QueryExecutor, profiler port.SchemaProfiler, auditor port.QueryAuditor, logger *slog.Logger) error {
	validator := domain.NewPgQueryValidator()

	explorerSvc := service.NewExplorerService(explorer)
	profilerSvc := service.NewProfilerService(profiler)
	querySvc := service.NewQueryService(validator, executor, auditor, logger)

	mcpServer := mcp.NewServer(ver, explorerSvc, profilerSvc, querySvc, logger)
	stdioServer := mcpserver.NewStdioServer(mcpServer)

	logger.Info("serving MCP over stdio")
	if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil {
		return fmt.Errorf("stdio server: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}

func parseFlags(args []string) (config.Overrides, error) {
	fs := flag.NewFlagSet("isthmus", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	showVersion := fs.Bool("version", false, "Print version and exit")
	dryRun := fs.Bool("dry-run", false, "Validate config, connect to DB, ping, then exit")
	explainOnly := fs.Bool("explain-only", false, "Force all query calls to return EXPLAIN plans")
	auditLog := fs.String("audit-log", "", "Path to NDJSON file for query audit logging")
	databaseURL := fs.String("database-url", "", "PostgreSQL connection string (overrides DATABASE_URL env)")
	logLevel := fs.String("log-level", "", "Log level: debug, info, warn, error (overrides LOG_LEVEL env)")
	maxRows := fs.Int("max-rows", 0, "Maximum rows returned per query (overrides MAX_ROWS env)")
	queryTimeout := fs.Duration("query-timeout", 0, "Query timeout duration, e.g. 30s (overrides QUERY_TIMEOUT env)")
	policyFile := fs.String("policy-file", "", "Path to policy YAML file (overrides POLICY_FILE env)")

	if err := fs.Parse(args); err != nil {
		return config.Overrides{}, err
	}

	if *showVersion {
		fmt.Fprintf(os.Stderr, "isthmus %s\n", version)
		os.Exit(0)
	}

	var overrides config.Overrides
	overrides.DryRun = *dryRun
	overrides.ExplainOnly = *explainOnly
	overrides.AuditLog = *auditLog

	if *databaseURL != "" {
		overrides.DatabaseURL = databaseURL
	}
	if *logLevel != "" {
		overrides.LogLevel = logLevel
	}
	if *maxRows != 0 {
		overrides.MaxRows = maxRows
	}
	if *queryTimeout != 0 {
		overrides.QueryTimeout = queryTimeout
	}
	if *policyFile != "" {
		overrides.PolicyFile = policyFile
	}

	return overrides, nil
}

// printResolvedConfig prints the resolved configuration to stderr with redacted DSN.
func printResolvedConfig(cfg *config.Config) {
	fmt.Fprintf(os.Stderr, "dry-run: config OK, database reachable\n")
	fmt.Fprintf(os.Stderr, "  database_url:  %s\n", redactDSN(cfg.DatabaseURL))
	fmt.Fprintf(os.Stderr, "  read_only:     %t\n", cfg.ReadOnly)
	fmt.Fprintf(os.Stderr, "  max_rows:      %d\n", cfg.MaxRows)
	fmt.Fprintf(os.Stderr, "  query_timeout: %s\n", cfg.QueryTimeout)
	fmt.Fprintf(os.Stderr, "  log_level:     %s\n", cfg.LogLevel)
	if cfg.PolicyFile != "" {
		fmt.Fprintf(os.Stderr, "  policy_file:   %s\n", cfg.PolicyFile)
	}
	if len(cfg.Schemas) > 0 {
		fmt.Fprintf(os.Stderr, "  schemas:       %v\n", cfg.Schemas)
	}
}

// redactDSN replaces the password in a PostgreSQL DSN with "***".
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "***"
	}
	if _, has := u.User.Password(); has {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}
