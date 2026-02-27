package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/guillermoBallester/isthmus/internal/core/domain"
	"github.com/guillermoBallester/isthmus/internal/core/port"
	"github.com/guillermoBallester/isthmus/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	validator port.QueryValidator
	executor  port.QueryExecutor
	auditor   port.QueryAuditor
	logger    *slog.Logger
	masks     map[string]string // column-name → mask-type (nil = no masking)
	tracer    trace.Tracer
	inst      *telemetry.Instruments
}

func NewQueryService(validator port.QueryValidator, executor port.QueryExecutor, auditor port.QueryAuditor, logger *slog.Logger) *QueryService {
	return &QueryService{
		validator: validator,
		executor:  executor,
		auditor:   auditor,
		logger:    logger,
		tracer:    telemetry.NoopTracer(),
		inst:      telemetry.NoopInstruments(),
	}
}

// SetMasks configures column-level masking rules (column-name → mask-type).
func (s *QueryService) SetMasks(masks map[string]string) {
	s.masks = masks
}

// SetTelemetry configures OTel tracing and metrics for the service.
func (s *QueryService) SetTelemetry(tracer trace.Tracer, inst *telemetry.Instruments) {
	if tracer != nil {
		s.tracer = tracer
	}
	if inst != nil {
		s.inst = inst
	}
}

// Execute validates the SQL statement and, if allowed, delegates to the executor.
func (s *QueryService) Execute(ctx context.Context, sql string) ([]map[string]any, error) {
	ctx, span := s.tracer.Start(ctx, "QueryService.Execute",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation.name", "query"),
			attribute.String("db.statement", sql),
		),
	)
	defer span.End()

	if err := s.validator.Validate(sql); err != nil {
		s.logger.WarnContext(ctx, "query validation rejected",
			slog.String("db.operation.name", "query"),
			slog.String("db.statement", sql),
			slog.String("error.type", "validation_error"),
		)
		span.RecordError(err)
		s.inst.QueryErrors.Add(ctx, 1)
		return nil, err
	}

	start := time.Now()
	results, err := s.executor.Execute(ctx, sql)
	durationMS := time.Since(start).Milliseconds()

	s.inst.QueryCount.Add(ctx, 1)
	s.inst.QueryDuration.Record(ctx, float64(durationMS))

	s.auditor.Record(ctx, port.AuditEntry{
		Tool:         toolNameFromCtx(ctx),
		SQL:          sql,
		RowsReturned: len(results),
		DurationMS:   durationMS,
		Err:          err,
	})

	if err != nil {
		span.RecordError(err)
		s.inst.QueryErrors.Add(ctx, 1)
		return results, err
	}

	span.SetAttributes(attribute.Int("db.response.rows", len(results)))
	domain.MaskRows(results, s.masks)

	return results, nil
}
