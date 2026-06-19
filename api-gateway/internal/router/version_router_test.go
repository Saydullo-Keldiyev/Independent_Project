package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() (*gin.Engine, *VersionRouter) {
	engine := gin.New()
	vr := NewVersionRouter(engine, 3)
	return engine, vr
}

func TestNewVersionRouter_ClampsMaxVersions(t *testing.T) {
	engine := gin.New()

	// Test lower bound
	vr := NewVersionRouter(engine, 1)
	if vr.maxVersions != 2 {
		t.Errorf("expected maxVersions=2 for input 1, got %d", vr.maxVersions)
	}

	// Test upper bound
	vr = NewVersionRouter(engine, 5)
	if vr.maxVersions != 3 {
		t.Errorf("expected maxVersions=3 for input 5, got %d", vr.maxVersions)
	}

	// Test valid
	vr = NewVersionRouter(engine, 2)
	if vr.maxVersions != 2 {
		t.Errorf("expected maxVersions=2, got %d", vr.maxVersions)
	}
}

func TestRegisterVersion_Success(t *testing.T) {
	_, vr := setupTestRouter()

	group, err := vr.RegisterVersion("v1", VersionConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group == nil {
		t.Fatal("expected non-nil router group")
	}
}

func TestRegisterVersion_MaxVersionsExceeded(t *testing.T) {
	engine := gin.New()
	vr := NewVersionRouter(engine, 2)

	_, err := vr.RegisterVersion("v1", VersionConfig{})
	if err != nil {
		t.Fatalf("unexpected error registering v1: %v", err)
	}

	_, err = vr.RegisterVersion("v2", VersionConfig{})
	if err != nil {
		t.Fatalf("unexpected error registering v2: %v", err)
	}

	_, err = vr.RegisterVersion("v3", VersionConfig{})
	if err == nil {
		t.Fatal("expected error when exceeding max versions, got nil")
	}
}

func TestRegisterVersion_SunsetVersionDoesNotCountTowardsMax(t *testing.T) {
	engine := gin.New()
	vr := NewVersionRouter(engine, 2)

	// Register v1 as deprecated with past sunset date
	pastSunset := time.Now().Add(-24 * time.Hour)
	_, err := vr.RegisterVersion("v1", VersionConfig{
		Deprecated: true,
		SunsetDate: pastSunset,
	})
	if err != nil {
		t.Fatalf("unexpected error registering v1: %v", err)
	}

	_, err = vr.RegisterVersion("v2", VersionConfig{})
	if err != nil {
		t.Fatalf("unexpected error registering v2: %v", err)
	}

	_, err = vr.RegisterVersion("v3", VersionConfig{})
	if err != nil {
		t.Fatalf("unexpected error registering v3: %v", err)
	}
}

func TestVersionRouter_RoutesToCorrectVersion(t *testing.T) {
	engine, vr := setupTestRouter()

	v1, _ := vr.RegisterVersion("v1", VersionConfig{})
	v1.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v1", "resource": "users"})
	})

	v2, _ := vr.RegisterVersion("v2", VersionConfig{})
	v2.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v2", "resource": "users"})
	})

	// Test v1 routing
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/users, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["version"] != "v1" {
		t.Errorf("expected version=v1, got %v", body["version"])
	}

	// Test v2 routing
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v2/users", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v2/users, got %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &body)
	if body["version"] != "v2" {
		t.Errorf("expected version=v2, got %v", body["version"])
	}
}

func TestVersionRouter_UnrecognizedVersion_Returns404(t *testing.T) {
	engine, vr := setupTestRouter()

	v1, _ := vr.RegisterVersion("v1", VersionConfig{})
	v1.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v1"})
	})

	// Use the VersionNotFoundHandler as NoRoute handler
	engine.NoRoute(vr.VersionNotFoundHandler())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v99/users", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unrecognized version, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "VERSION_NOT_FOUND" {
		t.Errorf("expected code=VERSION_NOT_FOUND, got %v", errObj["code"])
	}

	supportedVersions, ok := errObj["supported_versions"].([]interface{})
	if !ok {
		t.Fatal("expected supported_versions array in error response")
	}
	if len(supportedVersions) == 0 {
		t.Error("expected at least one supported version in response")
	}
}

func TestVersionRouter_DeprecatedVersion_AddsHeaders(t *testing.T) {
	engine, vr := setupTestRouter()

	// Deprecate v1 with sunset 90 days from now
	sunsetDate := time.Now().Add(90 * 24 * time.Hour)
	v1, _ := vr.RegisterVersion("v1", VersionConfig{
		Deprecated: true,
		SunsetDate: sunsetDate,
	})
	v1.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v1"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for deprecated but active version, got %d", w.Code)
	}

	deprecationHeader := w.Header().Get("Deprecation")
	if deprecationHeader == "" {
		t.Error("expected Deprecation header to be present")
	}

	// Verify the header is in RFC 7231 format
	_, err := time.Parse(http.TimeFormat, deprecationHeader)
	if err != nil {
		t.Errorf("Deprecation header is not in RFC 7231 format: %v", err)
	}

	sunsetHeader := w.Header().Get("Sunset")
	if sunsetHeader == "" {
		t.Error("expected Sunset header to be present")
	}
}

func TestVersionRouter_SunsetVersion_Returns410(t *testing.T) {
	engine, vr := setupTestRouter()

	// Register v1 with past sunset date
	pastSunset := time.Now().Add(-24 * time.Hour)
	v1, _ := vr.RegisterVersion("v1", VersionConfig{
		Deprecated: true,
		SunsetDate: pastSunset,
	})
	v1.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v1"})
	})

	// Register v2 as current active version
	v2, _ := vr.RegisterVersion("v2", VersionConfig{})
	v2.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v2"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("expected 410 for sunset version, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "VERSION_GONE" {
		t.Errorf("expected code=VERSION_GONE, got %v", errObj["code"])
	}

	supportedVersions, ok := errObj["supported_versions"].([]interface{})
	if !ok {
		t.Fatal("expected supported_versions array")
	}

	// Only v2 should be listed (v1 is past sunset)
	found := false
	for _, v := range supportedVersions {
		if v == "/api/v2" {
			found = true
		}
		if v == "/api/v1" {
			t.Error("sunset version /api/v1 should not be listed as supported")
		}
	}
	if !found {
		t.Error("expected /api/v2 in supported versions list")
	}
}

func TestVersionRouter_NonAPIPath_PassesThrough(t *testing.T) {
	engine, vr := setupTestRouter()

	vr.RegisterVersion("v1", VersionConfig{})

	// Register a non-API path
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.NoRoute(vr.VersionNotFoundHandler())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", w.Code)
	}
}

func TestVersionRouter_GetVersionGroup(t *testing.T) {
	_, vr := setupTestRouter()

	vr.RegisterVersion("v1", VersionConfig{})

	group := vr.GetVersionGroup("v1")
	if group == nil {
		t.Error("expected non-nil group for registered version")
	}

	group = vr.GetVersionGroup("v99")
	if group != nil {
		t.Error("expected nil group for unregistered version")
	}
}

func TestVersionRouter_UpdateVersionConfig(t *testing.T) {
	engine, vr := setupTestRouter()

	v1, _ := vr.RegisterVersion("v1", VersionConfig{})
	v1.GET("/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v1"})
	})

	// Initially no deprecation header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	engine.ServeHTTP(w, req)

	if w.Header().Get("Deprecation") != "" {
		t.Error("expected no Deprecation header before deprecation")
	}

	// Now deprecate v1
	sunsetDate := time.Now().Add(90 * 24 * time.Hour)
	err := vr.UpdateVersionConfig("v1", VersionConfig{
		Deprecated: true,
		SunsetDate: sunsetDate,
	})
	if err != nil {
		t.Fatalf("unexpected error updating config: %v", err)
	}

	// Should now have deprecation header
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users", nil)
	engine.ServeHTTP(w, req)

	if w.Header().Get("Deprecation") == "" {
		t.Error("expected Deprecation header after deprecation")
	}
}

func TestVersionRouter_UpdateVersionConfig_NotRegistered(t *testing.T) {
	_, vr := setupTestRouter()

	err := vr.UpdateVersionConfig("v99", VersionConfig{})
	if err == nil {
		t.Error("expected error updating config for unregistered version")
	}
}

func TestVersionRouter_SupportedVersions(t *testing.T) {
	_, vr := setupTestRouter()

	vr.RegisterVersion("v1", VersionConfig{
		Deprecated: true,
		SunsetDate: time.Now().Add(-24 * time.Hour), // past sunset
	})
	vr.RegisterVersion("v2", VersionConfig{})

	supported := vr.SupportedVersions()

	// Only v2 should be in supported list
	foundV1 := false
	foundV2 := false
	for _, v := range supported {
		if v == "/api/v1" {
			foundV1 = true
		}
		if v == "/api/v2" {
			foundV2 = true
		}
	}

	if foundV1 {
		t.Error("sunset v1 should not be in supported versions")
	}
	if !foundV2 {
		t.Error("v2 should be in supported versions")
	}
}

func TestVersionRouter_DeprecatedNotSunset_NoHeaderIfNotDeprecated(t *testing.T) {
	engine, vr := setupTestRouter()

	v1, _ := vr.RegisterVersion("v1", VersionConfig{
		Deprecated: false,
	})
	v1.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"pong": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/ping", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Deprecation") != "" {
		t.Error("non-deprecated version should not have Deprecation header")
	}
	if w.Header().Get("Sunset") != "" {
		t.Error("non-deprecated version should not have Sunset header")
	}
}
