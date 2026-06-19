package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter implements the RateLimiter interface using Redis
// with a sliding window algorithm and IP blocking support.
type RedisRateLimiter struct {
	client   *redis.Client
	config   RateLimitConfig
	fallback *InMemoryLimiter
}

// blockInfo stores information about a blocked IP in Redis.
type blockInfo struct {
	BlockedAt  time.Time `json:"blocked_at"`
	Violations int       `json:"violations"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// NewRedisRateLimiter creates a new Redis-backed sliding window rate limiter.
// It also initializes an in-memory fallback limiter for use when Redis is unavailable.
func NewRedisRateLimiter(client *redis.Client, config RateLimitConfig) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:   client,
		config:   config,
		fallback: NewInMemoryLimiter(config),
	}
}

// Allow checks whether a request is within the rate limit using the sliding window algorithm.
// The algorithm computes: current_count + (previous_count × remaining_fraction)
func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int) (bool, int, time.Duration, error) {
	now := time.Now()

	allowed, remaining, retryAfter, err := r.slidingWindowCheck(ctx, key, limit, now)
	if err != nil {
		// Fallback to in-memory limiter when Redis is unavailable
		return r.fallback.Allow(ctx, key, limit)
	}

	return allowed, remaining, retryAfter, nil
}

// slidingWindowCheck performs the sliding window rate limit check against Redis.
func (r *RedisRateLimiter) slidingWindowCheck(ctx context.Context, key string, limit int, now time.Time) (bool, int, time.Duration, error) {
	window := r.config.Window

	// Current and previous window timestamps
	currentWindow := now.Truncate(window)
	previousWindow := currentWindow.Add(-window)

	currentKey := fmt.Sprintf("rl:%s:%d", key, currentWindow.Unix())
	previousKey := fmt.Sprintf("rl:%s:%d", key, previousWindow.Unix())

	// Get counts from both windows using a pipeline
	pipe := r.client.Pipeline()
	currentCmd := pipe.Get(ctx, currentKey)
	previousCmd := pipe.Get(ctx, previousKey)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		// Check if it's a connection error (both commands would fail)
		if currentCmd.Err() != nil && currentCmd.Err() != redis.Nil {
			return false, 0, 0, fmt.Errorf("redis unavailable: %w", err)
		}
	}

	var currentCount int64
	if currentCmd.Err() == nil {
		currentCount, _ = currentCmd.Int64()
	}

	var previousCount int64
	if previousCmd.Err() == nil {
		previousCount, _ = previousCmd.Int64()
	}

	// Calculate the fraction of the previous window that overlaps
	elapsed := now.Sub(currentWindow)
	remainingFraction := float64(window-elapsed) / float64(window)

	// Sliding window count: current + (previous × remaining fraction of previous window)
	weightedCount := float64(currentCount) + (float64(previousCount) * remainingFraction)
	effectiveCount := int(weightedCount)

	if effectiveCount >= limit {
		// Calculate retry-after: time until current window ends
		retryAfter := currentWindow.Add(window).Sub(now)
		return false, 0, retryAfter, nil
	}

	// Increment current window counter
	incrPipe := r.client.Pipeline()
	incrPipe.Incr(ctx, currentKey)
	incrPipe.Expire(ctx, currentKey, 2*window) // TTL = 2 × window duration
	_, err = incrPipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, fmt.Errorf("redis incr failed: %w", err)
	}

	remaining := limit - effectiveCount - 1
	if remaining < 0 {
		remaining = 0
	}

	return true, remaining, 0, nil
}

// IsBlocked checks if an IP is currently blocked.
func (r *RedisRateLimiter) IsBlocked(ctx context.Context, ip string) (bool, time.Duration) {
	blocked, remaining, err := r.isBlockedRedis(ctx, ip)
	if err != nil {
		// Fallback to in-memory
		return r.fallback.IsBlocked(ctx, ip)
	}
	return blocked, remaining
}

// isBlockedRedis checks the IP block status in Redis.
func (r *RedisRateLimiter) isBlockedRedis(ctx context.Context, ip string) (bool, time.Duration, error) {
	key := fmt.Sprintf("rl:block:%s", ip)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("redis unavailable: %w", err)
	}

	var info blockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return false, 0, nil
	}

	now := time.Now()
	if now.After(info.ExpiresAt) {
		// Block has expired, clean up
		r.client.Del(ctx, key)
		return false, 0, nil
	}

	remaining := info.ExpiresAt.Sub(now)
	return true, remaining, nil
}

// RecordViolation records a rate limit violation for an IP.
// If the violation count reaches BlockThreshold within ViolationWindow, the IP is blocked.
func (r *RedisRateLimiter) RecordViolation(ctx context.Context, ip string) error {
	err := r.recordViolationRedis(ctx, ip)
	if err != nil {
		// Fallback to in-memory
		return r.fallback.RecordViolation(ctx, ip)
	}
	return nil
}

// recordViolationRedis records a violation in Redis.
func (r *RedisRateLimiter) recordViolationRedis(ctx context.Context, ip string) error {
	now := time.Now()
	hourWindow := now.Truncate(r.config.ViolationWindow)
	violationKey := fmt.Sprintf("rl:violations:%s:%d", ip, hourWindow.Unix())

	// Increment violation counter
	count, err := r.client.Incr(ctx, violationKey).Result()
	if err != nil {
		return fmt.Errorf("redis unavailable: %w", err)
	}

	// Set TTL on violation counter if it's the first violation in this window
	if count == 1 {
		r.client.Expire(ctx, violationKey, r.config.ViolationWindow)
	}

	// Check if we need to block this IP
	if int(count) >= r.config.BlockThreshold {
		return r.blockIP(ctx, ip, int(count))
	}

	return nil
}

// blockIP blocks an IP address for the configured block duration.
func (r *RedisRateLimiter) blockIP(ctx context.Context, ip string, violations int) error {
	now := time.Now()
	info := blockInfo{
		BlockedAt:  now,
		Violations: violations,
		ExpiresAt:  now.Add(r.config.BlockDuration),
	}

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal block info: %w", err)
	}

	key := fmt.Sprintf("rl:block:%s", ip)
	return r.client.Set(ctx, key, data, r.config.BlockDuration).Err()
}

// Check performs a comprehensive rate limit check including IP blocking,
// global limits, and endpoint-specific limits.
func (r *RedisRateLimiter) Check(ctx context.Context, ip string, authenticated bool, endpoint string) Result {
	// 1. Check if IP is blocked
	blocked, blockRemaining := r.IsBlocked(ctx, ip)
	if blocked {
		return Result{
			Allowed:        false,
			Blocked:        true,
			BlockReason:    "IP blocked due to excessive rate limit violations",
			BlockRemaining: blockRemaining,
		}
	}

	// 2. Determine the effective limit (most restrictive)
	effectiveLimit := EffectiveLimit(r.config, authenticated, endpoint)

	// 3. Build the rate limit key
	key := r.buildKey(ip, authenticated, endpoint)

	// 4. Check the sliding window rate limit
	allowed, remaining, retryAfter, err := r.Allow(ctx, key, effectiveLimit)
	if err != nil {
		// If everything fails, deny the request to be safe
		return Result{
			Allowed:    false,
			RetryAfter: time.Second,
		}
	}

	if !allowed {
		// Record a violation for this IP
		_ = r.RecordViolation(ctx, ip)
		return Result{
			Allowed:    false,
			Remaining:  remaining,
			RetryAfter: retryAfter,
		}
	}

	return Result{
		Allowed:   true,
		Remaining: remaining,
	}
}

// buildKey constructs the rate limit key for a request.
func (r *RedisRateLimiter) buildKey(ip string, authenticated bool, endpoint string) string {
	scope := "anon"
	if authenticated {
		scope = "auth"
	}

	if endpoint != "" {
		return fmt.Sprintf("%s:%s:%s", scope, endpoint, ip)
	}
	return fmt.Sprintf("%s:%s", scope, ip)
}
