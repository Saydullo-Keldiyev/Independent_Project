package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/auction-system/api-gateway/internal/proxy"
)

// CorrelationID ensures X-Correlation-ID on every request (tracing).
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		cid := c.GetHeader(proxy.HeaderCorrelationID)
		if cid == "" {
			cid = c.GetHeader("X-Request-ID")
		}
		if cid == "" {
			cid = uuid.NewString()
		}
		c.Set("correlation_id", cid)
		c.Header(proxy.HeaderCorrelationID, cid)
		c.Next()
	}
}
