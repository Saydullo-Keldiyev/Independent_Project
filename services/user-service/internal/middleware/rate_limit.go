package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/user-service/internal/redis"
	"github.com/auction-system/user-service/internal/utils"
)

func RateLimit(requests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if redis.Client == nil {
			c.Next()
			return
		}
		key := fmt.Sprintf("ratelimit:%s:%s", c.FullPath(), c.ClientIP())
		ctx := context.Background()
		n, err := redis.Client.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if n == 1 {
			redis.Client.Expire(ctx, key, window)
		}
		if int(n) > requests {
			utils.Fail(c, 429, "RATE_LIMITED", "too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}
