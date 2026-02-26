package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
)

type toolNameKey struct{}

// WithToolName returns a context carrying the MCP tool name for audit logging.
func WithToolName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, toolNameKey{}, name)
}

func toolNameFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(toolNameKey{}).(string); ok {
		return v
	}
	return ""
}

// QueryService orchestrates SQL validation (domain) and execution (infrastructure).
type QueryService struct {
	validator *domain.QueryValidator
	executor  port.QueryExecutor
	auditor   port.QueryAuditor
	logger    *slog.Logger
}

func NewQueryService(validator *domain.QueryValidator, executor port.QueryExecutor, auditor port.QueryAuditor, logger *slog.Logger) *QueryService {
	return &QueryService{
		validator: validator,
		executor:  executor,
		auditor:   auditor,
		logger:    logger,
	}
}

// Execute validates the SQL statement and, if allowed, delegates to the executor.
func (s *QueryService) Execute(ctx context.Context, sql string) ([]map[string]any, error) {
	if err := s.validator.Validate(sql); err != nil {
		s.logger.WarnContext(ctx, "query validation rejected",
			slog.String("db.operation.name", "query"),
			slog.String("db.statement", sql),
			slog.String("error.type", "validation_error"),
		)
		return nil, err
	}

	start := time.Now()
	results, err := s.executor.Execute(ctx, sql)
	durationMS := time.Since(start).Milliseconds()

	s.auditor.Record(ctx, port.AuditEntry{
		Tool:         toolNameFromCtx(ctx),
		SQL:          sql,
		RowsReturned: len(results),
		DurationMS:   durationMS,
		Err:          err,
	})

	return results, err
}
