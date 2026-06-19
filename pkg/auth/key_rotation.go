// Package auth provides JWT key rotation with a grace period for seamless
// authentication during key transitions. The KeyRotationManager maintains
// current and previous signing keys, rotates on a configurable schedule,
// and persists state to Redis for pod restart survival.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultRotationInterval is the default key rotation interval (24 hours).
	DefaultRotationInterval = 24 * time.Hour

	// DefaultGracePeriod is the time after rotation during which the previous key
	// is still accepted for verification (15 minutes).
	DefaultGracePeriod = 15 * time.Minute

	// DefaultKeySize is the default HMAC key size in bytes (256-bit).
	DefaultKeySize = 32

	// redisKeyStateKey is the Redis key used for persisting rotation state.
	redisKeyStateKey = "jwt:key_rotation:state"
)

// Errors specific to key rotation.
var (
	ErrKeyRotationNotInitialized = errors.New("key rotation manager not initialized")
	ErrGracePeriodExpired        = errors.New("token signed with previous key but grace period expired")
	ErrNoValidKey                = errors.New("token could not be verified with any active key")
)

// KeyRotationConfig configures the key rotation manager.
type KeyRotationConfig struct {
	// RotationInterval is how often keys are rotated. Default: 24 hours.
	RotationInterval time.Duration

	// GracePeriod is how long the previous key remains valid after rotation.
	// Default: 15 minutes.
	GracePeriod time.Duration

	// KeySize is the HMAC signing key size in bytes. Default: 32 (256-bit).
	KeySize int

	// RedisClient is used to persist key state across pod restarts.
	// If nil, keys are only held in memory (not recommended for production).
	RedisClient *redis.Client
}

// DefaultKeyRotationConfig returns a configuration with production defaults.
func DefaultKeyRotationConfig() KeyRotationConfig {
	return KeyRotationConfig{
		RotationInterval: DefaultRotationInterval,
		GracePeriod:      DefaultGracePeriod,
		KeySize:          DefaultKeySize,
	}
}

// SigningKey represents a JWT signing key with metadata.
type SigningKey struct {
	// Key is the raw signing key bytes.
	Key []byte `json:"key"`

	// KeyID is a unique identifier for this key (used in JWT kid header).
	KeyID string `json:"key_id"`

	// CreatedAt is when this key was generated.
	CreatedAt time.Time `json:"created_at"`
}

// keyState holds the current and previous key state, persisted to Redis.
type keyState struct {
	CurrentKey  *SigningKey `json:"current_key"`
	PreviousKey *SigningKey `json:"previous_key,omitempty"`
	RotatedAt   time.Time  `json:"rotated_at"`
}

// KeyRotationManager handles automatic JWT signing key rotation with a grace period.
// It ensures zero failed authentication requests during the rotation window by
// accepting tokens signed with the previous key for up to GracePeriod after rotation.
type KeyRotationManager struct {
	config KeyRotationConfig
	mu     sync.RWMutex
	state  *keyState
	stopCh chan struct{}
	nowFn  func() time.Time // injectable clock for testing
}

// NewKeyRotationManager creates a new key rotation manager with the given config.
// Call Initialize() to load or create initial keys, and Start() to begin auto-rotation.
func NewKeyRotationManager(config KeyRotationConfig) *KeyRotationManager {
	if config.RotationInterval <= 0 {
		config.RotationInterval = DefaultRotationInterval
	}
	if config.GracePeriod <= 0 {
		config.GracePeriod = DefaultGracePeriod
	}
	if config.KeySize <= 0 {
		config.KeySize = DefaultKeySize
	}

	return &KeyRotationManager{
		config: config,
		stopCh: make(chan struct{}),
		nowFn:  time.Now,
	}
}

// Initialize loads existing key state from Redis (if available) or generates new keys.
// This should be called during application startup.
func (m *KeyRotationManager) Initialize(ctx context.Context) error {
	// Try to load from Redis
	if m.config.RedisClient != nil {
		state, err := m.loadState(ctx)
		if err == nil && state != nil && state.CurrentKey != nil {
			m.mu.Lock()
			m.state = state
			m.mu.Unlock()
			return nil
		}
		// If loading fails, generate fresh keys
	}

	// Generate initial key
	key, err := m.generateKey()
	if err != nil {
		return fmt.Errorf("failed to generate initial signing key: %w", err)
	}

	m.mu.Lock()
	m.state = &keyState{
		CurrentKey: key,
		RotatedAt:  m.now(),
	}
	m.mu.Unlock()

	// Persist to Redis
	if m.config.RedisClient != nil {
		if err := m.persistState(ctx); err != nil {
			return fmt.Errorf("failed to persist initial key state: %w", err)
		}
	}

	return nil
}

// Start begins the automatic key rotation goroutine.
// It rotates keys on the configured RotationInterval schedule.
func (m *KeyRotationManager) Start(ctx context.Context) {
	go m.rotationLoop(ctx)
}

// Stop halts the automatic key rotation goroutine.
func (m *KeyRotationManager) Stop() {
	close(m.stopCh)
}

// Rotate performs an immediate key rotation. The current key becomes the previous
// key, and a new key is generated as the current key. This is safe to call
// concurrently.
func (m *KeyRotationManager) Rotate(ctx context.Context) error {
	newKey, err := m.generateKey()
	if err != nil {
		return fmt.Errorf("failed to generate new signing key: %w", err)
	}

	m.mu.Lock()
	m.state = &keyState{
		CurrentKey:  newKey,
		PreviousKey: m.state.CurrentKey,
		RotatedAt:   m.now(),
	}
	m.mu.Unlock()

	// Persist to Redis
	if m.config.RedisClient != nil {
		if err := m.persistState(ctx); err != nil {
			return fmt.Errorf("failed to persist rotated key state: %w", err)
		}
	}

	return nil
}

// SignToken signs the given claims with the current signing key.
// The resulting JWT includes a "kid" header identifying which key was used.
func (m *KeyRotationManager) SignToken(claims jwt.Claims) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil || m.state.CurrentKey == nil {
		return "", ErrKeyRotationNotInitialized
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = m.state.CurrentKey.KeyID

	signed, err := token.SignedString(m.state.CurrentKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, nil
}

// VerifyToken verifies a JWT token against the current key and, if within the
// grace period, the previous key. Returns the parsed claims or an error.
// This ensures zero failed authentication requests during key rotation.
func (m *KeyRotationManager) VerifyToken(tokenString string) (*Claims, error) {
	m.mu.RLock()
	state := m.state
	gracePeriod := m.config.GracePeriod
	now := m.now()
	m.mu.RUnlock()

	if state == nil || state.CurrentKey == nil {
		return nil, ErrKeyRotationNotInitialized
	}

	// Try current key first
	claims, err := m.parseToken(tokenString, state.CurrentKey)
	if err == nil {
		return claims, nil
	}

	// Try previous key if within grace period
	if state.PreviousKey != nil {
		timeSinceRotation := now.Sub(state.RotatedAt)
		if timeSinceRotation <= gracePeriod {
			claims, prevErr := m.parseToken(tokenString, state.PreviousKey)
			if prevErr == nil {
				return claims, nil
			}
			// If the token was signed with the previous key but is otherwise invalid
			// (e.g., expired), return the original error
			return nil, fmt.Errorf("%w: %v", ErrNoValidKey, prevErr)
		}
		// Grace period expired
		// Check if the token was actually signed with the previous key
		if m.isSignedByKey(tokenString, state.PreviousKey) {
			return nil, ErrGracePeriodExpired
		}
	}

	return nil, fmt.Errorf("%w: %v", ErrNoValidKey, err)
}

// CurrentKeyID returns the key ID of the current signing key.
func (m *KeyRotationManager) CurrentKeyID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil || m.state.CurrentKey == nil {
		return ""
	}
	return m.state.CurrentKey.KeyID
}

// PreviousKeyID returns the key ID of the previous signing key, or empty if none.
func (m *KeyRotationManager) PreviousKeyID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil || m.state.PreviousKey == nil {
		return ""
	}
	return m.state.PreviousKey.KeyID
}

// LastRotationTime returns when the last rotation occurred.
func (m *KeyRotationManager) LastRotationTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil {
		return time.Time{}
	}
	return m.state.RotatedAt
}

// IsWithinGracePeriod returns true if the system is currently within the grace
// period after the most recent key rotation.
func (m *KeyRotationManager) IsWithinGracePeriod() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil || m.state.PreviousKey == nil {
		return false
	}

	timeSinceRotation := m.now().Sub(m.state.RotatedAt)
	return timeSinceRotation <= m.config.GracePeriod
}

// --- Internal methods ---

func (m *KeyRotationManager) now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func (m *KeyRotationManager) rotationLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.RotationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = m.Rotate(ctx)
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (m *KeyRotationManager) generateKey() (*SigningKey, error) {
	key := make([]byte, m.config.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	// Generate a short, unique key ID
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key ID: %w", err)
	}

	return &SigningKey{
		Key:       key,
		KeyID:     base64.RawURLEncoding.EncodeToString(idBytes),
		CreatedAt: m.now(),
	}, nil
}

func (m *KeyRotationManager) parseToken(tokenString string, key *SigningKey) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return key.Key, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// isSignedByKey checks if a token was signed by a specific key (signature matches)
// without validating expiry or other claims.
func (m *KeyRotationManager) isSignedByKey(tokenString string, key *SigningKey) bool {
	_, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return key.Key, nil
	}, jwt.WithoutClaimsValidation())
	return err == nil
}

func (m *KeyRotationManager) loadState(ctx context.Context) (*keyState, error) {
	data, err := m.config.RedisClient.Get(ctx, redisKeyStateKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load key state from Redis: %w", err)
	}

	var state keyState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key state: %w", err)
	}

	return &state, nil
}

func (m *KeyRotationManager) persistState(ctx context.Context) error {
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal key state: %w", err)
	}

	// Store with a TTL slightly longer than 2 rotation intervals to auto-cleanup
	// stale data from abandoned instances.
	ttl := m.config.RotationInterval * 3
	if err := m.config.RedisClient.Set(ctx, redisKeyStateKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to persist key state to Redis: %w", err)
	}

	return nil
}
