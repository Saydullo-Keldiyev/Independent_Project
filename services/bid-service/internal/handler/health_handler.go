package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/bid-service/internal/database"
	redisPkg "github.com/auction-system/bid-service/internal/redis"
)

// Health handles GET /health — liveness probe.
// Returns 200 if the process is alive (no dependency checks).
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "bid-service",
	})
}

// Ready handles GET /ready — readiness probe.
// Returns 200 only if ALL dependencies are reachable.
// Kubernetes will stop sending traffic if this returns non-200.
func Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	allOK := true

	// Check PostgreSQL
	if database.DB != nil {
		if err := database.DB.Ping(ctx); err != nil {
			checks["postgres"] = "unhealthy: " + err.Error()
			allOK = false
		} else {
			checks["postgres"] = "ok"
		}
	} else {
		checks["postgres"] = "unhealthy: not initialized"
		allOK = false
	}

	// Check Redis
	if redisPkg.Client != nil {
		if err := redisPkg.Client.Ping(ctx).Err(); err != nil {
			checks["redis"] = "unhealthy: " + err.Error()
			allOK = false
		} else {
			checks["redis"] = "ok"
		}
	} else {
		checks["redis"] = "unhealthy: not initialized"
		allOK = false
	}

	status := http.StatusOK
	statusStr := "ready"
	if !allOK {
		status = http.StatusServiceUnavailable
		statusStr = "not ready"
	}

	c.JSON(status, gin.H{
		"status": statusStr,
		"checks": checks,
	})
}
