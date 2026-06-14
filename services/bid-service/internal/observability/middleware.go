package observability

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel"
)

// ObservabilityMiddleware combines:
//   - Structured request logging (zap)
//   - Prometheus HTTP metrics
//   - OpenTelemetry distributed tracing
//   - Correlation ID propagation
//
// Apply this as the FIRST middleware after gin.Recovery().
func ObservabilityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		// ── 1. Extract or generate correlation ID ─────────────────────────
		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = generateID()
		}
		c.Set(CorrelationIDKey, correlationID)
		c.Header(CorrelationIDHeader, correlationID)

		// ── 2. Extract trace context from incoming headers (W3C TraceContext)
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)

		// ── 3. Start a new span for this request ──────────────────────────
		spanName := fmt.Sprintf("%s %s", method, path)
		ctx, span := StartSpan(ctx, spanName,
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("correlation_id", correlationID),
		)
		defer span.End()

		// ── 4. Enrich context with log fields ─────────────────────────────
		ctx = WithFields(ctx,
			zap.String("correlation_id", correlationID),
			zap.String("method", method),
			zap.String("path", path),
		)
		c.Request = c.Request.WithContext(ctx)

		// ── 5. Process request ────────────────────────────────────────────
		c.Next()

		// ── 6. Record metrics & logs after handler returns ────────────────
		duration := time.Since(start)
		status := c.Writer.Status()
		statusStr := fmt.Sprintf("%d", status)

		// Prometheus
		HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

		// Span attributes
		span.SetAttributes(
			attribute.Int("http.status_code", status),
			attribute.Float64("duration_ms", float64(duration.Milliseconds())),
		)
		if status >= 500 {
			span.SetStatus(2, "server error")
		}

		// Structured log
		logger := FromCtx(ctx)
		logFn := logger.Info
		if status >= 500 {
			logFn = logger.Error
		} else if status >= 400 {
			logFn = logger.Warn
		}

		logFn("http request",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("client_ip", c.ClientIP()),
			zap.String("correlation_id", correlationID),
		)
	}
}

// generateID creates a short unique ID for correlation
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
