package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DedupStore provides idempotency checking via a Redis processed-events store.
// Each processed event_id is stored with a configurable TTL (default 7 days).
type DedupStore struct {
	client    redis.Cmdable
	groupID   string
	ttl       time.Duration
	logger    *zap.Logger
}

// NewDedupStore creates a new deduplication store backed by Redis.
func NewDedupStore(client redis.Cmdable, groupID string, ttl time.Duration, logger *zap.Logger) *DedupStore {
	if ttl == 0 {
		ttl = 7 * 24 * time.Hour // 7 days default
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DedupStore{
		client:  client,
		groupID: groupID,
		ttl:     ttl,
		logger:  logger,
	}
}

// keyFor builds the Redis key for a given event_id.
// Format: processed:{consumer_group}:{event_id}
func (d *DedupStore) keyFor(eventID string) string {
	return fmt.Sprintf("processed:%s:%s", d.groupID, eventID)
}

// IsDuplicate checks whether the event has already been processed.
// Returns true if the event_id exists in the store (duplicate).
// If Redis is unavailable, returns false (process anyway) and logs a warning.
func (d *DedupStore) IsDuplicate(ctx context.Context, eventID string) (bool, error) {
	key := d.keyFor(eventID)
	exists, err := d.client.Exists(ctx, key).Result()
	if err != nil {
		d.logger.Warn("processed-events store unavailable, proceeding with processing",
			zap.String("event_id", eventID),
			zap.Error(err),
		)
		// If store is unavailable, process anyway per requirement 8.7
		return false, err
	}
	return exists > 0, nil
}

// MarkProcessed marks an event_id as processed with the configured TTL.
// If Redis is unavailable, logs a warning but does not return an error
// to the caller (degraded idempotency protection).
func (d *DedupStore) MarkProcessed(ctx context.Context, eventID string) error {
	key := d.keyFor(eventID)
	err := d.client.Set(ctx, key, "1", d.ttl).Err()
	if err != nil {
		d.logger.Warn("failed to mark event as processed in store",
			zap.String("event_id", eventID),
			zap.Error(err),
		)
		return err
	}
	return nil
}
