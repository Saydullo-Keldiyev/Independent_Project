package pool

import (
	"testing"
	"time"
)

func TestDefaultPostgresConfig(t *testing.T) {
	cfg := DefaultPostgresConfig("postgres://localhost/test", "bid-service")

	if cfg.MaxConns != 25 {
		t.Fatalf("expected max 25 connections, got %d", cfg.MaxConns)
	}
	if cfg.MinConns != 5 {
		t.Fatalf("expected min 5 connections, got %d", cfg.MinConns)
	}
	if cfg.MaxConnIdleTime != 5*time.Minute {
		t.Fatalf("expected 5 min idle timeout, got %v", cfg.MaxConnIdleTime)
	}
	if cfg.ServiceName != "bid-service" {
		t.Fatalf("expected service name 'bid-service', got %s", cfg.ServiceName)
	}
}

func TestDefaultRedisConfig(t *testing.T) {
	cfg := DefaultRedisConfig("localhost:6379", "secret", 0, "auction-service")

	if cfg.MaxConns != 10 {
		t.Fatalf("expected max 10 connections, got %d", cfg.MaxConns)
	}
	if cfg.MinIdleConns != 3 {
		t.Fatalf("expected min 3 idle connections, got %d", cfg.MinIdleConns)
	}
	if cfg.MaxIdleTime != 5*time.Minute {
		t.Fatalf("expected 5 min idle timeout, got %v", cfg.MaxIdleTime)
	}
	if cfg.ServiceName != "auction-service" {
		t.Fatalf("expected service name 'auction-service', got %s", cfg.ServiceName)
	}
}
