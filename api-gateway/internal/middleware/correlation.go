package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/auction-system/api-gateway/internal/proxy"
)

const (
	// CorrelationIDKey is the Gin context key where the correlation ID is stored.
	// This value is read by the reverse proxy (proxy.doOnce) to propagate the ID
	// to all downstream service calls via the X-Correlation-ID header.
	// The pkg/kafka package also reads this to set the Kafka message header.
	CorrelationIDKey = "correlation_id"
)

// CorrelationMiddleware manages correlation ID generation and propagation.
// It ensures every request has a correlation ID that flows through the entire
// request lifecycle: HTTP response headers, downstream service calls, and Kafka messages.
type CorrelationMiddleware struct {
	headerName string
}

// NewCorrelationMiddleware creates a new CorrelationMiddleware instance.
func NewCorrelationMiddleware() *CorrelationMiddleware {
	return &CorrelationMiddleware{
		headerName: proxy.HeaderCorrelationID,
	}
}

// CorrelationID returns a Gin middleware that ensures every request has an
// X-Correlation-ID. If the incoming request already contains the header, the
// value is preserved without modification. If absent, a new UUID v4 is generated.
//
// The correlation ID is:
//   - Stored in the Gin context under CorrelationIDKey for use by handlers
//   - Set as a response header (X-Correlation-ID) for client visibility
//   - Propagated to downstream services via the reverse proxy (reads from context)
//   - Propagated to Kafka message headers via the pkg/kafka producer
func CorrelationID() gin.HandlerFunc {
	cm := NewCorrelationMiddleware()
	return cm.Handler()
}

// Handler returns the Gin handler function for this middleware.
func (cm *CorrelationMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader(cm.headerName)

		if correlationID == "" {
			// Generate a new UUID v4 when X-Correlation-ID header is absent.
			// Requirement 5.3: Generate correlation_id using UUID v4.
			correlationID = uuid.NewString()
		}
		// Requirement 5.4: Preserve and forward existing value without modification.
		// If the header was present, we use it as-is.

		// Store in Gin context so downstream handlers and the reverse proxy can access it.
		// The reverse proxy reads this key to set X-Correlation-ID on forwarded requests.
		c.Set(CorrelationIDKey, correlationID)

		// Set as response header so clients can correlate their requests.
		c.Header(cm.headerName, correlationID)

		c.Next()
	}
}
