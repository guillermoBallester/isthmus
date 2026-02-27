package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// callState holds per-request timing and span data.
type callState struct {
	start time.Time
	span  trace.Span
}

// ToolCallHooks creates MCP hooks that log tool calls and optionally record OTel spans/metrics.
func ToolCallHooks(logger *slog.Logger, tracer trace.Tracer, inst port.Instrumentation) *server.Hooks {
	hooks := &server.Hooks{}
	var calls sync.Map // id -> *callState

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, req *mcp.CallToolRequest) {
		state := &callState{start: time.Now()}

		if tracer != nil {
			_, span := tracer.Start(ctx, "mcp.tool.call",
				trace.WithAttributes(
					attribute.String("mcp.tool", req.Params.Name),
				),
			)
			state.span = span
		}

		calls.Store(id, state)
	})

	hooks.AddAfterCallTool(func(ctx context.Context, id any, req *mcp.CallToolRequest, result any) {
		var duration time.Duration
		var span trace.Span

		if v, ok := calls.LoadAndDelete(id); ok {
			state := v.(*callState)
			duration = time.Since(state.start)
			span = state.span
		}

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
			inst.RecordToolDuration(ctx, float64(duration.Milliseconds()))
		}

		if span != nil {
			if isErr {
				span.SetStatus(codes.Error, "tool returned error")
				span.RecordError(fmt.Errorf("tool %s returned error", req.Params.Name))
			}
			span.End()
		}
	})

	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		var duration time.Duration
		var span trace.Span

		if v, ok := calls.LoadAndDelete(id); ok {
			state := v.(*callState)
			duration = time.Since(state.start)
			span = state.span
		}

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

		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
		}
	})

	return hooks
}
