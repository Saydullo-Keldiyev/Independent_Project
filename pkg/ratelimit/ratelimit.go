// Package ratelimit provides a sliding window rate limiter with IP blocking
// and automatic fallback to in-memory limiting when Redis is unavailable.
package ratelimit

import (
	"context"
	"time"
)

// RateLimitConfig holds the global rate limiting configuration.
type RateLimitConfig struct {
	// AuthenticatedLimit is the max requests per window for authenticated users.
	AuthenticatedLimit int // default: 200 req/min
	// AnonymousLimit is the max requests per window for anonymous users.
	AnonymousLimit int // default: 30 req/min
	// Window is the sliding window duration.
	Window time.Duration // default: 1 minute
	// BlockThreshold is how many violations trigger an IP block.
	BlockThreshold int // default: 10 violations within ViolationWindow
	// BlockDuration is how long a blocked IP stays blocked.
	BlockDuration time.Duration // default: 15 minutes
	// ViolationWindow is the time window for counting violations.
	ViolationWindow time.Duration // default: 1 hour
	// EndpointLimits are per-endpoint rate limits.
	EndpointLimits []EndpointLimit
}

// EndpointLimit defines a rate limit for a specific endpoint path.
type EndpointLimit struct {
	Path  string
	Limit int // requests per window
}

// DefaultConfig returns a RateLimitConfig with production defaults.
func DefaultConfig() RateLimitConfig {
	return RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     10,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
		EndpointLimits: []EndpointLimit{
			{Path: "/login", Limit: 5},
			{Path: "/register", Limit: 3},
			{Path: "/bids", Limit: 30},
		},
	}
}

// Result contains the outcome of a rate limit check.
type Result struct {
	// Allowed indicates if the request is permitted.
	Allowed bool
	// Remaining is the number of requests left in the current window.
	Remaining int
	// RetryAfter is the duration to wait before retrying (non-zero when blocked).
	RetryAfter time.Duration
	// Blocked indicates if the IP is currently blocked due to violations.
	Blocked bool
	// BlockReason provides a human-readable reason for the block.
	BlockReason string
	// BlockRemaining is how long the block lasts.
	BlockRemaining time.Duration
}

// RateLimiter is the interface for the sliding window rate limiter.
type RateLimiter interface {
	// Allow checks whether a request identified by key is within the rate limit.
	// limit is the effective limit to apply.
	Allow(ctx context.Context, key string, limit int) (allowed bool, remaining int, retryAfter time.Duration, err error)

	// IsBlocked checks if an IP is currently blocked due to excessive violations.
	IsBlocked(ctx context.Context, ip string) (blocked bool, remainingDuration time.Duration)

	// RecordViolation records a rate limit violation for an IP address.
	// If violations exceed the threshold, the IP will be blocked.
	RecordViolation(ctx context.Context, ip string) error

	// Check performs a comprehensive rate limit check including IP blocking,
	// global limits, and endpoint-specific limits.
	Check(ctx context.Context, ip string, authenticated bool, endpoint string) Result
}

// EffectiveLimit returns the most restrictive limit applicable to a request.
// It takes the global limit (authenticated or anonymous) and the endpoint path,
// then returns the minimum of the global and endpoint-specific limits.
func EffectiveLimit(cfg RateLimitConfig, authenticated bool, endpoint string) int {
	globalLimit := cfg.AnonymousLimit
	if authenticated {
		globalLimit = cfg.AuthenticatedLimit
	}

	for _, el := range cfg.EndpointLimits {
		if el.Path == endpoint {
			if el.Limit < globalLimit {
				return el.Limit
			}
			break
		}
	}

	return globalLimit
}
