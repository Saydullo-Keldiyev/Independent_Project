// Package pool provides production-ready connection pool management
// with Prometheus metrics, resource limits, and health monitoring.
package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// PostgresConfig holds PostgreSQL connection pool settings per requirement 16.1.
type PostgresConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	HealthCheck     time.Duration
	ServiceName     string // used for metric labels
}

// DefaultPostgresConfig returns production-ready PostgreSQL pool defaults.
// Max 25 connections, min 5, idle timeout 5 minutes per instance (Requirement 16.1).
func DefaultPostgresConfig(url, serviceName string) PostgresConfig {
	return PostgresConfig{
		URL:             url,
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		HealthCheck:     1 * time.Minute,
		ServiceName:     serviceName,
	}
}

// ManagedPgPool wraps pgxpool.Pool with metrics collection and pool exhaustion warnings.
type ManagedPgPool struct {
	pool    *pgxpool.Pool
	config  PostgresConfig
	logger  *zap.Logger
	metrics *pgPoolMetrics

	stopOnce sync.Once
	stopCh   chan struct{}
}

type pgPoolMetrics struct {
	activeConns  prometheus.Gauge
	idleConns    prometheus.Gauge
	totalConns   prometheus.Gauge
	waitingCount prometheus.Gauge
	acquireCount prometheus.Counter
	acquireDur   prometheus.Histogram
}

// NewPostgresPool creates a configured pgxpool with Prometheus metrics and pool monitoring.
func NewPostgresPool(cfg PostgresConfig, logger *zap.Logger, reg prometheus.Registerer) (*ManagedPgPool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheck

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	metrics := registerPgMetrics(cfg.ServiceName, reg)

	mp := &ManagedPgPool{
		pool:    pool,
		config:  cfg,
		logger:  logger,
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}

	go mp.collectMetrics()

	return mp, nil
}

// Pool returns the underlying pgxpool.Pool for query execution.
func (mp *ManagedPgPool) Pool() *pgxpool.Pool {
	return mp.pool
}

// Close stops metrics collection and closes the pool.
func (mp *ManagedPgPool) Close() {
	mp.stopOnce.Do(func() {
		close(mp.stopCh)
	})
	mp.pool.Close()
}

// HealthCheck pings the database.
func (mp *ManagedPgPool) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return mp.pool.Ping(ctx)
}

// collectMetrics periodically collects pool stats and logs warnings
// when pool has no available connections for >5s (Requirement 16.7).
func (mp *ManagedPgPool) collectMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var exhaustedSince *time.Time

	for {
		select {
		case <-mp.stopCh:
			return
		case <-ticker.C:
			stat := mp.pool.Stat()

			mp.metrics.activeConns.Set(float64(stat.AcquiredConns()))
			mp.metrics.idleConns.Set(float64(stat.IdleConns()))
			mp.metrics.totalConns.Set(float64(stat.TotalConns()))

			// pgxpool doesn't expose waiting count directly via Stat(),
			// but we can infer from EmptyAcquireCount over time
			mp.metrics.acquireCount.Add(float64(stat.AcquireCount()))

			// Check pool exhaustion: if all connections are acquired and pool is at max
			available := int32(stat.TotalConns()) - int32(stat.AcquiredConns())
			if available <= 0 && stat.TotalConns() >= mp.config.MaxConns {
				now := time.Now()
				if exhaustedSince == nil {
					exhaustedSince = &now
				} else if now.Sub(*exhaustedSince) > 5*time.Second {
					mp.logger.Warn("PostgreSQL pool exhausted for >5s",
						zap.String("pool", mp.config.ServiceName),
						zap.Int32("active_conns", int32(stat.AcquiredConns())),
						zap.Int32("total_conns", int32(stat.TotalConns())),
						zap.Int64("empty_acquire_count", stat.EmptyAcquireCount()),
					)
					// Reset to avoid flooding logs — will warn again after another 5s
					exhaustedSince = &now
				}
			} else {
				exhaustedSince = nil
			}
		}
	}
}

func registerPgMetrics(serviceName string, reg prometheus.Registerer) *pgPoolMetrics {
	labels := prometheus.Labels{"service": serviceName, "pool": "postgres"}

	m := &pgPoolMetrics{
		activeConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "pool",
			Subsystem:   "postgres",
			Name:        "active_connections",
			Help:        "Number of currently acquired PostgreSQL connections",
			ConstLabels: labels,
		}),
		idleConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "pool",
			Subsystem:   "postgres",
			Name:        "idle_connections",
			Help:        "Number of idle PostgreSQL connections",
			ConstLabels: labels,
		}),
		totalConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "pool",
			Subsystem:   "postgres",
			Name:        "total_connections",
			Help:        "Total number of PostgreSQL connections in the pool",
			ConstLabels: labels,
		}),
		waitingCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "pool",
			Subsystem:   "postgres",
			Name:        "waiting_requests",
			Help:        "Number of requests waiting for a PostgreSQL connection",
			ConstLabels: labels,
		}),
		acquireCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "pool",
			Subsystem:   "postgres",
			Name:        "acquire_total",
			Help:        "Total number of connection acquire operations",
			ConstLabels: labels,
		}),
		acquireDur: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:   "pool",
			Subsystem:   "postgres",
			Name:        "acquire_duration_seconds",
			Help:        "Time to acquire a PostgreSQL connection from the pool",
			ConstLabels: labels,
			Buckets:     []float64{.0001, .001, .01, .05, .1, .5, 1, 5},
		}),
	}

	reg.MustRegister(m.activeConns, m.idleConns, m.totalConns, m.waitingCount, m.acquireCount, m.acquireDur)
	return m
}
