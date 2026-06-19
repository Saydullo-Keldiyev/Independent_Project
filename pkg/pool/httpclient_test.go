package pool

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultHTTPClientConfig(t *testing.T) {
	cfg := DefaultHTTPClientConfig()

	if cfg.Timeout != 30*time.Second {
		t.Fatalf("expected 30s timeout, got %v", cfg.Timeout)
	}
	if cfg.MaxIdleConns != 100 {
		t.Fatalf("expected 100 max idle conns, got %d", cfg.MaxIdleConns)
	}
	if cfg.MaxIdleConnsPerHost != 10 {
		t.Fatalf("expected 10 max idle conns per host, got %d", cfg.MaxIdleConnsPerHost)
	}
}

func TestNewHTTPClient_Timeout(t *testing.T) {
	cfg := HTTPClientConfig{
		Timeout:             100 * time.Millisecond,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		DialTimeout:         5 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	client := NewHTTPClient(cfg)

	// Create a server that sleeps longer than the timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DoWithTimeout(client, req)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestNewHTTPClient_Success(t *testing.T) {
	cfg := DefaultHTTPClientConfig()
	client := NewHTTPClient(cfg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := DoWithTimeout(client, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}
