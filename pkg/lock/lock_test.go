package lock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// testRedisClient creates a Redis client for testing.
// Tests require a running Redis instance on localhost:6379.
func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for tests to avoid conflicts
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available, skipping test: %v", err)
	}

	// Flush test DB before each test
	client.FlushDB(ctx)

	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})

	return client
}

func TestDefaultLockConfig(t *testing.T) {
	cfg := DefaultLockConfig()
	if cfg.TTL != 5*time.Second {
		t.Errorf("expected TTL 5s, got %v", cfg.TTL)
	}
	if cfg.RetryCount != 3 {
		t.Errorf("expected RetryCount 3, got %d", cfg.RetryCount)
	}
	if cfg.RetryBaseDelay != 50*time.Millisecond {
		t.Errorf("expected RetryBaseDelay 50ms, got %v", cfg.RetryBaseDelay)
	}
	if cfg.RetryMultiplier != 2.0 {
		t.Errorf("expected RetryMultiplier 2.0, got %f", cfg.RetryMultiplier)
	}
	if cfg.ExtendThreshold != 0.8 {
		t.Errorf("expected ExtendThreshold 0.8, got %f", cfg.ExtendThreshold)
	}
}

func TestAcquireAndRelease(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	cfg := LockConfig{
		TTL:             5 * time.Second,
		RetryCount:      3,
		RetryBaseDelay:  50 * time.Millisecond,
		RetryMultiplier: 2.0,
		ExtendThreshold: 0.8,
	}

	// Acquire lock
	lock, err := mgr.Acquire(ctx, "test:resource:1", cfg)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}
	if lock == nil {
		t.Fatal("expected non-nil lock")
	}
	if lock.Key != "test:resource:1" {
		t.Errorf("expected key test:resource:1, got %s", lock.Key)
	}
	if lock.FencingToken <= 0 {
		t.Errorf("expected positive fencing token, got %d", lock.FencingToken)
	}
	if lock.Owner == "" {
		t.Error("expected non-empty owner")
	}

	// Release lock
	err = mgr.Release(ctx, lock)
	if err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}
}

func TestFencingTokenMonotonicity(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	cfg := LockConfig{
		TTL:             1 * time.Second,
		RetryCount:      0,
		RetryBaseDelay:  50 * time.Millisecond,
		RetryMultiplier: 2.0,
	}

	var lastToken int64
	for i := 0; i < 10; i++ {
		lock, err := mgr.Acquire(ctx, "monotonic:test", cfg)
		if err != nil {
			t.Fatalf("iteration %d: failed to acquire: %v", i, err)
		}
		if lock.FencingToken <= lastToken {
			t.Fatalf("iteration %d: token %d not greater than %d", i, lock.FencingToken, lastToken)
		}
		lastToken = lock.FencingToken

		err = mgr.Release(ctx, lock)
		if err != nil {
			t.Fatalf("iteration %d: failed to release: %v", i, err)
		}
	}
}

func TestOwnershipVerificationOnRelease(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	cfg := LockConfig{
		TTL:             5 * time.Second,
		RetryCount:      0,
		RetryBaseDelay:  50 * time.Millisecond,
		RetryMultiplier: 2.0,
	}

	// Acquire lock
	lock, err := mgr.Acquire(ctx, "ownership:test", cfg)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Try to release with wrong owner
	fakeLock := &Lock{
		Key:          lock.Key,
		Owner:        "wrong-owner",
		FencingToken: lock.FencingToken,
		TTL:          lock.TTL,
		AcquiredAt:   lock.AcquiredAt,
	}
	err = mgr.Release(ctx, fakeLock)
	if err != ErrOwnershipMismatch {
		t.Fatalf("expected ErrOwnershipMismatch, got: %v", err)
	}

	// Try to release with wrong fencing token
	fakeLock2 := &Lock{
		Key:          lock.Key,
		Owner:        lock.Owner,
		FencingToken: lock.FencingToken + 100,
		TTL:          lock.TTL,
		AcquiredAt:   lock.AcquiredAt,
	}
	err = mgr.Release(ctx, fakeLock2)
	if err != ErrOwnershipMismatch {
		t.Fatalf("expected ErrOwnershipMismatch for wrong token, got: %v", err)
	}

	// Verify lock is still held (original release should work)
	err = mgr.Release(ctx, lock)
	if err != nil {
		t.Fatalf("expected successful release with correct owner: %v", err)
	}
}

func TestRetryWithExponentialBackoff(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()

	// Acquire lock with long TTL and no retries
	holdCfg := LockConfig{
		TTL:        10 * time.Second,
		RetryCount: 0,
	}
	lock, err := mgr.Acquire(ctx, "retry:test", holdCfg)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}

	// Try to acquire same lock with retries — should fail after exponential backoff
	retryCfg := LockConfig{
		TTL:             10 * time.Second,
		RetryCount:      3,
		RetryBaseDelay:  50 * time.Millisecond,
		RetryMultiplier: 2.0,
	}

	start := time.Now()
	_, err = mgr.Acquire(ctx, "retry:test", retryCfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when acquiring held lock")
	}

	// Should take at least: 50ms + 100ms + 200ms = 350ms for 3 retries
	expectedMinDelay := 300 * time.Millisecond
	if elapsed < expectedMinDelay {
		t.Errorf("expected at least %v delay for retries, got %v", expectedMinDelay, elapsed)
	}

	// Clean up
	mgr.Release(ctx, lock)
}

func TestExtendTTL(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	cfg := LockConfig{
		TTL:             2 * time.Second,
		RetryCount:      0,
		RetryBaseDelay:  50 * time.Millisecond,
		RetryMultiplier: 2.0,
	}

	// Acquire lock
	lock, err := mgr.Acquire(ctx, "extend:test", cfg)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Extend TTL
	err = mgr.Extend(ctx, lock)
	if err != nil {
		t.Fatalf("failed to extend lock: %v", err)
	}

	// Verify the lock still exists with extended TTL
	redisKey := lockKeyPrefix(lock.Key)
	ttl, err := client.PTTL(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("failed to get TTL: %v", err)
	}

	// TTL should be close to the full TTL after extension
	if ttl < time.Duration(float64(lock.TTL)*0.5) {
		t.Errorf("expected TTL > 50%% of original after extend, got %v", ttl)
	}

	// Clean up
	mgr.Release(ctx, lock)
}

func TestExtendFailsWithWrongOwner(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	cfg := LockConfig{
		TTL:        5 * time.Second,
		RetryCount: 0,
	}

	lock, err := mgr.Acquire(ctx, "extend:ownership", cfg)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Try extend with wrong owner
	fakeLock := &Lock{
		Key:          lock.Key,
		Owner:        "wrong-owner",
		FencingToken: lock.FencingToken,
		TTL:          lock.TTL,
		AcquiredAt:   lock.AcquiredAt,
	}
	err = mgr.Extend(ctx, fakeLock)
	if err != ErrOwnershipMismatch {
		t.Fatalf("expected ErrOwnershipMismatch, got: %v", err)
	}

	// Clean up
	mgr.Release(ctx, lock)
}

func TestExtendExpiredLock(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	cfg := LockConfig{
		TTL:        200 * time.Millisecond,
		RetryCount: 0,
	}

	lock, err := mgr.Acquire(ctx, "extend:expired", cfg)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Wait for lock to expire
	time.Sleep(300 * time.Millisecond)

	// Try to extend expired lock
	err = mgr.Extend(ctx, lock)
	if err != ErrLockNotHeld {
		t.Fatalf("expected ErrLockNotHeld for expired lock, got: %v", err)
	}
}

func TestShouldExtend(t *testing.T) {
	tests := []struct {
		name      string
		elapsed   time.Duration
		ttl       time.Duration
		threshold float64
		expected  bool
	}{
		{
			name:      "below threshold",
			elapsed:   1 * time.Second,
			ttl:       5 * time.Second,
			threshold: 0.8,
			expected:  false,
		},
		{
			name:      "at threshold",
			elapsed:   4 * time.Second,
			ttl:       5 * time.Second,
			threshold: 0.8,
			expected:  true,
		},
		{
			name:      "above threshold",
			elapsed:   4500 * time.Millisecond,
			ttl:       5 * time.Second,
			threshold: 0.8,
			expected:  true,
		},
		{
			name:      "nil lock",
			elapsed:   0,
			ttl:       0,
			threshold: 0.8,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lock *Lock
			if tt.name != "nil lock" {
				lock = &Lock{
					Key:        "test",
					TTL:        tt.ttl,
					AcquiredAt: time.Now().Add(-tt.elapsed),
				}
			}
			result := ShouldExtend(lock, tt.threshold)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestReleaseNilLock(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	err := mgr.Release(ctx, nil)
	if err != ErrLockNotHeld {
		t.Fatalf("expected ErrLockNotHeld for nil lock, got: %v", err)
	}
}

func TestAcquireWithContextCancellation(t *testing.T) {
	client := testRedisClient(t)
	logger := zaptest.NewLogger(t)
	mgr := NewRedisLockManager(client, logger)

	cfg := LockConfig{
		TTL:             5 * time.Second,
		RetryCount:      3,
		RetryBaseDelay:  100 * time.Millisecond,
		RetryMultiplier: 2.0,
	}

	// Hold the lock so retries will occur
	ctx := context.Background()
	lock, err := mgr.Acquire(ctx, "cancel:test", cfg)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}
	defer mgr.Release(ctx, lock)

	// Cancel context during retries
	cancelCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()

	_, err = mgr.Acquire(cancelCtx, "cancel:test", cfg)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

func TestNewRedisLockManagerNilLogger(t *testing.T) {
	client := testRedisClient(t)
	mgr := NewRedisLockManager(client, nil)
	if mgr.logger == nil {
		t.Fatal("expected non-nil logger even when nil passed")
	}
}

func TestLockKeyPrefix(t *testing.T) {
	result := lockKeyPrefix("auction:123")
	expected := "lock:auction:123"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestFencingCounterKey(t *testing.T) {
	result := fencingCounterKey("auction:123")
	expected := "lock:fencing_counter:auction:123"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

// TestTTLExpiry verifies the lock expires naturally and logs a warning.
func TestTTLExpiry(t *testing.T) {
	client := testRedisClient(t)
	logger, _ := zap.NewDevelopment()
	mgr := NewRedisLockManager(client, logger)

	ctx := context.Background()
	cfg := LockConfig{
		TTL:        200 * time.Millisecond,
		RetryCount: 0,
	}

	lock, err := mgr.Acquire(ctx, "expiry:test", cfg)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Wait for expiry
	time.Sleep(300 * time.Millisecond)

	// Verify lock is gone
	redisKey := lockKeyPrefix(lock.Key)
	exists, err := client.Exists(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if exists != 0 {
		t.Error("expected lock to be expired")
	}

	// Release should report lock not held (expired)
	err = mgr.Release(ctx, lock)
	if err != ErrLockNotHeld {
		t.Errorf("expected ErrLockNotHeld after expiry, got: %v", err)
	}
}
