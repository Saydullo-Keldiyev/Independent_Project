package lock

import (
	"context"
	"fmt"
	"time"

	redisPkg "github.com/auction-system/bid-service/internal/redis"
)

const defaultLockTTL = 5 * time.Second

type DistributedLock struct {
	key string
	ttl time.Duration
}

// NewLock creates a new distributed lock for the given auction
func NewLock(auctionID string, ttl time.Duration) *DistributedLock {
	if ttl == 0 {
		ttl = defaultLockTTL
	}
	return &DistributedLock{
		key: fmt.Sprintf("auction_lock:%s", auctionID),
		ttl: ttl,
	}
}

// Acquire tries to acquire the lock. Returns true if successful.
// Uses SET NX (set if not exists) — atomic Redis operation.
func (l *DistributedLock) Acquire(ctx context.Context) (bool, error) {
	ok, err := redisPkg.Client.SetNX(ctx, l.key, "locked", l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock for key %s: %w", l.key, err)
	}
	return ok, nil
}

// Release deletes the lock key from Redis
func (l *DistributedLock) Release(ctx context.Context) error {
	if err := redisPkg.Client.Del(ctx, l.key).Err(); err != nil {
		return fmt.Errorf("failed to release lock for key %s: %w", l.key, err)
	}
	return nil
}

// AcquireLock is a convenience function for simple use cases
func AcquireLock(key string) bool {
	ok, _ := redisPkg.Client.SetNX(
		redisPkg.Ctx,
		key,
		"locked",
		defaultLockTTL,
	).Result()
	return ok
}

// ReleaseLock is a convenience function for simple use cases
func ReleaseLock(key string) {
	redisPkg.Client.Del(redisPkg.Ctx, key)
}
