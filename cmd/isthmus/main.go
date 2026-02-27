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
	"time"

	"github.com/guillermoBallester/isthmus/internal/adapter/mcp"
	"github.com/guillermoBallester/isthmus/internal/adapter/policy"
	"github.com/guillermoBallester/isthmus/internal/adapter/postgres"
	"github.com/guillermoBallester/isthmus/internal/audit"
	"github.com/guillermoBallester/isthmus/internal/config"
	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"
	"github.com/guillermoBallester/isthmus/internal/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel"
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
		slog.String("transport", cfg.Transport),
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

	explorer, masks, err := buildExplorer(pool, cfg, logger)
	if err != nil {
		return err
	}
	executor := buildExecutor(pool, cfg, logger)

	var profiler port.SchemaProfiler = postgres.NewProfiler(pool, cfg.Schemas)
	if len(masks) > 0 {
		profiler = policy.NewMaskingProfiler(profiler, masks)
	}

	auditor, closeAuditor, err := buildAuditor(cfg, logger)
	if err != nil {
		return err
	}
	defer closeAuditor()

	var otelProvider *telemetry.Provider
	if cfg.OTelEnabled {
		otelProvider, err = telemetry.Init(ctx, "isthmus", version)
		if err != nil {
			return fmt.Errorf("initializing otel: %w", err)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otelProvider.Shutdown(shutdownCtx); err != nil {
				logger.Error("shutting down otel", slog.String("error", err.Error()))
			}
		}()
		logger.Info("opentelemetry enabled")
	}

	return serve(ctx, cfg, version, explorer, executor, profiler, masks, auditor, logger)
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

func buildExplorer(pool *pgxpool.Pool, cfg *config.Config, logger *slog.Logger) (port.SchemaExplorer, map[string]domain.MaskType, error) {
	var explorer port.SchemaExplorer = postgres.NewExplorer(pool, cfg.Schemas)
	var masks map[string]domain.MaskType

	if cfg.PolicyFile != "" {
		pol, err := policy.LoadFromFile(cfg.PolicyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("loading policy: %w", err)
		}
		explorer = policy.NewPolicyExplorer(explorer, pol)
		masks = policy.MaskSpec(pol.Context)
		logger.Info("policy loaded", slog.String("file", cfg.PolicyFile))
		if len(masks) > 0 {
			logger.Info("column masking enabled", slog.Int("masked_columns", len(masks)))
		}
	}

	return explorer, masks, nil
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

func serve(ctx context.Context, cfg *config.Config, ver string, explorer port.SchemaExplorer, executor port.QueryExecutor, profiler port.SchemaProfiler, masks map[string]domain.MaskType, auditor port.QueryAuditor, logger *slog.Logger) error {
	var tracer = telemetry.NoopTracer()
	var inst = telemetry.NoopInstruments()
	if cfg.OTelEnabled {
		tracer = otel.Tracer("github.com/guillermoBallester/isthmus")
		inst = telemetry.NewInstruments()
	}

	validator := domain.NewPgQueryValidator()
	querySvc := service.NewQueryService(validator, executor, auditor, logger, masks, tracer, inst)

	mcpServer := mcp.NewServer(ver, explorer, profiler, querySvc, logger, tracer, inst)

	switch cfg.Transport {
	case "http":
		return serveHTTP(ctx, mcpServer, cfg.HTTPAddr, logger)
	default:
		return serveStdio(ctx, mcpServer, logger)
	}
}

func serveStdio(ctx context.Context, mcpServer *mcpserver.MCPServer, logger *slog.Logger) error {
	stdioServer := mcpserver.NewStdioServer(mcpServer)

	logger.Info("serving MCP over stdio")
	if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil {
		return fmt.Errorf("stdio server: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}

func serveHTTP(ctx context.Context, mcpServer *mcpserver.MCPServer, addr string, logger *slog.Logger) error {
	httpServer := mcpserver.NewStreamableHTTPServer(mcpServer)

	logger.Info("serving MCP over HTTP", slog.String("addr", addr))

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Start(addr); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
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
	transport := fs.String("transport", "", "Transport: stdio or http (overrides TRANSPORT env)")
	httpAddr := fs.String("http-addr", "", "HTTP listen address, e.g. :8080 (overrides HTTP_ADDR env)")
	otel := fs.Bool("otel", false, "Enable OpenTelemetry tracing and metrics")

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
	if *transport != "" {
		overrides.Transport = transport
	}
	if *httpAddr != "" {
		overrides.HTTPAddr = httpAddr
	}
	overrides.OTelEnabled = *otel

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
	fmt.Fprintf(os.Stderr, "  transport:     %s\n", cfg.Transport)
	if cfg.Transport == "http" {
		fmt.Fprintf(os.Stderr, "  http_addr:     %s\n", cfg.HTTPAddr)
	}
	if cfg.PolicyFile != "" {
		fmt.Fprintf(os.Stderr, "  policy_file:   %s\n", cfg.PolicyFile)
	}
	if len(cfg.Schemas) > 0 {
		fmt.Fprintf(os.Stderr, "  schemas:       %v\n", cfg.Schemas)
	}
	if cfg.OTelEnabled {
		fmt.Fprintf(os.Stderr, "  otel:          enabled\n")
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
