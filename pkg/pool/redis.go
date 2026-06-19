package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisConfig holds Redis connection pool settings per requirement 16.1.
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	MaxConns     int // max 10 per instance
	MinIdleConns int // min 3 per instance
	MaxIdleTime  time.Duration
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	ServiceName  string // used for metric labels
}

// DefaultRedisConfig returns production-ready Redis pool defaults.
// Max 10 connections, min 3, idle timeout 5 minutes per instance (Requirement 16.1).
func DefaultRedisConfig(addr, password string, db int, serviceName string) RedisConfig {
	return RedisConfig{
		Addr:         addr,
		Password:     password,
		DB:           db,
		MaxConns:     10,
		MinIdleConns: 3,
		MaxIdleTime:  5 * time.Minute,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		ServiceName:  serviceName,
	}
}

// ManagedRedisPool wraps redis.Client with metrics collection and pool exhaustion warnings.
type ManagedRedisPool struct {
	client  *redis.Client
	config  RedisConfig
	logger  *zap.Logger
	metrics *redisPoolMetrics

	stopOnce sync.Once
	stopCh   chan struct{}
}

type redisPoolMetrics struct {
	activeConns prometheus.Gauge
	idleConns   prometheus.Gauge
	totalConns  prometheus.Gauge
	waitingHits prometheus.Counter
	timeouts    prometheus.Counter
}

// NewRedisPool creates a configured Redis client with Prometheus metrics and pool monitoring.
func NewRedisPool(cfg RedisConfig, logger *zap.Logger, reg prometheus.Registerer) (*ManagedRedisPool, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            cfg.Addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.MaxConns,
		MinIdleConns:    cfg.MinIdleConns,
		ConnMaxIdleTime: cfg.MaxIdleTime,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	metrics := registerRedisMetrics(cfg.ServiceName, reg)

	mp := &ManagedRedisPool{
		client:  client,
		config:  cfg,
		logger:  logger,
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}

	go mp.collectMetrics()

	return mp, nil
}

// Client returns the underlying redis.Client for command execution.
func (mp *ManagedRedisPool) Client() *redis.Client {
	return mp.client
}

// Close stops metrics collection and closes the Redis client.
func (mp *ManagedRedisPool) Close() error {
	mp.stopOnce.Do(func() {
		close(mp.stopCh)
	})
	return mp.client.Close()
}

// HealthCheck pings Redis.
func (mp *ManagedRedisPool) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return mp.client.Ping(ctx).Err()
}

// collectMetrics periodically collects Redis pool stats and logs warnings
// when pool has no available connections for >5s (Requirement 16.7).
func (mp *ManagedRedisPool) collectMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var exhaustedSince *time.Time

	for {
		select {
		case <-mp.stopCh:
			return
		case <-ticker.C:
			stats := mp.client.PoolStats()

			active := stats.TotalConns - stats.IdleConns
			mp.metrics.activeConns.Set(float64(active))
			mp.metrics.idleConns.Set(float64(stats.IdleConns))
			mp.metrics.totalConns.Set(float64(stats.TotalConns))

			// Check pool exhaustion: all connections in use
			available := int(stats.TotalConns) - int(active)
			if available <= 0 && int(stats.TotalConns) >= mp.config.MaxConns {
				now := time.Now()
				if exhaustedSince == nil {
					exhaustedSince = &now
				} else if now.Sub(*exhaustedSince) > 5*time.Second {
					mp.logger.Warn("Redis pool exhausted for >5s",
						zap.String("pool", mp.config.ServiceName),
						zap.Uint32("active_conns", active),
						zap.Uint32("total_conns", stats.TotalConns),
						zap.Uint32("timeouts", stats.Timeouts),
					)
					// Reset to avoid flooding logs
					exhaustedSince = &now
				}
			} else {
				exhaustedSince = nil
			}
		}
	}
}

func registerRedisMetrics(serviceName string, reg prometheus.Registerer) *redisPoolMetrics {
	labels := prometheus.Labels{"service": serviceName, "pool": "redis"}

	m := &redisPoolMetrics{
		activeConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "pool",
			Subsystem:   "redis",
			Name:        "active_connections",
			Help:        "Number of currently active Redis connections",
			ConstLabels: labels,
		}),
		idleConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "pool",
			Subsystem:   "redis",
			Name:        "idle_connections",
			Help:        "Number of idle Redis connections",
			ConstLabels: labels,
		}),
		totalConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "pool",
			Subsystem:   "redis",
			Name:        "total_connections",
			Help:        "Total number of Redis connections in the pool",
			ConstLabels: labels,
		}),
		waitingHits: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "pool",
			Subsystem:   "redis",
			Name:        "pool_hits_total",
			Help:        "Total number of times a free connection was found in the pool",
			ConstLabels: labels,
		}),
		timeouts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "pool",
			Subsystem:   "redis",
			Name:        "timeouts_total",
			Help:        "Total number of pool timeout errors",
			ConstLabels: labels,
		}),
	}

	reg.MustRegister(m.activeConns, m.idleConns, m.totalConns, m.waitingHits, m.timeouts)
	return m
}
