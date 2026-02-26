package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/guillermoBallester/isthmus/internal/adapter/mcp"
	"github.com/guillermoBallester/isthmus/internal/adapter/postgres"
	"github.com/guillermoBallester/isthmus/internal/config"
	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"
	"github.com/guillermoBallester/isthmus/internal/policy"
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Logs go to stderr — stdout is reserved for the MCP stdio transport.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	logger.Info("starting isthmus",
		slog.String("version", version),
		slog.String("log_level", cfg.LogLevel.String()),
		slog.Bool("read_only", cfg.ReadOnly),
		slog.Int("max_rows", cfg.MaxRows),
		slog.String("query_timeout", cfg.QueryTimeout.String()),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	logger.Info("database pool connected", slog.String("db.system", "postgresql"))

	// Adapters
	var explorer port.SchemaExplorer = postgres.NewExplorer(pool, cfg.Schemas)
	executor := postgres.NewExecutor(pool, cfg.ReadOnly, cfg.MaxRows, cfg.QueryTimeout)

	// Policy decorator (optional).
	if cfg.PolicyFile != "" {
		pol, err := policy.LoadFromFile(cfg.PolicyFile)
		if err != nil {
			return fmt.Errorf("loading policy: %w", err)
		}
		explorer = policy.NewPolicyExplorer(explorer, pol)
		logger.Info("policy loaded", slog.String("file", cfg.PolicyFile))
	}

	// Domain
	validator := domain.NewQueryValidator()

	// Services
	explorerSvc := service.NewExplorerService(explorer)
	querySvc := service.NewQueryService(validator, executor, logger)

	// MCP server with tool handlers.
	mcpServer := mcp.NewServer(version, explorerSvc, querySvc, logger)

	// Run MCP over stdio (stdin/stdout).
	stdioServer := mcpserver.NewStdioServer(mcpServer)

	logger.Info("serving MCP over stdio")
	if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil {
		return fmt.Errorf("stdio server: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}
