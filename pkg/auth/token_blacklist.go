package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const blacklistPrefix = "blacklist:"

// TokenBlacklist manages revoked JWT tokens in Redis.
// When a user logs out, their token is added here.
// Every auth check must verify the token is not blacklisted.
type TokenBlacklist struct {
	client *redis.Client
}

// NewTokenBlacklist creates a new blacklist backed by Redis
func NewTokenBlacklist(client *redis.Client) *TokenBlacklist {
	return &TokenBlacklist{client: client}
}

// Revoke adds a token to the blacklist with a TTL matching its expiry.
// ttl should be the remaining lifetime of the token so Redis auto-cleans it.
func (b *TokenBlacklist) Revoke(ctx context.Context, token string, ttl time.Duration) error {
	key := blacklistPrefix + token
	if err := b.client.Set(ctx, key, "revoked", ttl).Err(); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	return nil
}

// IsRevoked returns true if the token has been blacklisted
func (b *TokenBlacklist) IsRevoked(ctx context.Context, token string) (bool, error) {
	key := blacklistPrefix + token
	val, err := b.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // not in blacklist
	}
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return val == "revoked", nil
}
