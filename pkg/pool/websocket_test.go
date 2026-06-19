package pool

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func newTestWSLimiter(maxConns int64, heartbeatTimeout time.Duration) *WebSocketLimiter {
	reg := prometheus.NewRegistry()
	logger := zap.NewNop()
	cfg := WebSocketConfig{
		MaxConnections:   maxConns,
		HeartbeatTimeout: heartbeatTimeout,
		ServiceName:      "test-service",
	}
	return NewWebSocketLimiter(cfg, logger, reg)
}

func TestWebSocketLimiter_TryAccept_WithinLimit(t *testing.T) {
	wsl := newTestWSLimiter(5, 5*time.Minute)
	defer wsl.Stop()

	for i := 0; i < 5; i++ {
		connID := fmt.Sprintf("conn-%d", i)
		if !wsl.TryAccept(connID) {
			t.Fatalf("expected connection %d to be accepted", i)
		}
	}

	if wsl.CurrentConnections() != 5 {
		t.Fatalf("expected 5 connections, got %d", wsl.CurrentConnections())
	}
}

func TestWebSocketLimiter_TryAccept_AtCapacity(t *testing.T) {
	wsl := newTestWSLimiter(3, 5*time.Minute)
	defer wsl.Stop()

	// Fill to capacity
	for i := 0; i < 3; i++ {
		wsl.TryAccept(fmt.Sprintf("conn-%d", i))
	}

	// Next attempt should be rejected
	if wsl.TryAccept("conn-overflow") {
		t.Fatal("expected connection to be rejected at capacity")
	}

	if wsl.CurrentConnections() != 3 {
		t.Fatalf("expected 3 connections, got %d", wsl.CurrentConnections())
	}
}

func TestWebSocketLimiter_Release(t *testing.T) {
	wsl := newTestWSLimiter(3, 5*time.Minute)
	defer wsl.Stop()

	wsl.TryAccept("conn-1")
	wsl.TryAccept("conn-2")
	wsl.TryAccept("conn-3")

	// At capacity
	if wsl.TryAccept("conn-4") {
		t.Fatal("expected rejection at capacity")
	}

	// Release one
	wsl.Release("conn-2")

	if wsl.CurrentConnections() != 2 {
		t.Fatalf("expected 2 connections after release, got %d", wsl.CurrentConnections())
	}

	// Should now accept
	if !wsl.TryAccept("conn-4") {
		t.Fatal("expected connection to be accepted after release")
	}
}

func TestWebSocketLimiter_IsAtCapacity(t *testing.T) {
	wsl := newTestWSLimiter(2, 5*time.Minute)
	defer wsl.Stop()

	if wsl.IsAtCapacity() {
		t.Fatal("should not be at capacity initially")
	}

	wsl.TryAccept("conn-1")
	wsl.TryAccept("conn-2")

	if !wsl.IsAtCapacity() {
		t.Fatal("should be at capacity with 2/2 connections")
	}
}

func TestWebSocketLimiter_HeartbeatTimeout(t *testing.T) {
	// Use very short timeout for testing
	wsl := newTestWSLimiter(10, 50*time.Millisecond)
	defer wsl.Stop()

	wsl.TryAccept("conn-1")
	wsl.TryAccept("conn-2")

	// Record heartbeat for conn-2 only
	time.Sleep(60 * time.Millisecond)
	wsl.RecordHeartbeat("conn-2")

	// conn-1 should be timed out, conn-2 should not
	timedOut := wsl.TimedOutConnections()

	found := false
	for _, id := range timedOut {
		if id == "conn-1" {
			found = true
		}
		if id == "conn-2" {
			t.Fatal("conn-2 should not be timed out (heartbeat was recorded)")
		}
	}

	if !found {
		t.Fatal("conn-1 should be in timed-out list")
	}
}

func TestWebSocketLimiter_ConcurrentAccess(t *testing.T) {
	wsl := newTestWSLimiter(100, 5*time.Minute)
	defer wsl.Stop()

	var wg sync.WaitGroup
	accepted := make(chan bool, 200)

	// Try to accept 200 connections concurrently with limit of 100
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ok := wsl.TryAccept(fmt.Sprintf("conn-%d", id))
			accepted <- ok
		}(i)
	}

	wg.Wait()
	close(accepted)

	acceptCount := 0
	rejectCount := 0
	for ok := range accepted {
		if ok {
			acceptCount++
		} else {
			rejectCount++
		}
	}

	if acceptCount != 100 {
		t.Fatalf("expected exactly 100 accepted, got %d", acceptCount)
	}
	if rejectCount != 100 {
		t.Fatalf("expected exactly 100 rejected, got %d", rejectCount)
	}
}

func TestWebSocketLimiter_DefaultConfig(t *testing.T) {
	cfg := DefaultWebSocketConfig("bid-service")

	if cfg.MaxConnections != 10000 {
		t.Fatalf("expected max connections 10000, got %d", cfg.MaxConnections)
	}
	if cfg.HeartbeatTimeout != 5*time.Minute {
		t.Fatalf("expected heartbeat timeout 5m, got %v", cfg.HeartbeatTimeout)
	}
	if cfg.ServiceName != "bid-service" {
		t.Fatalf("expected service name 'bid-service', got %s", cfg.ServiceName)
	}
}
