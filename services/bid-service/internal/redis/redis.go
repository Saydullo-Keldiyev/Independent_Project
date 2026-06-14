package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var Ctx = context.Background()

type Options struct {
	Addr     string
	Password string
	DB       int
}

func Connect(opts Options) error {
	Client = redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     20,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	return nil
}

func Close() {
	if Client != nil {
		Client.Close()
	}
}

// SetHighestBid caches the current highest bid for an auction
func SetHighestBid(auctionID string, amount float64, ttl time.Duration) error {
	key := fmt.Sprintf("auction:highest_bid:%s", auctionID)
	return Client.Set(Ctx, key, amount, ttl).Err()
}

// GetHighestBid returns the cached highest bid for an auction
func GetHighestBid(auctionID string) (float64, error) {
	key := fmt.Sprintf("auction:highest_bid:%s", auctionID)
	val, err := Client.Get(Ctx, key).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}
