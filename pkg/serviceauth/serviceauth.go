// Package serviceauth provides service-to-service authentication using JWT (HMAC-SHA256).
// It supports signed service tokens with expiration ≤24h, automatic key rotation every 7 days
// with a 24h overlap period, and fail-closed behavior when the auth subsystem is unavailable.
package serviceauth

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// DefaultTokenTTL is the default expiration for service tokens (24h max per requirements).
	DefaultTokenTTL = 24 * time.Hour

	// DefaultRotationInterval is how often credentials are rotated (7 days).
	DefaultRotationInterval = 7 * 24 * time.Hour

	// DefaultOverlapPeriod is the overlap during which both old and new keys are accepted (24h).
	DefaultOverlapPeriod = 24 * time.Hour
)

var (
	ErrTokenExpired        = errors.New("service token has expired")
	ErrTokenInvalid        = errors.New("service token is invalid")
	ErrInvalidSigningMethod = errors.New("unexpected signing method")
	ErrMissingServiceID    = errors.New("missing service identity claim")
	ErrAuthUnavailable     = errors.New("authentication subsystem unavailable")
	ErrUnauthorized        = errors.New("unauthorized service request")
)

// ServiceClaims represents the JWT claims for service-to-service tokens.
type ServiceClaims struct {
	ServiceID string `json:"service_id"`
	jwt.RegisteredClaims
}

// Config holds the configuration for the service auth system.
type Config struct {
	// ServiceID is this service's identity (e.g., "api-gateway", "bid-service").
	ServiceID string

	// SigningKeys holds the current and optionally previous signing key.
	// Index 0 is the current key, index 1 (if present) is the previous key.
	SigningKeys []string

	// TokenTTL is the expiration duration for issued tokens. Must be ≤24h.
	TokenTTL time.Duration

	// RotationInterval is how often keys should be rotated (default 7 days).
	RotationInterval time.Duration

	// OverlapPeriod is how long old keys remain valid after rotation (default 24h).
	OverlapPeriod time.Duration

	// AllowedServices is the list of service IDs allowed to authenticate.
	// If empty, all valid service tokens are accepted.
	AllowedServices []string

	// Logger is the structured logger. If nil, a no-op logger is used.
	Logger *zap.Logger
}

// KeyInfo tracks when a key was activated.
type KeyInfo struct {
	Key       string
	ActivatedAt time.Time
}

// KeyManager handles signing key rotation with overlap periods.
type KeyManager struct {
	mu           sync.RWMutex
	currentKey   KeyInfo
	previousKey  *KeyInfo
	overlapPeriod time.Duration
	logger       *zap.Logger
}

// NewKeyManager creates a key manager with the given initial key(s).
func NewKeyManager(cfg *Config) *KeyManager {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	km := &KeyManager{
		currentKey: KeyInfo{
			Key:       cfg.SigningKeys[0],
			ActivatedAt: time.Now(),
		},
		overlapPeriod: cfg.OverlapPeriod,
		logger:       logger,
	}

	if len(cfg.SigningKeys) > 1 {
		km.previousKey = &KeyInfo{
			Key:       cfg.SigningKeys[1],
			ActivatedAt: time.Now().Add(-cfg.RotationInterval),
		}
	}

	return km
}

// RotateKey sets a new signing key, moving the current key to previous.
func (km *KeyManager) RotateKey(newKey string) {
	km.mu.Lock()
	defer km.mu.Unlock()

	km.previousKey = &KeyInfo{
		Key:       km.currentKey.Key,
		ActivatedAt: km.currentKey.ActivatedAt,
	}
	km.currentKey = KeyInfo{
		Key:       newKey,
		ActivatedAt: time.Now(),
	}

	km.logger.Info("service auth key rotated",
		zap.Time("new_key_activated_at", km.currentKey.ActivatedAt),
		zap.Duration("overlap_period", km.overlapPeriod),
	)
}

// CurrentKey returns the current signing key.
func (km *KeyManager) CurrentKey() string {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.currentKey.Key
}

// ValidationKeys returns all keys that should be tried during validation.
// The previous key is included only if we're within the overlap period.
func (km *KeyManager) ValidationKeys() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	keys := []string{km.currentKey.Key}

	if km.previousKey != nil {
		elapsed := time.Since(km.currentKey.ActivatedAt)
		if elapsed <= km.overlapPeriod {
			keys = append(keys, km.previousKey.Key)
		}
	}

	return keys
}

// TokenIssuer creates signed service tokens for outbound inter-service calls.
type TokenIssuer struct {
	serviceID  string
	keyManager *KeyManager
	tokenTTL   time.Duration
	logger     *zap.Logger
}

// NewTokenIssuer creates a token issuer for the given service.
func NewTokenIssuer(cfg *Config, keyManager *KeyManager) *TokenIssuer {
	ttl := cfg.TokenTTL
	if ttl == 0 || ttl > DefaultTokenTTL {
		ttl = DefaultTokenTTL
	}

	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &TokenIssuer{
		serviceID:  cfg.ServiceID,
		keyManager: keyManager,
		tokenTTL:   ttl,
		logger:     logger,
	}
}

// IssueToken creates a signed JWT service token for outbound calls.
func (ti *TokenIssuer) IssueToken() (string, error) {
	now := time.Now()
	claims := ServiceClaims{
		ServiceID: ti.serviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    ti.serviceID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ti.tokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	key := ti.keyManager.CurrentKey()

	signed, err := token.SignedString([]byte(key))
	if err != nil {
		ti.logger.Error("failed to sign service token",
			zap.String("service_id", ti.serviceID),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to sign service token: %w", err)
	}

	return signed, nil
}

// TokenValidator validates incoming service tokens.
type TokenValidator struct {
	keyManager      *KeyManager
	allowedServices map[string]bool
	available       bool
	mu              sync.RWMutex
	logger          *zap.Logger
}

// NewTokenValidator creates a validator that checks service tokens.
func NewTokenValidator(cfg *Config, keyManager *KeyManager) *TokenValidator {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	allowed := make(map[string]bool)
	for _, svc := range cfg.AllowedServices {
		allowed[svc] = true
	}

	return &TokenValidator{
		keyManager:      keyManager,
		allowedServices: allowed,
		available:       true,
		logger:          logger,
	}
}

// SetAvailable sets the availability state of the auth subsystem.
// When unavailable, all requests are rejected (fail-closed).
func (tv *TokenValidator) SetAvailable(available bool) {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	tv.available = available

	if !available {
		tv.logger.Warn("service auth subsystem marked unavailable - all requests will be rejected")
	} else {
		tv.logger.Info("service auth subsystem marked available")
	}
}

// IsAvailable returns whether the auth subsystem is available.
func (tv *TokenValidator) IsAvailable() bool {
	tv.mu.RLock()
	defer tv.mu.RUnlock()
	return tv.available
}

// ValidateToken validates a service token string and returns the claims.
// Returns ErrAuthUnavailable if the auth subsystem is marked unavailable (fail-closed).
func (tv *TokenValidator) ValidateToken(tokenString string) (*ServiceClaims, error) {
	// Fail-closed: reject all when auth subsystem unavailable
	if !tv.IsAvailable() {
		return nil, ErrAuthUnavailable
	}

	keys := tv.keyManager.ValidationKeys()

	var lastErr error
	for _, key := range keys {
		claims, err := tv.parseToken(tokenString, key)
		if err == nil {
			// Verify service identity is allowed
			if len(tv.allowedServices) > 0 && !tv.allowedServices[claims.ServiceID] {
				tv.logger.Warn("service token from unauthorized service",
					zap.String("service_id", claims.ServiceID),
				)
				return nil, ErrUnauthorized
			}
			return claims, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

// parseToken parses and validates a JWT token with the given key.
func (tv *TokenValidator) parseToken(tokenString, key string) (*ServiceClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&ServiceClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("%w: %v", ErrInvalidSigningMethod, t.Header["alg"])
			}
			return []byte(key), nil
		},
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*ServiceClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	if claims.ServiceID == "" {
		return nil, ErrMissingServiceID
	}

	return claims, nil
}
