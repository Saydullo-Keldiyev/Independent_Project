package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestExtractEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"login path", "/api/v1/auth/login", "/login"},
		{"register path", "/api/v1/auth/register", "/register"},
		{"bids path", "/api/v1/bids", "/bids"},
		{"bids/me path", "/api/v1/bids/me", ""},
		{"auctions path", "/api/v1/auctions", ""},
		{"plain login", "/login", "/login"},
		{"plain register", "/register", "/register"},
		{"plain bids", "/bids", "/bids"},
		{"other path", "/api/v1/users/me", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractEndpoint(tt.path)
			if got != tt.expected {
				t.Errorf("extractEndpoint(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestSlidingWindowRateLimiter_AnonymousRateLimiting(t *testing.T) {
	// Create rate limiter with nil Redis (uses in-memory fallback)
	rl := NewSlidingWindowRateLimiter(nil, nil)

	if !rl.UsingFallback() {
		t.Fatal("expected to use fallback when Redis client is nil")
	}

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Anonymous limit is 30/min. Send 30 requests — all should pass.
	for i := 0; i < 30; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Request 31 should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 31: expected 429, got %d", w.Code)
	}

	// Verify Retry-After header is present
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header to be set")
	}
}

func TestSlidingWindowRateLimiter_AuthenticatedRateLimiting(t *testing.T) {
	rl := NewSlidingWindowRateLimiter(nil, nil)

	router := gin.New()
	// Simulate auth middleware setting user_id
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-123")
		c.Next()
	})
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Authenticated limit is 200/min. Send 200 requests — all should pass.
	for i := 0; i < 200; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Request 201 should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 201: expected 429, got %d", w.Code)
	}
}

func TestSlidingWindowRateLimiter_EndpointSpecificLimit(t *testing.T) {
	rl := NewSlidingWindowRateLimiter(nil, nil)

	router := gin.New()
	router.Use(rl.Middleware())
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Login endpoint limit is 5/min for anonymous
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "172.16.0.1:12345"
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Request 6 should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.RemoteAddr = "172.16.0.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 6 to /login: expected 429, got %d", w.Code)
	}
}

func TestSlidingWindowRateLimiter_IPBlocking(t *testing.T) {
	rl := NewSlidingWindowRateLimiter(nil, nil)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	ip := "10.99.99.99"

	// Exhaust the rate limit multiple times to trigger IP blocking.
	// Anonymous limit = 30. We need 10 violations.
	// Each batch of 31 requests causes 1 violation (the 31st request).
	for violation := 0; violation < 10; violation++ {
		for i := 0; i < 31; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = ip + ":12345"
			router.ServeHTTP(w, req)
		}
	}

	// Now the IP should be blocked — next request should get 403
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = ip + ":12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked IP, got %d", w.Code)
	}

	// Verify the response contains block reason
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "IP_BLOCKED" {
		t.Fatalf("expected error code IP_BLOCKED, got %v", errObj["code"])
	}

	// Verify Retry-After header
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header for blocked IP")
	}
}

func TestSlidingWindowRateLimiter_ResponseHeaders(t *testing.T) {
	rl := NewSlidingWindowRateLimiter(nil, nil)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify rate limit headers
	limitHeader := w.Header().Get("X-RateLimit-Limit")
	if limitHeader == "" {
		t.Fatal("expected X-RateLimit-Limit header")
	}
	if limitHeader != "30" {
		t.Fatalf("expected anonymous limit of 30, got %s", limitHeader)
	}

	remainingHeader := w.Header().Get("X-RateLimit-Remaining")
	if remainingHeader == "" {
		t.Fatal("expected X-RateLimit-Remaining header")
	}
	if remainingHeader != "29" {
		t.Fatalf("expected remaining 29, got %s", remainingHeader)
	}
}
