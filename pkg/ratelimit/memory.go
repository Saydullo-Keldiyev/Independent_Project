package ratelimit

import (
	"context"
	"sync"
	"time"
)

// windowEntry tracks the count for a single time window.
type windowEntry struct {
	count     int
	timestamp time.Time
}

// violationRecord tracks violations for an IP.
type violationRecord struct {
	count     int
	firstSeen time.Time
}

// ipBlock tracks block info for an IP.
type ipBlock struct {
	expiresAt time.Time
	reason    string
}

// InMemoryLimiter implements the RateLimiter interface using in-memory storage.
// Used as a fallback when Redis is unavailable.
type InMemoryLimiter struct {
	config     RateLimitConfig
	mu         sync.RWMutex
	windows    map[string]*windowEntry // key → window entry
	prevWindows map[string]*windowEntry // key → previous window entry
	violations map[string]*violationRecord
	blocks     map[string]*ipBlock
	nowFunc    func() time.Time // for testing
}

// NewInMemoryLimiter creates a new in-memory rate limiter.
func NewInMemoryLimiter(config RateLimitConfig) *InMemoryLimiter {
	limiter := &InMemoryLimiter{
		config:      config,
		windows:     make(map[string]*windowEntry),
		prevWindows: make(map[string]*windowEntry),
		violations:  make(map[string]*violationRecord),
		blocks:      make(map[string]*ipBlock),
		nowFunc:     time.Now,
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// newInMemoryLimiterForTest creates a limiter with a custom time function (for testing).
func newInMemoryLimiterForTest(config RateLimitConfig, nowFunc func() time.Time) *InMemoryLimiter {
	return &InMemoryLimiter{
		config:      config,
		windows:     make(map[string]*windowEntry),
		prevWindows: make(map[string]*windowEntry),
		violations:  make(map[string]*violationRecord),
		blocks:      make(map[string]*ipBlock),
		nowFunc:     nowFunc,
	}
}

// Allow checks whether a request is within the rate limit using the sliding window algorithm.
func (m *InMemoryLimiter) Allow(ctx context.Context, key string, limit int) (bool, int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFunc()
	window := m.config.Window
	currentWindow := now.Truncate(window)

	// Rotate windows if needed
	m.rotateWindows(key, currentWindow)

	// Get current and previous counts
	currentCount := 0
	if entry, ok := m.windows[key]; ok && entry.timestamp.Equal(currentWindow) {
		currentCount = entry.count
	}

	previousCount := 0
	if entry, ok := m.prevWindows[key]; ok {
		previousCount = entry.count
	}

	// Calculate weighted count using sliding window algorithm
	elapsed := now.Sub(currentWindow)
	remainingFraction := float64(window-elapsed) / float64(window)
	weightedCount := float64(currentCount) + (float64(previousCount) * remainingFraction)
	effectiveCount := int(weightedCount)

	if effectiveCount >= limit {
		retryAfter := currentWindow.Add(window).Sub(now)
		return false, 0, retryAfter, nil
	}

	// Increment counter
	if entry, ok := m.windows[key]; ok && entry.timestamp.Equal(currentWindow) {
		entry.count++
	} else {
		m.windows[key] = &windowEntry{
			count:     1,
			timestamp: currentWindow,
		}
	}

	remaining := limit - effectiveCount - 1
	if remaining < 0 {
		remaining = 0
	}

	return true, remaining, 0, nil
}

// rotateWindows moves the current window to previous if a new window has started.
func (m *InMemoryLimiter) rotateWindows(key string, currentWindow time.Time) {
	if entry, ok := m.windows[key]; ok {
		if !entry.timestamp.Equal(currentWindow) {
			// Current window has advanced; move old current to previous
			m.prevWindows[key] = entry
			m.windows[key] = &windowEntry{
				count:     0,
				timestamp: currentWindow,
			}
		}
	}
}

// IsBlocked checks if an IP is currently blocked.
func (m *InMemoryLimiter) IsBlocked(ctx context.Context, ip string) (bool, time.Duration) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	block, ok := m.blocks[ip]
	if !ok {
		return false, 0
	}

	now := m.nowFunc()
	if now.After(block.expiresAt) {
		return false, 0
	}

	return true, block.expiresAt.Sub(now)
}

// RecordViolation records a rate limit violation for an IP.
func (m *InMemoryLimiter) RecordViolation(ctx context.Context, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFunc()

	record, ok := m.violations[ip]
	if !ok || now.Sub(record.firstSeen) > m.config.ViolationWindow {
		// Start a new violation window
		m.violations[ip] = &violationRecord{
			count:     1,
			firstSeen: now,
		}
		return nil
	}

	record.count++

	if record.count >= m.config.BlockThreshold {
		m.blocks[ip] = &ipBlock{
			expiresAt: now.Add(m.config.BlockDuration),
			reason:    "IP blocked due to excessive rate limit violations",
		}
	}

	return nil
}

// Check performs a comprehensive rate limit check including IP blocking.
func (m *InMemoryLimiter) Check(ctx context.Context, ip string, authenticated bool, endpoint string) Result {
	// 1. Check if IP is blocked
	blocked, blockRemaining := m.IsBlocked(ctx, ip)
	if blocked {
		return Result{
			Allowed:        false,
			Blocked:        true,
			BlockReason:    "IP blocked due to excessive rate limit violations",
			BlockRemaining: blockRemaining,
		}
	}

	// 2. Determine the effective limit
	effectiveLimit := EffectiveLimit(m.config, authenticated, endpoint)

	// 3. Build rate limit key
	key := m.buildKey(ip, authenticated, endpoint)

	// 4. Check sliding window
	allowed, remaining, retryAfter, _ := m.Allow(ctx, key, effectiveLimit)

	if !allowed {
		_ = m.RecordViolation(ctx, ip)
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

// buildKey constructs the rate limit key.
func (m *InMemoryLimiter) buildKey(ip string, authenticated bool, endpoint string) string {
	scope := "anon"
	if authenticated {
		scope = "auth"
	}

	if endpoint != "" {
		return scope + ":" + endpoint + ":" + ip
	}
	return scope + ":" + ip
}

// cleanup runs periodically to remove expired entries.
func (m *InMemoryLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()

		// Clean expired blocks
		for ip, block := range m.blocks {
			if now.After(block.expiresAt) {
				delete(m.blocks, ip)
			}
		}

		// Clean old violation records
		for ip, record := range m.violations {
			if now.Sub(record.firstSeen) > m.config.ViolationWindow {
				delete(m.violations, ip)
			}
		}

		// Clean old window entries
		windowDuration := m.config.Window
		for key, entry := range m.windows {
			if now.Sub(entry.timestamp) > 2*windowDuration {
				delete(m.windows, key)
			}
		}
		for key, entry := range m.prevWindows {
			if now.Sub(entry.timestamp) > 2*windowDuration {
				delete(m.prevWindows, key)
			}
		}

		m.mu.Unlock()
	}
}
