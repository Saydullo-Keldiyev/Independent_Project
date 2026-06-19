package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/auction-system/api-gateway/internal/observability"
	"github.com/auction-system/pkg/ratelimit"
)

// SlidingWindowRateLimiter integrates pkg/ratelimit into the Gin middleware chain.
// It provides sliding window rate limiting with IP blocking, endpoint-specific limits,
// and automatic fallback to in-memory limiting when Redis is unavailable.
type SlidingWindowRateLimiter struct {
	limiter        ratelimit.RateLimiter
	config         ratelimit.RateLimitConfig
	usingFallback  bool
	logger         *zap.Logger
}

// NewSlidingWindowRateLimiter creates a new sliding window rate limiter middleware.
// If the Redis client is nil or unavailable, it falls back to in-memory limiting
// and logs a warning.
func NewSlidingWindowRateLimiter(redisClient *redis.Client, logger *zap.Logger) *SlidingWindowRateLimiter {
	cfg := ratelimit.DefaultConfig()

	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	var limiter ratelimit.RateLimiter
	usingFallback := false

	if redisClient == nil {
		logger.Warn("Redis client is nil, falling back to in-memory rate limiting",
			zap.String("component", "rate_limiter"),
		)
		limiter = ratelimit.NewInMemoryLimiter(cfg)
		usingFallback = true
	} else {
		limiter = ratelimit.NewRedisRateLimiter(redisClient, cfg)
	}

	return &SlidingWindowRateLimiter{
		limiter:       limiter,
		config:        cfg,
		usingFallback: usingFallback,
		logger:        logger,
	}
}

// Middleware returns a Gin middleware handler that performs comprehensive rate limiting.
// It checks:
// 1. Whether the IP is blocked (returns 403)
// 2. Whether the request is authenticated (JWT presence in context)
// 3. Endpoint-specific limits for /login, /register, /bids
// 4. Global limits (200/min authenticated, 30/min anonymous)
// Returns HTTP 429 with Retry-After header when rate limited.
// Returns HTTP 403 with block reason and remaining duration for blocked IPs.
func (s *SlidingWindowRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// Determine if request is authenticated (JWT was parsed by auth middleware)
		authenticated := c.GetString("user_id") != ""

		// Extract endpoint path for endpoint-specific limits
		endpoint := extractEndpoint(c.Request.URL.Path)

		// Perform comprehensive rate limit check
		result := s.limiter.Check(c.Request.Context(), ip, authenticated, endpoint)

		// Set rate limit headers for all responses
		effectiveLimit := ratelimit.EffectiveLimit(s.config, authenticated, endpoint)
		c.Header("X-RateLimit-Limit", strconv.Itoa(effectiveLimit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))

		if result.Blocked {
			// IP is blocked — return 403 Forbidden
			observability.RateLimitHits.WithLabelValues("blocked").Inc()

			remainingSec := int(result.BlockRemaining.Seconds())
			if remainingSec < 1 {
				remainingSec = 1
			}

			c.Header("Retry-After", strconv.Itoa(remainingSec))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":               "IP_BLOCKED",
					"message":            result.BlockReason,
					"remaining_duration": fmt.Sprintf("%ds", remainingSec),
				},
			})
			return
		}

		if !result.Allowed {
			// Rate limit exceeded — return 429 Too Many Requests
			scope := "anonymous"
			if authenticated {
				scope = "authenticated"
			}
			if endpoint != "" {
				scope = "endpoint:" + endpoint
			}
			observability.RateLimitHits.WithLabelValues(scope).Inc()

			retryAfterSec := int(result.RetryAfter.Seconds())
			if retryAfterSec < 1 {
				retryAfterSec = 1
			}

			c.Header("Retry-After", strconv.Itoa(retryAfterSec))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests. Please retry after the specified duration.",
				},
				"retry_after": retryAfterSec,
			})
			return
		}

		c.Next()
	}
}

// UsingFallback returns true if the rate limiter is using in-memory fallback.
func (s *SlidingWindowRateLimiter) UsingFallback() bool {
	return s.usingFallback
}

// extractEndpoint maps request paths to endpoint-specific rate limit keys.
// It matches known endpoints that have specific limits: /login, /register, /bids.
func extractEndpoint(path string) string {
	// Normalize the path - strip API version prefix if present
	normalized := path
	if strings.HasPrefix(path, "/api/v") {
		// Strip /api/vN/ prefix
		parts := strings.SplitN(path, "/", 5) // ["", "api", "vN", "rest..."]
		if len(parts) >= 4 {
			normalized = "/" + strings.Join(parts[3:], "/")
		}
	}

	// Check for endpoint-specific paths
	switch {
	case strings.HasSuffix(normalized, "/login") || normalized == "/login":
		return "/login"
	case strings.HasSuffix(normalized, "/register") || normalized == "/register":
		return "/register"
	case normalized == "/bids" || strings.HasPrefix(normalized, "/bids"):
		// Only match POST /bids (bid placement), not GET /bids/me
		// The endpoint limit applies to the /bids path for bid placement
		if normalized == "/bids" {
			return "/bids"
		}
	}

	return ""
}

// ──────────────────────────────────────────────────────────────────────────────
// Legacy support: keep NewRateLimiter for backward compatibility during migration.
// Deprecated: Use NewSlidingWindowRateLimiter instead.
// ──────────────────────────────────────────────────────────────────────────────

// RateLimiter is the legacy rate limiter. Deprecated: use SlidingWindowRateLimiter.
type RateLimiter struct {
	sliding *SlidingWindowRateLimiter
}

// NewRateLimiter creates the legacy rate limiter, now backed by the sliding window implementation.
// Deprecated: Use NewSlidingWindowRateLimiter directly.
func NewRateLimiter(client *redis.Client, perMinute int) *RateLimiter {
	logger, _ := zap.NewProduction()
	return &RateLimiter{
		sliding: NewSlidingWindowRateLimiter(client, logger),
	}
}

// Middleware returns the rate limiting middleware. The scope parameter is ignored
// in the new implementation as authentication state is auto-detected.
func (rl *RateLimiter) Middleware(scope string) gin.HandlerFunc {
	return rl.sliding.Middleware()
}
