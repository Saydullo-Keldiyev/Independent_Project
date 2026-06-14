package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "bid-service"

// Tracer is the global tracer — use this to create spans
var Tracer trace.Tracer

// TracerConfig holds OpenTelemetry configuration
type TracerConfig struct {
	// OTLPEndpoint is the OTLP HTTP endpoint (e.g. "localhost:4318")
	// Jaeger supports OTLP natively since v1.35
	OTLPEndpoint string
	Environment  string
	Version      string
}

// InitTracing sets up OpenTelemetry with OTLP exporter (Jaeger-compatible).
// Returns a shutdown function — call it on service shutdown.
func InitTracing(cfg TracerConfig) (shutdown func(context.Context) error, err error) {
	// OTLP HTTP exporter — works with Jaeger, Grafana Tempo, etc.
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(), // use WithTLSClientConfig in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Resource describes this service to the tracing backend
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(cfg.Version),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracing resource: %w", err)
	}

	// Trace provider with batch exporter for performance
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerForEnv(cfg.Environment)),
	)

	// Register as global provider
	otel.SetTracerProvider(tp)

	// W3C TraceContext + Baggage propagation (works with Jaeger, Zipkin, etc.)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	Tracer = otel.Tracer(serviceName)

	return tp.Shutdown, nil
}

// samplerForEnv returns AlwaysSample in dev, 10% in production
func samplerForEnv(env string) sdktrace.Sampler {
	if env == "production" {
		return sdktrace.TraceIDRatioBased(0.1) // sample 10% in prod
	}
	return sdktrace.AlwaysSample()
}

// ── Span helpers ──────────────────────────────────────────────────────────────

// StartSpan creates a new span. Always defer span.End().
//
//	ctx, span := observability.StartSpan(ctx, "PlaceBid")
//	defer span.End()
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	ctx, span := Tracer.Start(ctx, name)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

// SpanFromCtx returns the active span from context (no-op span if none)
func SpanFromCtx(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// RecordError records an error on the current span and marks it as failed
func RecordError(span trace.Span, err error) {
	if err != nil && span != nil {
		span.RecordError(err)
		span.SetStatus(2, err.Error()) // codes.Error = 2
	}
}

// BidAttributes returns standard span attributes for a bid operation
func BidAttributes(auctionID, userID string, amount float64) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("auction.id", auctionID),
		attribute.String("user.id", userID),
		attribute.Float64("bid.amount", amount),
	}
}
