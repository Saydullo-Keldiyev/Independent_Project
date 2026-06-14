package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

const blacklistPrefix = "blacklist:"

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type Validator struct {
	secret []byte
	redis  *redis.Client
}

func NewValidator(secret string, redis *redis.Client) *Validator {
	return &Validator{secret: []byte(secret), redis: redis}
}

func (v *Validator) ParseBearer(authHeader string) (*Claims, string, error) {
	if authHeader == "" {
		return nil, "", errors.New("authorization required")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, "", errors.New("invalid authorization format")
	}
	tokenStr := parts[1]

	if v.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if val, err := v.redis.Get(ctx, blacklistPrefix+tokenStr).Result(); err == nil && val == "revoked" {
			return nil, "", errors.New("token revoked")
		}
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return v.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, "", errors.New("invalid token")
	}
	return claims, tokenStr, nil
}
