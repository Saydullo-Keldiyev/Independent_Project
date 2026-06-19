package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/auction-system/api-gateway/internal/proxy"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCorrelationID_GeneratesUUIDWhenHeaderAbsent(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(CorrelationID())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, c.Request)

	// Should have X-Correlation-ID in response header.
	responseCorrelationID := w.Header().Get(proxy.HeaderCorrelationID)
	if responseCorrelationID == "" {
		t.Fatal("expected X-Correlation-ID response header to be set")
	}

	// Should be a valid UUID v4.
	parsed, err := uuid.Parse(responseCorrelationID)
	if err != nil {
		t.Fatalf("expected valid UUID, got %q: %v", responseCorrelationID, err)
	}
	if parsed.Version() != 4 {
		t.Errorf("expected UUID v4, got version %d", parsed.Version())
	}
}

func TestCorrelationID_PreservesExistingHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	existingID := "my-custom-correlation-id-12345"

	r.Use(CorrelationID())
	r.GET("/test", func(c *gin.Context) {
		// Verify the correlation ID in context matches what was sent.
		ctxID := c.GetString(CorrelationIDKey)
		if ctxID != existingID {
			t.Errorf("expected context correlation_id=%q, got %q", existingID, ctxID)
		}
		c.Status(http.StatusOK)
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Request.Header.Set(proxy.HeaderCorrelationID, existingID)
	r.ServeHTTP(w, c.Request)

	// Response header should echo back the same value without modification.
	responseCorrelationID := w.Header().Get(proxy.HeaderCorrelationID)
	if responseCorrelationID != existingID {
		t.Errorf("expected response header %q, got %q", existingID, responseCorrelationID)
	}
}

func TestCorrelationID_StoresInGinContext(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	var capturedID string
	r.Use(CorrelationID())
	r.GET("/test", func(c *gin.Context) {
		capturedID = c.GetString(CorrelationIDKey)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if capturedID == "" {
		t.Fatal("expected correlation_id to be stored in Gin context")
	}

	// Verify it's a valid UUID when generated.
	_, err := uuid.Parse(capturedID)
	if err != nil {
		t.Fatalf("expected valid UUID in context, got %q: %v", capturedID, err)
	}
}

func TestCorrelationID_PreservesNonUUIDValues(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	// Per requirement 5.4: preserve and forward existing value without modification.
	// Even non-UUID values should be preserved.
	nonUUIDValue := "trace-abc-123-request-xyz"

	var capturedID string
	r.Use(CorrelationID())
	r.GET("/test", func(c *gin.Context) {
		capturedID = c.GetString(CorrelationIDKey)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(proxy.HeaderCorrelationID, nonUUIDValue)
	r.ServeHTTP(w, req)

	if capturedID != nonUUIDValue {
		t.Errorf("expected non-UUID value to be preserved: want %q, got %q", nonUUIDValue, capturedID)
	}

	responseCorrelationID := w.Header().Get(proxy.HeaderCorrelationID)
	if responseCorrelationID != nonUUIDValue {
		t.Errorf("expected response header to preserve non-UUID value: want %q, got %q", nonUUIDValue, responseCorrelationID)
	}
}

func TestCorrelationID_UniquePerRequest(t *testing.T) {
	w1 := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w1)

	var ids []string
	r.Use(CorrelationID())
	r.GET("/test", func(c *gin.Context) {
		ids = append(ids, c.GetString(CorrelationIDKey))
		c.Status(http.StatusOK)
	})

	// Make multiple requests without correlation ID header.
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
	}

	// All generated IDs should be unique.
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate correlation ID generated: %q", id)
		}
		seen[id] = true
	}
}

func TestNewCorrelationMiddleware(t *testing.T) {
	cm := NewCorrelationMiddleware()
	if cm == nil {
		t.Fatal("expected non-nil CorrelationMiddleware")
	}
	if cm.headerName != proxy.HeaderCorrelationID {
		t.Errorf("expected headerName=%q, got %q", proxy.HeaderCorrelationID, cm.headerName)
	}
}
