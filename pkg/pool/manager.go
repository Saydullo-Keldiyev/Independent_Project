package pool

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Manager aggregates all managed resource pools for a service instance.
// It provides a single point of initialization and shutdown for PostgreSQL,
// Redis, HTTP client, and WebSocket connection management.
type Manager struct {
	Postgres  *ManagedPgPool
	Redis     *ManagedRedisPool
	HTTP      *http.Client
	WebSocket *WebSocketLimiter
	logger    *zap.Logger
}

// ManagerConfig holds all pool configuration for a service instance.
type ManagerConfig struct {
	Postgres  *PostgresConfig
	Redis     *RedisConfig
	HTTP      *HTTPClientConfig
	WebSocket *WebSocketConfig
}

// NewManager creates a pool manager with the given configuration.
// Only non-nil configs result in pool initialization. For example,
// services without WebSocket support can omit the WebSocket config.
func NewManager(cfg ManagerConfig, logger *zap.Logger, reg prometheus.Registerer) (*Manager, error) {
	m := &Manager{
		logger: logger,
	}

	if cfg.Postgres != nil {
		pg, err := NewPostgresPool(*cfg.Postgres, logger, reg)
		if err != nil {
			return nil, err
		}
		m.Postgres = pg
	}

	if cfg.Redis != nil {
		r, err := NewRedisPool(*cfg.Redis, logger, reg)
		if err != nil {
			m.Close() // clean up already-created pools
			return nil, err
		}
		m.Redis = r
	}

	if cfg.HTTP != nil {
		m.HTTP = NewHTTPClient(*cfg.HTTP)
	} else {
		// Always create an HTTP client with defaults for inter-service calls
		defaultCfg := DefaultHTTPClientConfig()
		m.HTTP = NewHTTPClient(defaultCfg)
	}

	if cfg.WebSocket != nil {
		m.WebSocket = NewWebSocketLimiter(*cfg.WebSocket, logger, reg)
	}

	return m, nil
}

// Close shuts down all managed pools and stops background goroutines.
func (m *Manager) Close() {
	if m.Postgres != nil {
		m.Postgres.Close()
	}
	if m.Redis != nil {
		if err := m.Redis.Close(); err != nil {
			m.logger.Error("error closing Redis pool", zap.Error(err))
		}
	}
	if m.WebSocket != nil {
		m.WebSocket.Stop()
	}
	// HTTP client has no Close method; transport idle connections
	// are cleaned up via IdleConnTimeout.
}

// HealthCheck runs health checks on all active pools.
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)

	if m.Postgres != nil {
		results["postgres"] = m.Postgres.HealthCheck(ctx)
	}
	if m.Redis != nil {
		results["redis"] = m.Redis.HealthCheck(ctx)
	}

	return results
}
