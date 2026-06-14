package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/auction-system/api-gateway/internal/observability"
)

// RateLimiter — Redis INCR per minute (100 req/min default).
type RateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func NewRateLimiter(client *redis.Client, perMinute int) *RateLimiter {
	if perMinute <= 0 {
		perMinute = 100
	}
	return &RateLimiter{client: client, limit: perMinute, window: time.Minute}
}

func (rl *RateLimiter) Middleware(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.client == nil {
			c.Next()
			return
		}

		keyID := c.ClientIP()
		if scope == "user" {
			if uid := c.GetString("user_id"); uid != "" {
				keyID = uid
			}
		}
		key := fmt.Sprintf("gw:rl:%s:%s", scope, keyID)

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		n, err := rl.client.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if n == 1 {
			rl.client.Expire(ctx, key, rl.window)
		}

		remaining := int64(rl.limit) - n
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(max0(remaining), 10))

		if n > int64(rl.limit) {
			observability.RateLimitHits.WithLabelValues(scope).Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "too many requests",
			})
			return
		}
		c.Next()
	}
}

func max0(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
