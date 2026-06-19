package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Helper to create a manager with a controllable clock
func newTestManager(rotationInterval, gracePeriod time.Duration) *KeyRotationManager {
	config := KeyRotationConfig{
		RotationInterval: rotationInterval,
		GracePeriod:      gracePeriod,
		KeySize:          32,
		RedisClient:      nil, // in-memory only for tests
	}
	return NewKeyRotationManager(config)
}

func TestKeyRotationManager_Initialize(t *testing.T) {
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if mgr.CurrentKeyID() == "" {
		t.Fatal("expected non-empty current key ID after initialization")
	}

	if mgr.PreviousKeyID() != "" {
		t.Fatal("expected empty previous key ID after fresh initialization")
	}
}

func TestKeyRotationManager_SignAndVerify(t *testing.T) {
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	claims := &Claims{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "buyer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("expected non-empty token string")
	}

	// Verify the token
	parsed, err := mgr.VerifyToken(tokenStr)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}

	if parsed.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", parsed.UserID)
	}
	if parsed.Email != "test@example.com" {
		t.Errorf("expected Email=test@example.com, got %s", parsed.Email)
	}
	if parsed.Role != "buyer" {
		t.Errorf("expected Role=buyer, got %s", parsed.Role)
	}
}

func TestKeyRotationManager_Rotate(t *testing.T) {
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	originalKeyID := mgr.CurrentKeyID()

	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	newKeyID := mgr.CurrentKeyID()
	prevKeyID := mgr.PreviousKeyID()

	if newKeyID == originalKeyID {
		t.Error("expected current key ID to change after rotation")
	}

	if prevKeyID != originalKeyID {
		t.Errorf("expected previous key to be the old current key; got prevKeyID=%s, originalKeyID=%s",
			prevKeyID, originalKeyID)
	}
}

func TestKeyRotationManager_GracePeriod_AcceptsPreviousKey(t *testing.T) {
	now := time.Now()
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	mgr.nowFn = func() time.Time { return now }
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Sign a token with the current (soon-to-be-previous) key
	claims := &Claims{
		UserID: "user-456",
		Email:  "grace@example.com",
		Role:   "seller",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	tokenStr, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed: %v", err)
	}

	// Rotate the key — the old token should still work within grace period
	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// Advance time by 5 minutes (within 15-min grace period)
	mgr.nowFn = func() time.Time { return now.Add(5 * time.Minute) }

	// The token signed with the previous key should still verify
	parsed, err := mgr.VerifyToken(tokenStr)
	if err != nil {
		t.Fatalf("VerifyToken should accept previous key within grace period, got: %v", err)
	}

	if parsed.UserID != "user-456" {
		t.Errorf("expected UserID=user-456, got %s", parsed.UserID)
	}
}

func TestKeyRotationManager_GracePeriod_RejectsAfterExpiry(t *testing.T) {
	now := time.Now()
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	mgr.nowFn = func() time.Time { return now }
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Sign a token with the current (soon-to-be-previous) key
	claims := &Claims{
		UserID: "user-789",
		Email:  "expired@example.com",
		Role:   "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	tokenStr, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed: %v", err)
	}

	// Rotate the key
	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// Advance time beyond grace period (16 minutes > 15 min grace)
	mgr.nowFn = func() time.Time { return now.Add(16 * time.Minute) }

	// The token signed with the previous key should be rejected
	_, err = mgr.VerifyToken(tokenStr)
	if err == nil {
		t.Fatal("VerifyToken should reject token signed with previous key after grace period")
	}
}

func TestKeyRotationManager_NewTokenVerifiesAfterRotation(t *testing.T) {
	now := time.Now()
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	mgr.nowFn = func() time.Time { return now }
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Rotate the key
	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// Sign a NEW token (uses new current key)
	claims := &Claims{
		UserID: "user-new",
		Email:  "new@example.com",
		Role:   "buyer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	tokenStr, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed: %v", err)
	}

	// Verify with new key
	parsed, err := mgr.VerifyToken(tokenStr)
	if err != nil {
		t.Fatalf("VerifyToken failed for new token: %v", err)
	}

	if parsed.UserID != "user-new" {
		t.Errorf("expected UserID=user-new, got %s", parsed.UserID)
	}
}

func TestKeyRotationManager_IsWithinGracePeriod(t *testing.T) {
	now := time.Now()
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	mgr.nowFn = func() time.Time { return now }
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// No previous key yet — not in grace period
	if mgr.IsWithinGracePeriod() {
		t.Error("should not be within grace period when no previous key exists")
	}

	// Rotate
	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// Right after rotation, within grace period
	if !mgr.IsWithinGracePeriod() {
		t.Error("should be within grace period immediately after rotation")
	}

	// Advance past grace period
	mgr.nowFn = func() time.Time { return now.Add(16 * time.Minute) }
	if mgr.IsWithinGracePeriod() {
		t.Error("should not be within grace period after 16 minutes")
	}
}

func TestKeyRotationManager_MultipleRotations(t *testing.T) {
	now := time.Now()
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	mgr.nowFn = func() time.Time { return now }
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Sign with key1
	claims := &Claims{
		UserID: "user-multi",
		Email:  "multi@example.com",
		Role:   "buyer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	tokenKey1, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed: %v", err)
	}

	// Rotate to key2
	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("First Rotate failed: %v", err)
	}

	// Sign with key2
	tokenKey2, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed after first rotation: %v", err)
	}

	// Both tokens should verify (key1 via grace, key2 via current)
	if _, err := mgr.VerifyToken(tokenKey1); err != nil {
		t.Fatalf("tokenKey1 should verify during grace period: %v", err)
	}
	if _, err := mgr.VerifyToken(tokenKey2); err != nil {
		t.Fatalf("tokenKey2 should verify with current key: %v", err)
	}

	// Rotate again to key3 — key1 is now gone, only key2 (previous) and key3 (current) remain
	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("Second Rotate failed: %v", err)
	}

	// tokenKey2 should still verify via grace period
	if _, err := mgr.VerifyToken(tokenKey2); err != nil {
		t.Fatalf("tokenKey2 should verify as previous key during grace: %v", err)
	}

	// tokenKey1 should NOT verify — it's two rotations old
	if _, err := mgr.VerifyToken(tokenKey1); err == nil {
		t.Fatal("tokenKey1 should not verify after two rotations")
	}
}

func TestKeyRotationManager_ExpiredTokenRejected(t *testing.T) {
	now := time.Now()
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	mgr.nowFn = func() time.Time { return now }
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Sign a token that's already expired
	claims := &Claims{
		UserID: "user-exp",
		Email:  "exp@example.com",
		Role:   "buyer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-20 * time.Minute)),
		},
	}

	tokenStr, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed: %v", err)
	}

	// Should reject expired token
	_, err = mgr.VerifyToken(tokenStr)
	if err == nil {
		t.Fatal("VerifyToken should reject expired token")
	}
}

func TestKeyRotationManager_UninitializedReturnsError(t *testing.T) {
	mgr := newTestManager(24*time.Hour, 15*time.Minute)

	claims := &Claims{
		UserID: "test",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}

	_, err := mgr.SignToken(claims)
	if err == nil {
		t.Fatal("SignToken should fail when not initialized")
	}

	_, err = mgr.VerifyToken("some.token.here")
	if err == nil {
		t.Fatal("VerifyToken should fail when not initialized")
	}
}

func TestKeyRotationManager_DefaultConfig(t *testing.T) {
	cfg := DefaultKeyRotationConfig()

	if cfg.RotationInterval != 24*time.Hour {
		t.Errorf("expected RotationInterval=24h, got %v", cfg.RotationInterval)
	}
	if cfg.GracePeriod != 15*time.Minute {
		t.Errorf("expected GracePeriod=15m, got %v", cfg.GracePeriod)
	}
	if cfg.KeySize != 32 {
		t.Errorf("expected KeySize=32, got %d", cfg.KeySize)
	}
}

func TestKeyRotationManager_GracePeriodBoundary(t *testing.T) {
	now := time.Now()
	mgr := newTestManager(24*time.Hour, 15*time.Minute)
	mgr.nowFn = func() time.Time { return now }
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Sign a token with the current key
	claims := &Claims{
		UserID: "boundary-user",
		Email:  "boundary@test.com",
		Role:   "buyer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	tokenStr, err := mgr.SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken failed: %v", err)
	}

	// Rotate
	if err := mgr.Rotate(ctx); err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// At exactly 15 minutes (grace period boundary) — should still be accepted
	mgr.nowFn = func() time.Time { return now.Add(15 * time.Minute) }
	if _, err := mgr.VerifyToken(tokenStr); err != nil {
		t.Fatalf("token should be accepted at exactly the grace period boundary: %v", err)
	}

	// At 15 minutes + 1 second — should be rejected
	mgr.nowFn = func() time.Time { return now.Add(15*time.Minute + 1*time.Second) }
	if _, err := mgr.VerifyToken(tokenStr); err == nil {
		t.Fatal("token should be rejected just past the grace period boundary")
	}
}
