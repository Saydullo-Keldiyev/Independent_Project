package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var Ctx = context.Background()

type Options struct{ Addr, Password string; DB int }

func Connect(opts Options) error {
	Client = redis.NewClient(&redis.Options{
		Addr: opts.Addr, Password: opts.Password, DB: opts.DB,
		PoolSize: 15, DialTimeout: 5 * time.Second,
	})
	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	defer cancel()
	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

func Close() { if Client != nil { Client.Close() } }

// ── Auction cache ─────────────────────────────────────────────────────────────

const auctionPrefix = "auction:"
const auctionTTL = 10 * time.Minute

func CacheAuction(id string, data any) error {
	b, _ := json.Marshal(data)
	return Client.Set(Ctx, auctionPrefix+id, b, auctionTTL).Err()
}

func GetCachedAuction(id string) ([]byte, error) {
	val, err := Client.Get(Ctx, auctionPrefix+id).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func InvalidateAuction(id string) {
	Client.Del(Ctx, auctionPrefix+id)
}

// ── Distributed lock for scheduler ───────────────────────────────────────────

func AcquireSchedulerLock(lockName string, ttl time.Duration) (bool, error) {
	ok, err := Client.SetNX(Ctx, "lock:"+lockName, "1", ttl).Result()
	return ok, err
}

func ReleaseSchedulerLock(lockName string) {
	Client.Del(Ctx, "lock:"+lockName)
}
