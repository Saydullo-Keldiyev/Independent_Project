package serviceauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

func testConfig(serviceID string, keys ...string) *Config {
	if len(keys) == 0 {
		keys = []string{"test-secret-key-32-bytes-long!!"}
	}
	return &Config{
		ServiceID:        serviceID,
		SigningKeys:      keys,
		TokenTTL:         1 * time.Hour,
		RotationInterval: DefaultRotationInterval,
		OverlapPeriod:    DefaultOverlapPeriod,
		Logger:           testLogger(),
	}
}

func TestTokenIssuer_IssueToken(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)

	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}
}

func TestTokenIssuer_TokenTTLCapped(t *testing.T) {
	cfg := testConfig("bid-service")
	cfg.TokenTTL = 48 * time.Hour // exceeds 24h max

	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)

	// TTL should be capped at 24h
	if issuer.tokenTTL != DefaultTokenTTL {
		t.Fatalf("expected TTL capped at %v, got %v", DefaultTokenTTL, issuer.tokenTTL)
	}

	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	// Parse and check expiration
	claims := &ServiceClaims{}
	parsed, _ := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.SigningKeys[0]), nil
	})
	if !parsed.Valid {
		t.Fatal("token should be valid")
	}

	expiry := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	if expiry > DefaultTokenTTL+time.Second {
		t.Fatalf("token TTL %v exceeds max %v", expiry, DefaultTokenTTL)
	}
}

func TestTokenValidator_ValidToken(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	claims, err := validator.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if claims.ServiceID != "bid-service" {
		t.Fatalf("expected service_id=bid-service, got %s", claims.ServiceID)
	}
}

func TestTokenValidator_ExpiredToken(t *testing.T) {
	cfg := testConfig("bid-service")
	cfg.TokenTTL = 1 * time.Millisecond // very short TTL
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, err = validator.ValidateToken(token)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestTokenValidator_InvalidSignature(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)

	// Create validator with different key
	cfg2 := testConfig("api-gateway", "different-secret-key-32-bytes!!")
	km2 := NewKeyManager(cfg2)
	validator := NewTokenValidator(cfg2, km2)

	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	_, err = validator.ValidateToken(token)
	if err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestTokenValidator_FailClosed(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	// Mark auth subsystem as unavailable
	validator.SetAvailable(false)

	_, err = validator.ValidateToken(token)
	if err != ErrAuthUnavailable {
		t.Fatalf("expected ErrAuthUnavailable, got %v", err)
	}

	// Restore availability
	validator.SetAvailable(true)

	claims, err := validator.ValidateToken(token)
	if err != nil {
		t.Fatalf("should succeed after availability restored: %v", err)
	}
	if claims.ServiceID != "bid-service" {
		t.Fatalf("expected bid-service, got %s", claims.ServiceID)
	}
}

func TestTokenValidator_UnauthorizedService(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)

	// Validator only allows "api-gateway"
	validatorCfg := testConfig("api-gateway")
	validatorCfg.AllowedServices = []string{"api-gateway"}
	validator := NewTokenValidator(validatorCfg, km)

	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	_, err = validator.ValidateToken(token)
	if err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestKeyManager_RotateKey(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	// Issue token with current key
	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	// Rotate key
	km.RotateKey("new-secret-key-after-rotation!!")

	// Token signed with old key should still be valid (within overlap period)
	claims, err := validator.ValidateToken(token)
	if err != nil {
		t.Fatalf("token signed with previous key should be valid during overlap: %v", err)
	}
	if claims.ServiceID != "bid-service" {
		t.Fatalf("expected bid-service, got %s", claims.ServiceID)
	}

	// New token should use new key
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

func TestKeyManager_ValidationKeysOverlapExpired(t *testing.T) {
	cfg := testConfig("bid-service")
	cfg.OverlapPeriod = 1 * time.Millisecond // very short overlap
	km := NewKeyManager(cfg)

	// Issue token with current key
	issuer := NewTokenIssuer(cfg, km)
	token, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	// Rotate key
	km.RotateKey("new-secret-key-after-rotation!!")

	// Wait for overlap to expire
	time.Sleep(5 * time.Millisecond)

	// Token signed with old key should now be rejected
	validator := NewTokenValidator(cfg, km)
	_, err = validator.ValidateToken(token)
	if err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid after overlap expiry, got %v", err)
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	token, _ := issuer.IssueToken()

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(Middleware(&MiddlewareConfig{
		Validator: validator,
		Logger:    testLogger(),
	}))
	r.GET("/test", func(c *gin.Context) {
		svcID := c.GetString("service_id")
		c.JSON(200, gin.H{"service_id": svcID})
	})

	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("X-Service-Token", token)
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMiddleware_MissingToken(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	validator := NewTokenValidator(cfg, km)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(Middleware(&MiddlewareConfig{
		Validator: validator,
		Logger:    testLogger(),
	}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	validator := NewTokenValidator(cfg, km)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(Middleware(&MiddlewareConfig{
		Validator: validator,
		Logger:    testLogger(),
	}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Service-Token", "invalid.jwt.token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestMiddleware_AuthUnavailable_FailClosed(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	// Mark auth subsystem unavailable
	validator.SetAvailable(false)

	token, _ := issuer.IssueToken()

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(Middleware(&MiddlewareConfig{
		Validator: validator,
		Logger:    testLogger(),
	}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Service-Token", token)
	r.ServeHTTP(w, req)

	// Should reject even valid tokens when auth is unavailable
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (fail-closed), got %d", w.Code)
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	validator := NewTokenValidator(cfg, km)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(Middleware(&MiddlewareConfig{
		Validator: validator,
		Logger:    testLogger(),
		SkipPaths: []string{"/health", "/ready"},
	}))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for skip path, got %d", w.Code)
	}
}

func TestMiddleware_BearerTokenFormat(t *testing.T) {
	cfg := testConfig("bid-service")
	km := NewKeyManager(cfg)
	issuer := NewTokenIssuer(cfg, km)
	validator := NewTokenValidator(cfg, km)

	token, _ := issuer.IssueToken()

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(Middleware(&MiddlewareConfig{
		Validator: validator,
		Logger:    testLogger(),
	}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"service_id": c.GetString("service_id")})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with Bearer format, got %d: %s", w.Code, w.Body.String())
	}
}
