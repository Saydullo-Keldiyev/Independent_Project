package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key-min-32-characters"

// TestJWT_GenerateAndValidate verifies token round-trip
func TestJWT_GenerateAndValidate(t *testing.T) {
	token, err := generateToken("user-123", "bidder", testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := validateToken(token, testSecret)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("user_id = %s, want user-123", claims.UserID)
	}
	if claims.Role != "bidder" {
		t.Errorf("role = %s, want bidder", claims.Role)
	}
}

// TestJWT_ExpiredToken verifies expired tokens are rejected
func TestJWT_ExpiredToken(t *testing.T) {
	token, _ := generateToken("user-123", "bidder", testSecret, -1*time.Minute)

	_, err := validateToken(token, testSecret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

// TestJWT_InvalidSecret verifies wrong secret is rejected
func TestJWT_InvalidSecret(t *testing.T) {
	token, _ := generateToken("user-123", "bidder", testSecret, 15*time.Minute)

	_, err := validateToken(token, "wrong-secret-key-that-is-different")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

// TestJWT_TamperedToken verifies tampered tokens are rejected
func TestJWT_TamperedToken(t *testing.T) {
	token, _ := generateToken("user-123", "bidder", testSecret, 15*time.Minute)

	// Tamper with the token
	tampered := token[:len(token)-5] + "XXXXX"

	_, err := validateToken(tampered, testSecret)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

// TestJWT_RoleValidation verifies RBAC roles
func TestJWT_RoleValidation(t *testing.T) {
	roles := []string{"admin", "seller", "bidder"}
	for _, role := range roles {
		token, _ := generateToken("user-1", role, testSecret, 15*time.Minute)
		claims, err := validateToken(token, testSecret)
		if err != nil {
			t.Fatalf("role %s: %v", role, err)
		}
		if claims.Role != role {
			t.Errorf("got role %s, want %s", claims.Role, role)
		}
	}
}

// ── JWT helpers (mirror production logic) ─────────────────────────────────────

type testClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func generateToken(userID, role, secret string, ttl time.Duration) (string, error) {
	claims := testClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func validateToken(tokenStr, secret string) (*testClaims, error) {
	claims := &testClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}
