package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var ctx = context.Background()

type Options struct {
	Addr     string
	Password string
	DB       int
}

func Connect(opts Options) error {
	Client = redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
		PoolSize: 15,
	})

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connect: %w", err)
	}
	return nil
}

func Close() {
	if Client != nil {
		Client.Close()
	}
}

// ── Search result caching ─────────────────────────────────────────────────────

func GetCached(key string) ([]byte, error) {
	val, err := Client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // cache miss
	}
	return val, err
}

func SetCached(key string, data any, ttl time.Duration) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return Client.Set(ctx, key, bytes, ttl).Err()
}

// CacheKey generates a deterministic cache key from search params
func CacheKey(prefix, query, category, state, sortBy string, page int) string {
	return fmt.Sprintf("search:%s:%s:%s:%s:%s:%d", prefix, query, category, state, sortBy, page)
}

// ── Trending searches ─────────────────────────────────────────────────────────

const trendingKey = "trending:searches"

// RecordSearch increments the search count for a query (sorted set)
func RecordSearch(query string) {
	if query == "" {
		return
	}
	Client.ZIncrBy(ctx, trendingKey, 1, query)
	// Set TTL so old data expires
	Client.Expire(ctx, trendingKey, 24*time.Hour)
}

// GetTrendingSearches returns top N searched queries
func GetTrendingSearches(limit int) ([]redis.Z, error) {
	return Client.ZRevRangeWithScores(ctx, trendingKey, 0, int64(limit-1)).Result()
}
