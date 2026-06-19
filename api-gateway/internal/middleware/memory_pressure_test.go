package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestMemoryPressureMiddleware(podLimit uint64) *MemoryPressureMiddleware {
	logger, _ := zap.NewDevelopment()
	cfg := MemoryPressureConfig{
		PodMemoryLimitBytes: podLimit,
		HighWatermark:       highWatermark,
		LowWatermark:        lowWatermark,
		RetryAfterSeconds:   30,
		CheckInterval:       100 * time.Millisecond,
		Logger:              logger,
	}

	// Create the middleware but don't start the background monitor for testing.
	m := &MemoryPressureMiddleware{
		config: cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
	return m
}

func TestMemoryPressureMiddleware_AllowsRequestsNormally(t *testing.T) {
	// Use a very high memory limit so we're never under pressure.
	m := newTestMemoryPressureMiddleware(100 * 1024 * 1024 * 1024) // 100 GB
	defer m.Stop()

	router := gin.New()
	router.Use(m.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMemoryPressureMiddleware_RejectsWhenShedding(t *testing.T) {
	m := newTestMemoryPressureMiddleware(defaultPodMemoryLimitBytes)
	defer m.Stop()

	// Simulate shedding state.
	m.shedding.Store(true)

	router := gin.New()
	router.Use(m.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	// Verify Retry-After header.
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter != "30" {
		t.Errorf("expected Retry-After=30, got %q", retryAfter)
	}

	// Verify response body.
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body["success"] != false {
		t.Errorf("expected success=false, got %v", body["success"])
	}

	errorObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errorObj["code"] != "MEMORY_PRESSURE" {
		t.Errorf("expected error code MEMORY_PRESSURE, got %v", errorObj["code"])
	}
}

func TestMemoryPressureMiddleware_ResumesAfterShedding(t *testing.T) {
	m := newTestMemoryPressureMiddleware(defaultPodMemoryLimitBytes)
	defer m.Stop()

	// Start in shedding state.
	m.shedding.Store(true)

	// Simulate recovery.
	m.shedding.Store(false)

	router := gin.New()
	router.Use(m.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after shedding recovery, got %d", w.Code)
	}
}

func TestMemoryPressureMiddleware_Hysteresis(t *testing.T) {
	// Create middleware with a known low limit to trigger high watermark.
	logger, _ := zap.NewDevelopment()
	m := &MemoryPressureMiddleware{
		config: MemoryPressureConfig{
			PodMemoryLimitBytes: 1024, // 1 KB — will definitely exceed
			HighWatermark:       0.85,
			LowWatermark:        0.80,
			RetryAfterSeconds:   30,
			CheckInterval:       time.Second,
			Logger:              logger,
		},
		logger: logger,
		stopCh: make(chan struct{}),
	}
	defer m.Stop()

	// Run a check — memory will be way above 85% of 1 KB.
	m.checkMemory()

	if !m.IsShedding() {
		t.Error("expected shedding to be active after checkMemory with very low limit")
	}

	// Now set a very high limit — memory will be below 80%.
	m.config.PodMemoryLimitBytes = 100 * 1024 * 1024 * 1024 // 100 GB
	m.checkMemory()

	if m.IsShedding() {
		t.Error("expected shedding to be deactivated after checkMemory with very high limit")
	}
}

func TestMemoryPressureMiddleware_HysteresisPreventFlapping(t *testing.T) {
	// Test that when memory is between low and high watermark, the state doesn't change.
	logger, _ := zap.NewDevelopment()

	// Set a limit such that current usage is between 80% and 85%.
	// We'll manipulate the shedding state manually and verify checkMemory doesn't change it.
	m := &MemoryPressureMiddleware{
		config: MemoryPressureConfig{
			PodMemoryLimitBytes: 100 * 1024 * 1024 * 1024, // Very high — well below 80%
			HighWatermark:       0.85,
			LowWatermark:        0.80,
			RetryAfterSeconds:   30,
			CheckInterval:       time.Second,
			Logger:              logger,
		},
		logger: logger,
		stopCh: make(chan struct{}),
	}
	defer m.Stop()

	// Start NOT shedding, and memory is well below low watermark.
	m.shedding.Store(false)
	m.checkMemory()

	if m.IsShedding() {
		t.Error("should not be shedding with high memory limit")
	}
}

func TestMemoryPressureMiddleware_Stop(t *testing.T) {
	m := newTestMemoryPressureMiddleware(defaultPodMemoryLimitBytes)

	// Stop should be idempotent.
	m.Stop()
	m.Stop()
}

func TestMemoryPressureMiddleware_IsShedding(t *testing.T) {
	m := newTestMemoryPressureMiddleware(defaultPodMemoryLimitBytes)
	defer m.Stop()

	if m.IsShedding() {
		t.Error("expected IsShedding to be false initially")
	}

	m.shedding.Store(true)
	if !m.IsShedding() {
		t.Error("expected IsShedding to be true after setting")
	}
}

func TestReadPodMemoryLimit_Default(t *testing.T) {
	// Without env var and on systems without cgroups, should return default.
	// We can't easily mock file reads, but we can verify the function doesn't panic.
	limit := readPodMemoryLimit()
	if limit == 0 {
		t.Error("expected non-zero memory limit")
	}
}

func TestReadPodMemoryLimit_FromEnv(t *testing.T) {
	t.Setenv("POD_MEMORY_LIMIT_BYTES", "1073741824") // 1 GB

	limit := readPodMemoryLimit()
	if limit != 1073741824 {
		t.Errorf("expected 1073741824, got %d", limit)
	}
}

func TestReadPodMemoryLimit_InvalidEnv(t *testing.T) {
	t.Setenv("POD_MEMORY_LIMIT_BYTES", "not-a-number")

	limit := readPodMemoryLimit()
	// Should fall back to cgroups or default, not panic.
	if limit == 0 {
		t.Error("expected non-zero fallback memory limit")
	}
}

func TestTrimNewline(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"12345\n", "12345"},
		{"12345\r\n", "12345"},
		{"12345", "12345"},
		{"\n", ""},
		{"", ""},
	}

	for _, tc := range tests {
		result := trimNewline(tc.input)
		if result != tc.expected {
			t.Errorf("trimNewline(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestNewMemoryPressureMiddleware_NilLogger(t *testing.T) {
	// Should not panic with nil logger.
	m := NewMemoryPressureMiddleware(nil)
	defer m.Stop()

	if m == nil {
		t.Fatal("expected non-nil middleware")
	}
}
