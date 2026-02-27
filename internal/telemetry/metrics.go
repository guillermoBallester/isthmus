package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

const meterName = "github.com/guillermoBallester/isthmus"

// Instruments holds pre-created OTel metric instruments.
type Instruments struct {
	QueryCount    metric.Int64Counter
	QueryDuration metric.Float64Histogram
	QueryErrors   metric.Int64Counter
	ToolDuration  metric.Float64Histogram
}

// NewInstruments creates metric instruments from the global MeterProvider.
// Returns nil-safe instruments: if creation fails, noop instruments are used.
func NewInstruments() *Instruments {
	meter := otel.Meter(meterName)
	return newInstrumentsFromMeter(meter)
}

// NoopInstruments returns instruments that record nothing.
func NoopInstruments() *Instruments {
	meter := noop.NewMeterProvider().Meter(meterName)
	return newInstrumentsFromMeter(meter)
}

func newInstrumentsFromMeter(meter metric.Meter) *Instruments {
	// OTel SDK returns noop instruments on error; safe to discard.
	queryCount, _ := meter.Int64Counter("isthmus.query.count",
		metric.WithDescription("Total number of SQL queries executed"),
	)
	queryDuration, _ := meter.Float64Histogram("isthmus.query.duration",
		metric.WithDescription("SQL query execution duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	queryErrors, _ := meter.Int64Counter("isthmus.query.errors",
		metric.WithDescription("Total number of failed SQL queries"),
	)
	toolDuration, _ := meter.Float64Histogram("isthmus.tool.duration",
		metric.WithDescription("MCP tool call duration in milliseconds"),
		metric.WithUnit("ms"),
	)

	return &Instruments{
		QueryCount:    queryCount,
		QueryDuration: queryDuration,
		QueryErrors:   queryErrors,
		ToolDuration:  toolDuration,
	}
}
