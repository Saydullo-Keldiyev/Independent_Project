package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/auction-service/internal/observability"
)

func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		st := strconv.Itoa(c.Writer.Status())
		observability.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, st).Inc()
		observability.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
