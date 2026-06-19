package pool

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// WebSocket resource limit errors.
var (
	ErrWebSocketCapacityExceeded = errors.New("websocket: connection limit reached, capacity exceeded")
	ErrWebSocketHeartbeatTimeout = errors.New("websocket: no heartbeat received within timeout")
)

// WebSocketConfig holds WebSocket connection management settings.
type WebSocketConfig struct {
	// MaxConnections is the maximum number of concurrent WebSocket connections per pod.
	// Default: 10000 per Bid Service pod (Requirement 16.3).
	MaxConnections int64

	// HeartbeatTimeout is the duration after which a connection with no
	// ping/pong exchange is terminated (Requirement 16.4: 5 minutes).
	HeartbeatTimeout time.Duration

	// ServiceName is used for metric labels.
	ServiceName string
}

// DefaultWebSocketConfig returns production-ready WebSocket defaults.
// 10000 max connections per pod, 5 min heartbeat timeout (Requirements 16.3, 16.4).
func DefaultWebSocketConfig(serviceName string) WebSocketConfig {
	return WebSocketConfig{
		MaxConnections:   10000,
		HeartbeatTimeout: 5 * time.Minute,
		ServiceName:      serviceName,
	}
}

// WebSocketLimiter manages WebSocket connection limits, heartbeat tracking,
// and exposes Prometheus metrics for connection count (Requirement 16.6).
type WebSocketLimiter struct {
	config  WebSocketConfig
	logger  *zap.Logger
	current atomic.Int64
	metrics *wsMetrics

	// heartbeat tracking
	mu         sync.RWMutex
	heartbeats map[string]time.Time // connID -> last heartbeat time

	stopOnce sync.Once
	stopCh   chan struct{}
}

type wsMetrics struct {
	activeConnections prometheus.Gauge
	totalAccepted     prometheus.Counter
	totalRejected     prometheus.Counter
	heartbeatTimeouts prometheus.Counter
}

// NewWebSocketLimiter creates a WebSocket connection limiter with metrics and heartbeat tracking.
func NewWebSocketLimiter(cfg WebSocketConfig, logger *zap.Logger, reg prometheus.Registerer) *WebSocketLimiter {
	metrics := registerWSMetrics(cfg.ServiceName, reg)

	wsl := &WebSocketLimiter{
		config:     cfg,
		logger:     logger,
		metrics:    metrics,
		heartbeats: make(map[string]time.Time),
		stopCh:     make(chan struct{}),
	}

	// Start heartbeat checker goroutine
	go wsl.checkHeartbeats()

	return wsl
}

// TryAccept attempts to accept a new WebSocket connection.
// Returns true if the connection is accepted, false if the limit is reached (Requirement 16.3).
func (wsl *WebSocketLimiter) TryAccept(connID string) bool {
	for {
		current := wsl.current.Load()
		if current >= wsl.config.MaxConnections {
			wsl.metrics.totalRejected.Inc()
			wsl.logger.Warn("WebSocket connection rejected: capacity exceeded",
				zap.Int64("current", current),
				zap.Int64("max", wsl.config.MaxConnections),
			)
			return false
		}
		if wsl.current.CompareAndSwap(current, current+1) {
			wsl.metrics.activeConnections.Set(float64(current + 1))
			wsl.metrics.totalAccepted.Inc()

			// Register heartbeat
			wsl.mu.Lock()
			wsl.heartbeats[connID] = time.Now()
			wsl.mu.Unlock()

			return true
		}
	}
}

// Release marks a WebSocket connection as closed and decrements the counter.
func (wsl *WebSocketLimiter) Release(connID string) {
	current := wsl.current.Add(-1)
	if current < 0 {
		wsl.current.Store(0)
		current = 0
	}
	wsl.metrics.activeConnections.Set(float64(current))

	wsl.mu.Lock()
	delete(wsl.heartbeats, connID)
	wsl.mu.Unlock()
}

// RecordHeartbeat updates the last heartbeat time for a connection.
func (wsl *WebSocketLimiter) RecordHeartbeat(connID string) {
	wsl.mu.Lock()
	wsl.heartbeats[connID] = time.Now()
	wsl.mu.Unlock()
}

// CurrentConnections returns the current number of active WebSocket connections.
func (wsl *WebSocketLimiter) CurrentConnections() int64 {
	return wsl.current.Load()
}

// IsAtCapacity returns true if the WebSocket connection limit has been reached.
func (wsl *WebSocketLimiter) IsAtCapacity() bool {
	return wsl.current.Load() >= wsl.config.MaxConnections
}

// TimedOutConnections returns connection IDs that have exceeded the heartbeat timeout.
// Callers should terminate these connections and call Release for each (Requirement 16.4).
func (wsl *WebSocketLimiter) TimedOutConnections() []string {
	wsl.mu.RLock()
	defer wsl.mu.RUnlock()

	var timedOut []string
	cutoff := time.Now().Add(-wsl.config.HeartbeatTimeout)

	for connID, lastBeat := range wsl.heartbeats {
		if lastBeat.Before(cutoff) {
			timedOut = append(timedOut, connID)
		}
	}

	return timedOut
}

// checkHeartbeats periodically scans for connections that have exceeded the heartbeat timeout.
// It logs warnings and increments the heartbeat timeout counter (Requirement 16.4).
func (wsl *WebSocketLimiter) checkHeartbeats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wsl.stopCh:
			return
		case <-ticker.C:
			timedOut := wsl.TimedOutConnections()
			for _, connID := range timedOut {
				wsl.metrics.heartbeatTimeouts.Inc()
				wsl.logger.Warn("WebSocket connection heartbeat timeout",
					zap.String("conn_id", connID),
					zap.Duration("timeout", wsl.config.HeartbeatTimeout),
				)
			}
		}
	}
}

// Stop stops the background heartbeat checker.
func (wsl *WebSocketLimiter) Stop() {
	wsl.stopOnce.Do(func() {
		close(wsl.stopCh)
	})
}

func registerWSMetrics(serviceName string, reg prometheus.Registerer) *wsMetrics {
	labels := prometheus.Labels{"service": serviceName}

	m := &wsMetrics{
		activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   "websocket",
			Name:        "connections_active",
			Help:        "Number of currently active WebSocket connections",
			ConstLabels: labels,
		}),
		totalAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "websocket",
			Name:        "connections_accepted_total",
			Help:        "Total number of WebSocket connections accepted",
			ConstLabels: labels,
		}),
		totalRejected: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "websocket",
			Name:        "connections_rejected_total",
			Help:        "Total number of WebSocket connections rejected due to capacity",
			ConstLabels: labels,
		}),
		heartbeatTimeouts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "websocket",
			Name:        "heartbeat_timeouts_total",
			Help:        "Total number of WebSocket connections terminated due to heartbeat timeout",
			ConstLabels: labels,
		}),
	}

	reg.MustRegister(m.activeConnections, m.totalAccepted, m.totalRejected, m.heartbeatTimeouts)
	return m
}
