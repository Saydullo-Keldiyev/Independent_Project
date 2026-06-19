package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestEffectiveLimit(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name          string
		authenticated bool
		endpoint      string
		want          int
	}{
		{
			name:          "anonymous global limit",
			authenticated: false,
			endpoint:      "",
			want:          30,
		},
		{
			name:          "authenticated global limit",
			authenticated: true,
			endpoint:      "",
			want:          200,
		},
		{
			name:          "login endpoint - anonymous (endpoint more restrictive)",
			authenticated: false,
			endpoint:      "/login",
			want:          5,
		},
		{
			name:          "login endpoint - authenticated (endpoint more restrictive)",
			authenticated: true,
			endpoint:      "/login",
			want:          5,
		},
		{
			name:          "register endpoint - anonymous (endpoint more restrictive)",
			authenticated: false,
			endpoint:      "/register",
			want:          3,
		},
		{
			name:          "register endpoint - authenticated (endpoint more restrictive)",
			authenticated: true,
			endpoint:      "/register",
			want:          3,
		},
		{
			name:          "bids endpoint - anonymous (anonymous global more restrictive)",
			authenticated: false,
			endpoint:      "/bids",
			want:          30,
		},
		{
			name:          "bids endpoint - authenticated (endpoint more restrictive)",
			authenticated: true,
			endpoint:      "/bids",
			want:          30,
		},
		{
			name:          "unknown endpoint - uses global",
			authenticated: true,
			endpoint:      "/unknown",
			want:          200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveLimit(cfg, tt.authenticated, tt.endpoint)
			if got != tt.want {
				t.Errorf("EffectiveLimit() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestInMemoryLimiter_Allow(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 30, 0, time.UTC)
	cfg := RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     10,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return now })
	ctx := context.Background()

	// Should allow first request
	allowed, remaining, retryAfter, err := limiter.Allow(ctx, "test-key", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected request to be allowed")
	}
	if remaining != 4 {
		t.Errorf("expected remaining=4, got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0, got %v", retryAfter)
	}

	// Fill up to limit
	for i := 0; i < 4; i++ {
		allowed, _, _, _ = limiter.Allow(ctx, "test-key", 5)
		if !allowed {
			t.Errorf("request %d should be allowed", i+2)
		}
	}

	// Should be denied now
	allowed, _, retryAfter, _ = limiter.Allow(ctx, "test-key", 5)
	if allowed {
		t.Error("expected request to be denied after exceeding limit")
	}
	if retryAfter == 0 {
		t.Error("expected non-zero retryAfter when denied")
	}
}

func TestInMemoryLimiter_SlidingWindow(t *testing.T) {
	// Start at the beginning of a window
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := baseTime

	cfg := RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     10,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return currentTime })
	ctx := context.Background()

	// Fill the first window with 8 requests
	for i := 0; i < 8; i++ {
		limiter.Allow(ctx, "slide-key", 10)
	}

	// Move to 30 seconds into the next window (50% of previous window still counts)
	currentTime = baseTime.Add(time.Minute + 30*time.Second)

	// At this point: current_count=0, previous_count=8, remaining_fraction=0.5
	// Weighted count = 0 + (8 × 0.5) = 4
	// So we should be able to make 6 more requests (limit=10)
	for i := 0; i < 6; i++ {
		allowed, _, _, _ := limiter.Allow(ctx, "slide-key", 10)
		if !allowed {
			t.Errorf("request %d should be allowed (weighted count should be < 10)", i+1)
		}
	}

	// The 7th should be denied (4 from prev + 6 current = 10)
	allowed, _, _, _ := limiter.Allow(ctx, "slide-key", 10)
	if allowed {
		t.Error("expected denial: weighted count should be at limit")
	}
}

func TestInMemoryLimiter_IPBlocking(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	cfg := RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     3, // Low threshold for testing
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return now })
	ctx := context.Background()

	// Record violations below threshold
	limiter.RecordViolation(ctx, "192.168.1.1")
	limiter.RecordViolation(ctx, "192.168.1.1")

	blocked, _ := limiter.IsBlocked(ctx, "192.168.1.1")
	if blocked {
		t.Error("IP should not be blocked yet (only 2 violations)")
	}

	// Third violation triggers block
	limiter.RecordViolation(ctx, "192.168.1.1")

	blocked, remaining := limiter.IsBlocked(ctx, "192.168.1.1")
	if !blocked {
		t.Error("IP should be blocked after 3 violations")
	}
	if remaining <= 0 {
		t.Error("expected positive remaining block duration")
	}
}

func TestInMemoryLimiter_IPBlockExpiry(t *testing.T) {
	currentTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	cfg := RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     2,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return currentTime })
	ctx := context.Background()

	// Trigger block
	limiter.RecordViolation(ctx, "10.0.0.1")
	limiter.RecordViolation(ctx, "10.0.0.1")

	blocked, _ := limiter.IsBlocked(ctx, "10.0.0.1")
	if !blocked {
		t.Fatal("expected IP to be blocked")
	}

	// Move past block duration
	currentTime = currentTime.Add(16 * time.Minute)
	blocked, _ = limiter.IsBlocked(ctx, "10.0.0.1")
	if blocked {
		t.Error("IP should no longer be blocked after duration expires")
	}
}

func TestInMemoryLimiter_ViolationWindowReset(t *testing.T) {
	currentTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	cfg := RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     5,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return currentTime })
	ctx := context.Background()

	// Record 4 violations (one short of threshold)
	for i := 0; i < 4; i++ {
		limiter.RecordViolation(ctx, "10.0.0.2")
	}

	// Move past the violation window
	currentTime = currentTime.Add(time.Hour + time.Minute)

	// Record 1 more violation — should start a new window, not trigger block
	limiter.RecordViolation(ctx, "10.0.0.2")

	blocked, _ := limiter.IsBlocked(ctx, "10.0.0.2")
	if blocked {
		t.Error("IP should not be blocked — violation window should have reset")
	}
}

func TestInMemoryLimiter_Check(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 30, 0, time.UTC)
	cfg := RateLimitConfig{
		AuthenticatedLimit: 5,
		AnonymousLimit:     2,
		Window:             time.Minute,
		BlockThreshold:     10,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
		EndpointLimits: []EndpointLimit{
			{Path: "/login", Limit: 1},
		},
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return now })
	ctx := context.Background()

	// First request should succeed
	result := limiter.Check(ctx, "192.168.1.100", true, "/login")
	if !result.Allowed {
		t.Error("first request should be allowed")
	}

	// Second request to login should be denied (limit is 1)
	result = limiter.Check(ctx, "192.168.1.100", true, "/login")
	if result.Allowed {
		t.Error("second request should be denied (endpoint limit is 1)")
	}
	if result.RetryAfter == 0 {
		t.Error("expected non-zero RetryAfter")
	}
}

func TestInMemoryLimiter_CheckBlockedIP(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	cfg := RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     2,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return now })
	ctx := context.Background()

	// Block the IP
	limiter.RecordViolation(ctx, "10.10.10.10")
	limiter.RecordViolation(ctx, "10.10.10.10")

	// Check should return blocked result
	result := limiter.Check(ctx, "10.10.10.10", true, "")
	if result.Allowed {
		t.Error("request from blocked IP should be denied")
	}
	if !result.Blocked {
		t.Error("result should indicate IP is blocked")
	}
	if result.BlockReason == "" {
		t.Error("expected block reason")
	}
	if result.BlockRemaining == 0 {
		t.Error("expected positive block remaining duration")
	}
}

func TestInMemoryLimiter_RetryAfterValue(t *testing.T) {
	// Start at 30 seconds into the window
	now := time.Date(2024, 1, 15, 10, 0, 30, 0, time.UTC)
	cfg := RateLimitConfig{
		AuthenticatedLimit: 200,
		AnonymousLimit:     30,
		Window:             time.Minute,
		BlockThreshold:     10,
		BlockDuration:      15 * time.Minute,
		ViolationWindow:    time.Hour,
	}

	limiter := newInMemoryLimiterForTest(cfg, func() time.Time { return now })
	ctx := context.Background()

	// Exhaust the limit
	for i := 0; i < 30; i++ {
		limiter.Allow(ctx, "retry-test", 30)
	}

	// The next request should fail with retryAfter = 30 seconds (until window ends)
	_, _, retryAfter, _ := limiter.Allow(ctx, "retry-test", 30)
	expectedRetry := 30 * time.Second
	if retryAfter != expectedRetry {
		t.Errorf("expected retryAfter=%v, got %v", expectedRetry, retryAfter)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.AuthenticatedLimit != 200 {
		t.Errorf("expected AuthenticatedLimit=200, got %d", cfg.AuthenticatedLimit)
	}
	if cfg.AnonymousLimit != 30 {
		t.Errorf("expected AnonymousLimit=30, got %d", cfg.AnonymousLimit)
	}
	if cfg.Window != time.Minute {
		t.Errorf("expected Window=1m, got %v", cfg.Window)
	}
	if cfg.BlockThreshold != 10 {
		t.Errorf("expected BlockThreshold=10, got %d", cfg.BlockThreshold)
	}
	if cfg.BlockDuration != 15*time.Minute {
		t.Errorf("expected BlockDuration=15m, got %v", cfg.BlockDuration)
	}
	if len(cfg.EndpointLimits) != 3 {
		t.Errorf("expected 3 endpoint limits, got %d", len(cfg.EndpointLimits))
	}
}
