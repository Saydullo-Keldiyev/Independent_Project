package pool

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HTTPClientConfig holds settings for outbound inter-service HTTP calls.
type HTTPClientConfig struct {
	// Timeout is the total request timeout (Requirement 16.2: 30s).
	Timeout time.Duration

	// MaxIdleConns controls the maximum number of idle connections across all hosts.
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum number of idle connections per host.
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle connection will remain idle.
	IdleConnTimeout time.Duration

	// DialTimeout is the maximum time to establish a TCP connection.
	DialTimeout time.Duration

	// TLSHandshakeTimeout is the maximum time for TLS handshake.
	TLSHandshakeTimeout time.Duration
}

// DefaultHTTPClientConfig returns production-ready HTTP client defaults.
// 30s timeout for all outbound HTTP calls between services (Requirement 16.2).
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:             30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialTimeout:         5 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
}

// NewHTTPClient creates an http.Client configured for inter-service communication.
// All outbound HTTP calls have a 30s timeout per Requirement 16.2.
// If a timeout occurs, the request is cancelled, resources are released,
// and an error is returned to the caller.
func NewHTTPClient(cfg HTTPClientConfig) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		TLSHandshakeTimeout: cfg.TLSHandshakeTimeout,
		ForceAttemptHTTP2:   true,
	}

	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}
}

// DoWithTimeout executes an HTTP request with the configured timeout.
// If the context is cancelled or the timeout is reached, the request is cancelled,
// resources are released, and a descriptive error is returned (Requirement 16.2).
func DoWithTimeout(client *http.Client, req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if _, ok := ctx.Deadline(); !ok {
		// If no deadline is set on the context, use 30s default
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("outbound HTTP request timed out after %v: %w", client.Timeout, err)
		}
		return nil, fmt.Errorf("outbound HTTP request failed: %w", err)
	}

	return resp, nil
}
