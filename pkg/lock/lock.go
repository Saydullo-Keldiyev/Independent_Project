// Package lock provides a production-grade distributed lock with fencing tokens.
// It uses Redis for storage with monotonically increasing fencing tokens via INCR,
// ownership verification on release, retry with exponential backoff, and TTL auto-extension.
package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Errors returned by the lock manager.
var (
	ErrLockNotAcquired   = errors.New("lock: failed to acquire lock after retries")
	ErrOwnershipMismatch = errors.New("lock: ownership mismatch on release")
	ErrLockNotHeld       = errors.New("lock: lock is not held")
	ErrExtendFailed      = errors.New("lock: failed to extend TTL")
)

// LockConfig configures the behavior of a distributed lock acquisition.
type LockConfig struct {
	TTL             time.Duration // TTL for the lock key; default 5s
	RetryCount      int           // Number of acquisition retries; default 3
	RetryBaseDelay  time.Duration // Base delay for exponential backoff; default 50ms
	RetryMultiplier float64       // Backoff multiplier; default 2.0
	ExtendThreshold float64       // Fraction of TTL at which auto-extension triggers; default 0.8
}

// DefaultLockConfig returns a LockConfig with production defaults.
func DefaultLockConfig() LockConfig {
	return LockConfig{
		TTL:             5 * time.Second,
		RetryCount:      3,
		RetryBaseDelay:  50 * time.Millisecond,
		RetryMultiplier: 2.0,
		ExtendThreshold: 0.8,
	}
}

// Lock represents a held distributed lock.
type Lock struct {
	Key          string    `json:"key"`
	Owner        string    `json:"owner"`
	FencingToken int64     `json:"fencing_token"`
	TTL          time.Duration `json:"-"`
	AcquiredAt   time.Time `json:"acquired_at"`
}

// lockValue is the JSON structure stored in Redis for a lock.
type lockValue struct {
	Owner        string `json:"owner"`
	FencingToken int64  `json:"fencing_token"`
	AcquiredAt   string `json:"acquired_at"`
}

// LockManager defines the interface for distributed lock operations.
type LockManager interface {
	// Acquire attempts to acquire a distributed lock on the given key.
	// Returns the Lock with a unique fencing token on success.
	Acquire(ctx context.Context, key string, cfg LockConfig) (*Lock, error)

	// Release releases a held lock after verifying ownership via fencing token and owner.
	Release(ctx context.Context, lock *Lock) error

	// Extend extends the TTL of a held lock after verifying ownership.
	Extend(ctx context.Context, lock *Lock) error
}

// RedisLockManager implements LockManager using Redis.
type RedisLockManager struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisLockManager creates a new RedisLockManager.
func NewRedisLockManager(client *redis.Client, logger *zap.Logger) *RedisLockManager {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	return &RedisLockManager{
		client: client,
		logger: logger,
	}
}

// lockKeyPrefix returns the full Redis key for a lock.
func lockKeyPrefix(key string) string {
	return fmt.Sprintf("lock:%s", key)
}

// fencingCounterKey returns the Redis key for the fencing token counter.
func fencingCounterKey(key string) string {
	// Extract resource type from key (e.g., "auction:123" → "auction")
	return fmt.Sprintf("lock:fencing_counter:%s", key)
}

// Acquire attempts to acquire a distributed lock with retry and exponential backoff.
func (m *RedisLockManager) Acquire(ctx context.Context, key string, cfg LockConfig) (*Lock, error) {
	if cfg.TTL == 0 {
		cfg.TTL = 5 * time.Second
	}
	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}
	if cfg.RetryBaseDelay == 0 {
		cfg.RetryBaseDelay = 50 * time.Millisecond
	}
	if cfg.RetryMultiplier == 0 {
		cfg.RetryMultiplier = 2.0
	}
	if cfg.ExtendThreshold == 0 {
		cfg.ExtendThreshold = 0.8
	}

	owner := uuid.New().String()
	redisKey := lockKeyPrefix(key)

	var lastErr error
	delay := cfg.RetryBaseDelay

	for attempt := 0; attempt <= cfg.RetryCount; attempt++ {
		if attempt > 0 {
			// Wait with exponential backoff before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay = time.Duration(float64(delay) * cfg.RetryMultiplier)
		}

		lock, err := m.tryAcquire(ctx, redisKey, key, owner, cfg.TTL)
		if err == nil {
			return lock, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("%w: key=%s, last error: %v", ErrLockNotAcquired, key, lastErr)
}

// tryAcquire makes a single lock acquisition attempt using Redis SET NX + INCR for fencing token.
func (m *RedisLockManager) tryAcquire(ctx context.Context, redisKey, logicalKey, owner string, ttl time.Duration) (*Lock, error) {
	// Generate fencing token via INCR (monotonically increasing)
	counterKey := fencingCounterKey(logicalKey)
	fencingToken, err := m.client.Incr(ctx, counterKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to generate fencing token: %w", err)
	}

	acquiredAt := time.Now().UTC()
	val := lockValue{
		Owner:        owner,
		FencingToken: fencingToken,
		AcquiredAt:   acquiredAt.Format(time.RFC3339Nano),
	}

	valBytes, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lock value: %w", err)
	}

	// Attempt SET NX (only set if key doesn't exist)
	ok, err := m.client.SetNX(ctx, redisKey, string(valBytes), ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to set lock: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("lock already held by another owner")
	}

	return &Lock{
		Key:          logicalKey,
		Owner:        owner,
		FencingToken: fencingToken,
		TTL:          ttl,
		AcquiredAt:   acquiredAt,
	}, nil
}

// Release releases a held lock after verifying ownership.
// Uses a Lua script to atomically check ownership and delete.
func (m *RedisLockManager) Release(ctx context.Context, lock *Lock) error {
	if lock == nil {
		return ErrLockNotHeld
	}

	redisKey := lockKeyPrefix(lock.Key)

	// Lua script: verify owner + fencing token, then delete
	script := redis.NewScript(`
		local val = redis.call("GET", KEYS[1])
		if val == false then
			return -1
		end
		local lock = cjson.decode(val)
		if lock.owner == ARGV[1] and lock.fencing_token == tonumber(ARGV[2]) then
			redis.call("DEL", KEYS[1])
			return 1
		end
		return 0
	`)

	result, err := script.Run(ctx, m.client, []string{redisKey},
		lock.Owner,
		lock.FencingToken,
	).Int64()
	if err != nil {
		return fmt.Errorf("failed to execute release script: %w", err)
	}

	switch result {
	case 1:
		return nil
	case 0:
		return ErrOwnershipMismatch
	case -1:
		// Lock already expired
		m.logger.Warn("lock already expired during release",
			zap.String("key", lock.Key),
			zap.String("owner", lock.Owner),
			zap.Int64("fencing_token", lock.FencingToken),
			zap.Duration("elapsed", time.Since(lock.AcquiredAt)),
		)
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unexpected release result: %d", result)
	}
}

// Extend extends the TTL of a held lock after verifying ownership.
// Uses a Lua script to atomically check ownership and extend TTL.
func (m *RedisLockManager) Extend(ctx context.Context, lock *Lock) error {
	if lock == nil {
		return ErrLockNotHeld
	}

	redisKey := lockKeyPrefix(lock.Key)

	// Lua script: verify owner + fencing token, then extend TTL
	script := redis.NewScript(`
		local val = redis.call("GET", KEYS[1])
		if val == false then
			return -1
		end
		local lock = cjson.decode(val)
		if lock.owner == ARGV[1] and lock.fencing_token == tonumber(ARGV[2]) then
			redis.call("PEXPIRE", KEYS[1], ARGV[3])
			return 1
		end
		return 0
	`)

	ttlMs := lock.TTL.Milliseconds()
	result, err := script.Run(ctx, m.client, []string{redisKey},
		lock.Owner,
		lock.FencingToken,
		ttlMs,
	).Int64()
	if err != nil {
		m.logger.Warn("TTL extension failed",
			zap.String("key", lock.Key),
			zap.String("owner", lock.Owner),
			zap.Error(err),
		)
		return fmt.Errorf("%w: %v", ErrExtendFailed, err)
	}

	switch result {
	case 1:
		return nil
	case 0:
		m.logger.Warn("TTL extension failed: ownership mismatch",
			zap.String("key", lock.Key),
			zap.String("owner", lock.Owner),
			zap.Int64("fencing_token", lock.FencingToken),
		)
		return ErrOwnershipMismatch
	case -1:
		m.logger.Warn("TTL extension failed: lock expired",
			zap.String("key", lock.Key),
			zap.String("owner", lock.Owner),
			zap.Int64("fencing_token", lock.FencingToken),
			zap.Duration("elapsed", time.Since(lock.AcquiredAt)),
		)
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unexpected extend result: %d", result)
	}
}

// ShouldExtend returns true if the lock has been held beyond the ExtendThreshold of its TTL.
func ShouldExtend(lock *Lock, threshold float64) bool {
	if lock == nil {
		return false
	}
	elapsed := time.Since(lock.AcquiredAt)
	thresholdDuration := time.Duration(float64(lock.TTL) * threshold)
	return elapsed >= thresholdDuration
}
