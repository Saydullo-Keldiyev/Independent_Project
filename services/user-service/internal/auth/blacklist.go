package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/auction-system/user-service/internal/redis"
)

const blacklistPrefix = "jwt:blacklist:"

func BlacklistJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if redis.Client == nil {
		return fmt.Errorf("redis not available")
	}
	return redis.Client.Set(ctx, blacklistPrefix+jti, "1", ttl).Err()
}

func IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	if redis.Client == nil {
		return false, nil
	}
	n, err := redis.Client.Exists(ctx, blacklistPrefix+jti).Result()
	return n > 0, err
}
