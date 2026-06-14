package observability

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	CorrelationIDHeader = "X-Correlation-ID"
	CorrelationIDKey    = "correlation_id"
)

// CorrelationMiddleware ensures every request has a correlation ID.
// If the client sends X-Correlation-ID, it is reused (for distributed tracing).
// Otherwise a new UUID is generated.
// The ID is:
//   - stored in gin context
//   - stored in request context (for service calls)
//   - echoed back in the response header
func CorrelationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Store in gin context
		c.Set(CorrelationIDKey, correlationID)

		// Store in request context so it flows into service/repo layers
		ctx := WithFields(
			c.Request.Context(),
			zap.String("correlation_id", correlationID),
		)
		c.Request = c.Request.WithContext(ctx)

		// Echo back to client
		c.Header(CorrelationIDHeader, correlationID)

		c.Next()
	}
}

// GetCorrelationID extracts the correlation ID from gin context
func GetCorrelationID(c *gin.Context) string {
	if id, exists := c.Get(CorrelationIDKey); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// CorrelationIDFromCtx extracts correlation ID from a plain context.Context
func CorrelationIDFromCtx(ctx context.Context) string {
	fields := fieldsFromCtx(ctx)
	for _, f := range fields {
		if f.Key == "correlation_id" {
			return f.String
		}
	}
	return ""
}
