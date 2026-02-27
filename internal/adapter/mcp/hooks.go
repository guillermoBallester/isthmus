package mcp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/guillermoBallester/isthmus/internal/telemetry"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ToolCallHooks creates MCP hooks that log tool calls and optionally record OTel spans/metrics.
func ToolCallHooks(logger *slog.Logger, tracer trace.Tracer, inst *telemetry.Instruments) *server.Hooks {
	hooks := &server.Hooks{}
	var starts sync.Map
	var spans sync.Map

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, req *mcp.CallToolRequest) {
		starts.Store(id, time.Now())

		if tracer != nil {
			_, span := tracer.Start(ctx, "mcp.tool."+req.Params.Name,
				trace.WithAttributes(
					attribute.String("mcp.tool", req.Params.Name),
				),
			)
			spans.Store(id, span)
		}
	})

	hooks.AddAfterCallTool(func(ctx context.Context, id any, req *mcp.CallToolRequest, result any) {
		duration := sinceStart(&starts, id)
		level := slog.LevelInfo
		isErr := false

		if r, ok := result.(*mcp.CallToolResult); ok && r.IsError {
			level = slog.LevelError
			isErr = true
		}

		logger.LogAttrs(ctx, level, "tool call",
			slog.String("rpc.method", "tools/call"),
			slog.String("mcp.tool", req.Params.Name),
			slog.Duration("duration", duration),
			slog.Bool("error", isErr),
		)

		if inst != nil {
			inst.ToolDuration.Record(ctx, float64(duration.Milliseconds()))
		}

		if span, ok := spans.LoadAndDelete(id); ok {
			s := span.(trace.Span)
			if isErr {
				s.SetAttributes(attribute.Bool("error", true))
			}
			s.End()
		}
	})

	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		duration := sinceStart(&starts, id)
		toolName := ""
		if req, ok := message.(*mcp.CallToolRequest); ok {
			toolName = req.Params.Name
		}
		if toolName != "" {
			logger.LogAttrs(ctx, slog.LevelError, "tool call",
				slog.String("rpc.method", "tools/call"),
				slog.String("mcp.tool", toolName),
				slog.Duration("duration", duration),
				slog.Bool("error", true),
				slog.String("error.message", err.Error()),
			)
		}

		if span, ok := spans.LoadAndDelete(id); ok {
			s := span.(trace.Span)
			s.RecordError(err)
			s.End()
		}
	})

	return hooks
}

func sinceStart(starts *sync.Map, id any) time.Duration {
	if v, ok := starts.LoadAndDelete(id); ok {
		return time.Since(v.(time.Time))
	}
	return 0
}
