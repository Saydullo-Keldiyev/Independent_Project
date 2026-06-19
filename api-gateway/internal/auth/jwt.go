package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	pkgauth "github.com/auction-system/pkg/auth"
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
	secret         []byte
	redis          *redis.Client
	keyRotationMgr *pkgauth.KeyRotationManager
}

// NewValidator creates a Validator using a static secret for token verification.
func NewValidator(secret string, redis *redis.Client) *Validator {
	return &Validator{secret: []byte(secret), redis: redis}
}

// NewValidatorWithKeyRotation creates a Validator that uses the key rotation manager
// for token verification, supporting graceful key rotation with zero auth failures.
func NewValidatorWithKeyRotation(keyRotationMgr *pkgauth.KeyRotationManager, redis *redis.Client) *Validator {
	return &Validator{
		keyRotationMgr: keyRotationMgr,
		redis:          redis,
	}
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

	// Use key rotation manager if available; otherwise fall back to static secret
	if v.keyRotationMgr != nil {
		pkgClaims, err := v.keyRotationMgr.VerifyToken(tokenStr)
		if err != nil {
			return nil, "", errors.New("invalid token")
		}
		// Convert pkg/auth Claims to gateway Claims
		claims := &Claims{
			UserID:           pkgClaims.UserID,
			Email:            pkgClaims.Email,
			Role:             pkgClaims.Role,
			RegisteredClaims: pkgClaims.RegisteredClaims,
		}
		return claims, tokenStr, nil
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
