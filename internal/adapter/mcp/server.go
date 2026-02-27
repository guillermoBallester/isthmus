package mcp

import (
	"log/slog"

	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"
	"github.com/guillermoBallester/isthmus/internal/telemetry"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// NewServer creates an MCPServer with tools and logging hooks.
func NewServer(version string, explorer port.SchemaExplorer, profiler port.SchemaProfiler, query *service.QueryService, logger *slog.Logger, tracer trace.Tracer, inst *telemetry.Instruments) *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		version,
		server.WithHooks(ToolCallHooks(logger, tracer, inst)),
	)

	RegisterTools(s, explorer, profiler, query, logger)

	return s
}
