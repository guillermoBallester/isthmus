package port

import "context"

// Instrumentation records application-level metrics.
type Instrumentation interface {
	RecordQueryDuration(ctx context.Context, ms float64)
	IncrementQueryCount(ctx context.Context)
	IncrementQueryErrors(ctx context.Context)
	RecordToolDuration(ctx context.Context, ms float64)
}

// NoopInstrumentation discards all metrics.
type NoopInstrumentation struct{}

func (NoopInstrumentation) RecordQueryDuration(context.Context, float64) {}
func (NoopInstrumentation) IncrementQueryCount(context.Context)          {}
func (NoopInstrumentation) IncrementQueryErrors(context.Context)         {}
func (NoopInstrumentation) RecordToolDuration(context.Context, float64)  {}
