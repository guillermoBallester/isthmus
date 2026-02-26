package mcp

import (
	"log/slog"

	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/core/service"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates an MCPServer with tools and logging hooks.
func NewServer(version string, explorer port.SchemaExplorer, profiler port.SchemaProfiler, query *service.QueryService, logger *slog.Logger) *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		version,
		server.WithHooks(ToolCallHooks(logger)),
	)

	RegisterTools(s, explorer, profiler, query)

	return s
}
