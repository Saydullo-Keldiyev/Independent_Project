package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var Client *goredis.Client

type Options struct {
	Addr     string
	Password string
	DB       int
}

func Connect(opts Options) error {
	Client = goredis.NewClient(&goredis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

func Close() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

func Ping(ctx context.Context) error {
	if Client == nil {
		return fmt.Errorf("redis not connected")
	}
	return Client.Ping(ctx).Err()
}
