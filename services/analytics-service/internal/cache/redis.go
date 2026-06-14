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
		Addr: opts.Addr, Password: opts.Password, DB: opts.DB, PoolSize: 15,
	})
	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

func Close() {
	if Client != nil {
		Client.Close()
	}
}

// ── Realtime counters ─────────────────────────────────────────────────────────

func IncrBidsToday() {
	key := fmt.Sprintf("analytics:bids:%s", time.Now().Format("2006-01-02"))
	Client.Incr(ctx, key)
	Client.Expire(ctx, key, 48*time.Hour)
}

func GetBidsToday() int64 {
	key := fmt.Sprintf("analytics:bids:%s", time.Now().Format("2006-01-02"))
	val, _ := Client.Get(ctx, key).Int64()
	return val
}

func IncrActiveUsers(userID string) {
	Client.SAdd(ctx, "analytics:active_users:"+time.Now().Format("2006-01-02"), userID)
	Client.Expire(ctx, "analytics:active_users:"+time.Now().Format("2006-01-02"), 48*time.Hour)
}

func GetActiveUsersToday() int64 {
	val, _ := Client.SCard(ctx, "analytics:active_users:"+time.Now().Format("2006-01-02")).Result()
	return val
}

func IncrRevenue(amount float64) {
	key := fmt.Sprintf("analytics:revenue:%s", time.Now().Format("2006-01-02"))
	Client.IncrByFloat(ctx, key, amount)
	Client.Expire(ctx, key, 48*time.Hour)
}

func GetRevenueToday() float64 {
	key := fmt.Sprintf("analytics:revenue:%s", time.Now().Format("2006-01-02"))
	val, _ := Client.Get(ctx, key).Float64()
	return val
}

// ── Leaderboards ──────────────────────────────────────────────────────────────

func IncrSellerRevenue(sellerID string, amount float64) {
	Client.ZIncrBy(ctx, "analytics:leaderboard:sellers", amount, sellerID)
}

func IncrBidderActivity(bidderID string) {
	Client.ZIncrBy(ctx, "analytics:leaderboard:bidders", 1, bidderID)
}

func IncrCategoryActivity(category string) {
	Client.ZIncrBy(ctx, "analytics:leaderboard:categories", 1, category)
}

func GetTopSellers(limit int) ([]redis.Z, error) {
	return Client.ZRevRangeWithScores(ctx, "analytics:leaderboard:sellers", 0, int64(limit-1)).Result()
}

func GetTopBidders(limit int) ([]redis.Z, error) {
	return Client.ZRevRangeWithScores(ctx, "analytics:leaderboard:bidders", 0, int64(limit-1)).Result()
}

func GetTopCategories(limit int) ([]redis.Z, error) {
	return Client.ZRevRangeWithScores(ctx, "analytics:leaderboard:categories", 0, int64(limit-1)).Result()
}

// ── Dashboard cache ───────────────────────────────────────────────────────────

func GetCached(key string) ([]byte, error) {
	val, err := Client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func SetCached(key string, data any, ttl time.Duration) error {
	b, _ := json.Marshal(data)
	return Client.Set(ctx, key, b, ttl).Err()
}
