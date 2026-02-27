package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNoopTracer(t *testing.T) {
	tracer := NoopTracer()
	assert.NotNil(t, tracer)

	_, span := tracer.Start(context.Background(), "test")
	assert.NotNil(t, span)
	span.End()
}

func TestNoopInstruments(t *testing.T) {
	inst := NoopInstruments()
	assert.NotNil(t, inst)
	assert.NotNil(t, inst.QueryCount)
	assert.NotNil(t, inst.QueryDuration)
	assert.NotNil(t, inst.QueryErrors)
	assert.NotNil(t, inst.ToolDuration)

	// Should not panic.
	inst.QueryCount.Add(context.Background(), 1)
	inst.QueryDuration.Record(context.Background(), 100.0)
}

func TestProvider_Shutdown_Nil(t *testing.T) {
	var p *Provider
	err := p.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestSpanRecording(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	ctx := context.Background()
	_, span := tracer.Start(ctx, "test-op")
	span.SetAttributes(attribute.String("db.system", "postgresql"))
	span.End()

	require.NoError(t, tp.ForceFlush(ctx))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-op", spans[0].Name)
}

func TestMetricRecording(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")

	counter, err := meter.Int64Counter("test.counter")
	require.NoError(t, err)

	counter.Add(context.Background(), 5)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	require.Len(t, rm.ScopeMetrics, 1)
	require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
	assert.Equal(t, "test.counter", rm.ScopeMetrics[0].Metrics[0].Name)
}
