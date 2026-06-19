package serviceauth

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRotationScheduler_ForceRotate(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	// Issue token with original key
	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	scheduler := NewRotationScheduler(km, &RotationConfig{
		RotationInterval: DefaultRotationInterval,
		OverlapPeriod:    DefaultOverlapPeriod,
		Logger:           testLogger(),
	})

	// Force rotation
	err = scheduler.ForceRotate()
	if err != nil {
		t.Fatalf("force rotate failed: %v", err)
	}

	// Old token should still be valid (within overlap period)
	claims, err := validator.ValidateToken(token)
	if err != nil {
		t.Fatalf("old token should be valid during overlap: %v", err)
	}
	if claims.ServiceID != "bid-service" {
		t.Fatalf("expected bid-service, got %s", claims.ServiceID)
	}

	// New token should also be valid
	newToken, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue new token: %v", err)
	}

	claims, err = validator.ValidateToken(newToken)
	if err != nil {
		t.Fatalf("new token should be valid: %v", err)
	}
	if claims.ServiceID != "bid-service" {
		t.Fatalf("expected bid-service, got %s", claims.ServiceID)
	}
}

func TestRotationScheduler_AutoRotation(t *testing.T) {
	cfg := testConfig("bid-service")
	cfg.OverlapPeriod = 1 * time.Second // short overlap for test
	km := NewKeyManager(cfg)

	rotationCount := 0
	scheduler := NewRotationScheduler(km, &RotationConfig{
		RotationInterval: 50 * time.Millisecond, // rotate quickly for test
		OverlapPeriod:    1 * time.Second,
		CheckInterval:    10 * time.Millisecond, // check frequently for test
		KeyGenerator: func() (string, error) {
			rotationCount++
			return "rotated-key-" + time.Now().String(), nil
		},
		Logger: testLogger(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	scheduler.Start(ctx)
	defer scheduler.Stop()

	// Wait for at least one rotation
	time.Sleep(150 * time.Millisecond)

	if rotationCount == 0 {
		t.Fatal("expected at least one auto-rotation")
	}
}

func TestRotationScheduler_NextRotation(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)

	scheduler := NewRotationScheduler(km, &RotationConfig{
		RotationInterval: 7 * 24 * time.Hour,
		OverlapPeriod:    24 * time.Hour,
		Logger:           testLogger(),
	})

	next := scheduler.NextRotation()
	if next.Before(time.Now()) {
		t.Fatal("next rotation should be in the future")
	}

	expected := time.Now().Add(7 * 24 * time.Hour)
	if next.After(expected.Add(1 * time.Second)) {
		t.Fatalf("next rotation too far in future: %v", next)
	}
}

func TestDefaultKeyGenerator(t *testing.T) {
	key1, err := defaultKeyGenerator()
	if err != nil {
		t.Fatalf("key generation failed: %v", err)
	}
	if len(key1) != 64 { // 32 bytes as hex = 64 chars
		t.Fatalf("expected 64 char hex key, got %d chars", len(key1))
	}

	key2, err := defaultKeyGenerator()
	if err != nil {
		t.Fatalf("key generation failed: %v", err)
	}

	if key1 == key2 {
		t.Fatal("generated keys should be unique")
	}
}

func TestRotationScheduler_StopBeforeStart(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)

	scheduler := NewRotationScheduler(km, &RotationConfig{
		Logger: zap.NewNop(),
	})

	// Should not panic
	scheduler.Stop()
}
