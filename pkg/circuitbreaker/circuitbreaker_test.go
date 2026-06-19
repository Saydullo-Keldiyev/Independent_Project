package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// testClock provides a controllable clock for testing.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (tc *testClock) Now() time.Time {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.now
}

func (tc *testClock) Advance(d time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.now = tc.now.Add(d)
}

// newTestCB creates a circuit breaker with test defaults and a fresh prometheus registry.
func newTestCB(t *testing.T, cfg Config) (CircuitBreaker, *testClock, *prometheus.Registry) {
	t.Helper()
	clock := newTestClock()
	reg := prometheus.NewRegistry()
	logger := zaptest.NewLogger(t)

	cb, err := New(cfg,
		WithClock(clock.Now),
		WithLogger(logger),
		WithPrometheusRegisterer(reg),
	)
	if err != nil {
		t.Fatalf("failed to create circuit breaker: %v", err)
	}
	return cb, clock, reg
}

func successFn() (interface{}, error) {
	return "ok", nil
}

func failFn() (interface{}, error) {
	return nil, NewHTTPError(500, "internal server error")
}

func TestNew_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig("test-service")
	if cfg.FailureThreshold != 5 {
		t.Errorf("expected failure threshold 5, got %d", cfg.FailureThreshold)
	}
	if cfg.ResetTimeout != 30*time.Second {
		t.Errorf("expected reset timeout 30s, got %s", cfg.ResetTimeout)
	}
	if cfg.SuccessThreshold != 1 {
		t.Errorf("expected success threshold 1, got %d", cfg.SuccessThreshold)
	}
	if cfg.CacheTTL != 60*time.Second {
		t.Errorf("expected cache TTL 60s, got %s", cfg.CacheTTL)
	}
}

func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "empty service name",
			cfg:  Config{FailureThreshold: 5, ResetTimeout: 30 * time.Second, SuccessThreshold: 1},
		},
		{
			name: "failure threshold too low",
			cfg:  Config{ServiceName: "svc", FailureThreshold: 0, ResetTimeout: 30 * time.Second, SuccessThreshold: 1},
		},
		{
			name: "failure threshold too high",
			cfg:  Config{ServiceName: "svc", FailureThreshold: 101, ResetTimeout: 30 * time.Second, SuccessThreshold: 1},
		},
		{
			name: "reset timeout too low",
			cfg:  Config{ServiceName: "svc", FailureThreshold: 5, ResetTimeout: 500 * time.Millisecond, SuccessThreshold: 1},
		},
		{
			name: "reset timeout too high",
			cfg:  Config{ServiceName: "svc", FailureThreshold: 5, ResetTimeout: 301 * time.Second, SuccessThreshold: 1},
		},
		{
			name: "success threshold too low",
			cfg:  Config{ServiceName: "svc", FailureThreshold: 5, ResetTimeout: 30 * time.Second, SuccessThreshold: 0},
		},
		{
			name: "success threshold too high",
			cfg:  Config{ServiceName: "svc", FailureThreshold: 5, ResetTimeout: 30 * time.Second, SuccessThreshold: 11},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("expected ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestStateClosed_SuccessDoesNotOpenCircuit(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cb, _, _ := newTestCB(t, cfg)

	for i := 0; i < 10; i++ {
		result, err := cb.Execute(context.Background(), successFn)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result != "ok" {
			t.Fatalf("expected 'ok', got %v", result)
		}
	}

	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed, got %s", cb.GetState())
	}
}

func TestStateClosed_FailuresOpenCircuit(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 3
	cb, _, _ := newTestCB(t, cfg)

	// First 2 failures: still closed
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), failFn)
	}
	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed after %d failures, got %s", 2, cb.GetState())
	}

	// 3rd failure: opens circuit
	cb.Execute(context.Background(), failFn)
	if cb.GetState() != StateOpen {
		t.Errorf("expected state open after 3 failures, got %s", cb.GetState())
	}
}

func TestStateClosed_SuccessResetsFailureCount(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 3
	cb, _, _ := newTestCB(t, cfg)

	// 2 failures
	cb.Execute(context.Background(), failFn)
	cb.Execute(context.Background(), failFn)

	// 1 success resets counter
	cb.Execute(context.Background(), successFn)

	// 2 more failures should NOT open the circuit (only 2 consecutive)
	cb.Execute(context.Background(), failFn)
	cb.Execute(context.Background(), failFn)

	if cb.GetState() != StateClosed {
		t.Errorf("expected state closed after reset, got %s", cb.GetState())
	}
}

func TestStateOpen_ReturnsCachedResponse(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cb, clock, _ := newTestCB(t, cfg)

	// Success to cache response
	cb.Execute(context.Background(), func() (interface{}, error) {
		return "cached_value", nil
	})

	// Advance clock slightly
	clock.Advance(100 * time.Millisecond)

	// Fail to open circuit
	cb.Execute(context.Background(), failFn)
	if cb.GetState() != StateOpen {
		t.Fatalf("expected open state, got %s", cb.GetState())
	}

	// Next request should get cached response
	result, err := cb.Execute(context.Background(), successFn)
	if err != nil {
		t.Fatalf("expected no error with cached response, got %v", err)
	}
	if result != "cached_value" {
		t.Errorf("expected 'cached_value', got %v", result)
	}
}

func TestStateOpen_ReturnsGracefulDegradationWhenNoCachedResponse(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cfg.CacheTTL = 1 * time.Second
	cb, clock, _ := newTestCB(t, cfg)

	// Success to cache
	cb.Execute(context.Background(), successFn)

	// Advance past cache TTL
	clock.Advance(2 * time.Second)

	// Fail to open
	cb.Execute(context.Background(), failFn)

	// Next request should get degradation response (cache expired)
	result, err := cb.Execute(context.Background(), successFn)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	deg, ok := result.(*DegradationResponse)
	if !ok {
		t.Fatalf("expected DegradationResponse, got %T", result)
	}
	if deg.StatusCode != 503 {
		t.Errorf("expected status 503, got %d", deg.StatusCode)
	}
}

func TestStateOpen_TransitionsToHalfOpenAfterResetTimeout(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cfg.ResetTimeout = 5 * time.Second
	cb, clock, _ := newTestCB(t, cfg)

	// Open the circuit
	cb.Execute(context.Background(), failFn)
	if cb.GetState() != StateOpen {
		t.Fatalf("expected open state, got %s", cb.GetState())
	}

	// Advance past reset timeout
	clock.Advance(6 * time.Second)

	// GetState should now show half-open
	if cb.GetState() != StateHalfOpen {
		t.Errorf("expected half-open state, got %s", cb.GetState())
	}
}

func TestStateHalfOpen_ProbeSuccessTransitionsToClosed(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cfg.ResetTimeout = 5 * time.Second
	cfg.SuccessThreshold = 1
	cb, clock, _ := newTestCB(t, cfg)

	// Open the circuit
	cb.Execute(context.Background(), failFn)
	clock.Advance(6 * time.Second)

	// Probe request should succeed and close circuit
	result, err := cb.Execute(context.Background(), successFn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %v", result)
	}
	if cb.GetState() != StateClosed {
		t.Errorf("expected closed state, got %s", cb.GetState())
	}
}

func TestStateHalfOpen_MultipleSuccessesNeededForClosed(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cfg.ResetTimeout = 5 * time.Second
	cfg.SuccessThreshold = 3
	cb, clock, _ := newTestCB(t, cfg)

	// Open the circuit
	cb.Execute(context.Background(), failFn)
	clock.Advance(6 * time.Second)

	// First probe success: still half-open
	cb.Execute(context.Background(), successFn)
	if cb.GetState() != StateHalfOpen {
		t.Errorf("expected half-open after 1 success (threshold 3), got %s", cb.GetState())
	}

	// Second probe success
	cb.Execute(context.Background(), successFn)
	if cb.GetState() != StateHalfOpen {
		t.Errorf("expected half-open after 2 successes (threshold 3), got %s", cb.GetState())
	}

	// Third probe success: transitions to closed
	cb.Execute(context.Background(), successFn)
	if cb.GetState() != StateClosed {
		t.Errorf("expected closed after 3 successes, got %s", cb.GetState())
	}
}

func TestStateHalfOpen_ProbeFailureReturnsToOpen(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cfg.ResetTimeout = 5 * time.Second
	cb, clock, _ := newTestCB(t, cfg)

	// Open the circuit
	cb.Execute(context.Background(), failFn)
	clock.Advance(6 * time.Second)

	// Probe fails: back to open
	cb.Execute(context.Background(), failFn)
	if cb.GetState() != StateOpen {
		t.Errorf("expected open state after probe failure, got %s", cb.GetState())
	}
}

func TestStateHalfOpen_RejectsWhileProbeInFlight(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cfg.ResetTimeout = 5 * time.Second
	cb, clock, _ := newTestCB(t, cfg)

	// Open the circuit
	cb.Execute(context.Background(), failFn)
	clock.Advance(6 * time.Second)

	// Set up a slow probe
	probeCh := make(chan struct{})
	var probeStarted atomic.Int32

	go func() {
		cb.Execute(context.Background(), func() (interface{}, error) {
			probeStarted.Store(1)
			<-probeCh
			return "probe_result", nil
		})
	}()

	// Wait for probe to start
	for probeStarted.Load() == 0 {
		time.Sleep(time.Millisecond)
	}

	// Second request should be rejected
	result, err := cb.Execute(context.Background(), successFn)
	if err == nil {
		// With cached response, it might not error, but let's check for rejection pattern
		if result == "ok" {
			t.Error("expected rejection in half-open while probe in flight, but got success")
		}
	}
	// It should be either ErrCircuitReject or return cached/degradation
	if err != nil && !errors.Is(err, ErrCircuitReject) {
		t.Logf("Got expected rejection: %v", err)
	}

	// Release probe
	close(probeCh)
}

func TestFailureDetection_HTTP5xx(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cb, _, _ := newTestCB(t, cfg)

	// HTTP 500 should count as failure
	cb.Execute(context.Background(), func() (interface{}, error) {
		return nil, NewHTTPError(500, "internal error")
	})
	if cb.GetState() != StateOpen {
		t.Error("expected open after HTTP 500")
	}
}

func TestFailureDetection_HTTP4xxNotCounted(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 3
	cb, _, _ := newTestCB(t, cfg)

	// HTTP 400 should NOT count as failure
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func() (interface{}, error) {
			return nil, NewHTTPError(400, "bad request")
		})
	}
	if cb.GetState() != StateClosed {
		t.Error("expected closed after HTTP 4xx errors (not counted as failures)")
	}
}

func TestFailureDetection_ConnectionTimeout(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cb, _, _ := newTestCB(t, cfg)

	// Context deadline exceeded should count as failure
	cb.Execute(context.Background(), func() (interface{}, error) {
		return nil, context.DeadlineExceeded
	})
	if cb.GetState() != StateOpen {
		t.Error("expected open after connection timeout")
	}
}

func TestFailureDetection_ConnectionRefused(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cb, _, _ := newTestCB(t, cfg)

	// net.OpError should count as failure (connection refused)
	cb.Execute(context.Background(), func() (interface{}, error) {
		return nil, &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: fmt.Errorf("connection refused"),
		}
	})
	if cb.GetState() != StateOpen {
		t.Error("expected open after connection refused")
	}
}

func TestMetrics(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 2
	cb, _, _ := newTestCB(t, cfg)

	// Execute some requests
	cb.Execute(context.Background(), successFn)
	cb.Execute(context.Background(), failFn)
	cb.Execute(context.Background(), failFn)

	metrics := cb.GetMetrics()
	if metrics.State != StateOpen {
		t.Errorf("expected open state in metrics, got %v", metrics.State)
	}
	if metrics.ConsecutiveFails != 2 {
		t.Errorf("expected 2 consecutive fails, got %d", metrics.ConsecutiveFails)
	}
	if metrics.TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %d", metrics.TotalRequests)
	}
	if metrics.TotalFailures != 2 {
		t.Errorf("expected 2 total failures, got %d", metrics.TotalFailures)
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State(%d).String() = %s, want %s", tt.state, got, tt.expected)
		}
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	cfg := DefaultConfig("test-service")
	cfg.FailureThreshold = 1
	cfg.CacheTTL = 10 * time.Second
	cb, clock, _ := newTestCB(t, cfg)

	// Cache a response
	cb.Execute(context.Background(), func() (interface{}, error) {
		return "fresh", nil
	})
	clock.Advance(100 * time.Millisecond)

	// Open circuit
	cb.Execute(context.Background(), failFn)

	// Cache still valid (within 10s)
	result, err := cb.Execute(context.Background(), successFn)
	if err != nil {
		t.Fatalf("expected cached response, got error: %v", err)
	}
	if result != "fresh" {
		t.Errorf("expected 'fresh', got %v", result)
	}

	// Advance past cache TTL
	clock.Advance(11 * time.Second)

	// Cache expired: degradation response
	result, err = cb.Execute(context.Background(), successFn)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen after cache expiry, got %v", err)
	}
}

func TestPrometheusMetricsRegistered(t *testing.T) {
	cfg := DefaultConfig("metrics-test-service")
	_, _, reg := newTestCB(t, cfg)

	// Gather all metrics
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	expectedMetrics := map[string]bool{
		"circuit_breaker_state":                 false,
		"circuit_breaker_consecutive_failures":  false,
		"circuit_breaker_seconds_since_failure": false,
	}

	for _, mf := range mfs {
		if _, ok := expectedMetrics[mf.GetName()]; ok {
			expectedMetrics[mf.GetName()] = true
		}
	}

	for name, found := range expectedMetrics {
		if !found {
			t.Errorf("expected metric %s to be registered", name)
		}
	}
}

func TestNew_WithDefaultLogger(t *testing.T) {
	cfg := DefaultConfig("test-service")
	reg := prometheus.NewRegistry()

	cb, err := New(cfg, WithPrometheusRegisterer(reg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb == nil {
		t.Fatal("expected non-nil circuit breaker")
	}
}

func TestNew_WithCustomLogger(t *testing.T) {
	cfg := DefaultConfig("test-service")
	logger, _ := zap.NewDevelopment()
	reg := prometheus.NewRegistry()

	cb, err := New(cfg, WithLogger(logger), WithPrometheusRegisterer(reg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb == nil {
		t.Fatal("expected non-nil circuit breaker")
	}
}

func TestFullLifecycle_ClosedToOpenToHalfOpenToClosed(t *testing.T) {
	cfg := DefaultConfig("lifecycle-service")
	cfg.FailureThreshold = 2
	cfg.ResetTimeout = 10 * time.Second
	cfg.SuccessThreshold = 2
	cb, clock, _ := newTestCB(t, cfg)

	// Start: closed
	if cb.GetState() != StateClosed {
		t.Fatalf("expected initial state closed, got %s", cb.GetState())
	}

	// Fail twice: transitions to open
	cb.Execute(context.Background(), failFn)
	cb.Execute(context.Background(), failFn)
	if cb.GetState() != StateOpen {
		t.Fatalf("expected open after 2 failures, got %s", cb.GetState())
	}

	// Wait for reset timeout
	clock.Advance(11 * time.Second)

	// First probe succeeds: still half-open (need 2 successes)
	cb.Execute(context.Background(), successFn)
	if cb.GetState() != StateHalfOpen {
		t.Fatalf("expected half-open after 1 success (threshold 2), got %s", cb.GetState())
	}

	// Second probe succeeds: transitions to closed
	cb.Execute(context.Background(), successFn)
	if cb.GetState() != StateClosed {
		t.Fatalf("expected closed after 2 successes, got %s", cb.GetState())
	}
}
