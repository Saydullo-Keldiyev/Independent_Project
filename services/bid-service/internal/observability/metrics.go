package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ── HTTP / Bid metrics ────────────────────────────────────────────────────────

var (
	// BidRequestsTotal counts all bid placement attempts, labeled by status
	BidRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bid_service",
			Name:      "bid_requests_total",
			Help:      "Total number of bid placement requests",
		},
		[]string{"status"}, // "success" | "error" | "rejected"
	)

	// BidLatency tracks how long PlaceBid takes end-to-end
	BidLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "bid_service",
			Name:      "bid_latency_seconds",
			Help:      "End-to-end latency of bid placement in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"auction_id"},
	)

	// HTTPRequestsTotal counts all HTTP requests by method, path, status
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bid_service",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks HTTP handler latency
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "bid_service",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

// ── WebSocket metrics ─────────────────────────────────────────────────────────

var (
	// WebSocketConnections tracks currently active WebSocket connections
	WebSocketConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "bid_service",
			Name:      "websocket_connections_active",
			Help:      "Number of currently active WebSocket connections",
		},
	)

	// WebSocketMessagesTotal counts messages broadcast over WebSocket
	WebSocketMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bid_service",
			Name:      "websocket_messages_total",
			Help:      "Total WebSocket messages broadcast",
		},
		[]string{"auction_id"},
	)
)

// ── Redis metrics ─────────────────────────────────────────────────────────────

var (
	// RedisOperationDuration tracks Redis command latency
	RedisOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "bid_service",
			Name:      "redis_operation_duration_seconds",
			Help:      "Redis operation latency in seconds",
			Buckets:   []float64{.0001, .0005, .001, .005, .01, .05, .1},
		},
		[]string{"operation"}, // "get" | "set" | "del" | "lock"
	)

	// RedisErrorsTotal counts Redis errors by operation
	RedisErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bid_service",
			Name:      "redis_errors_total",
			Help:      "Total Redis errors",
		},
		[]string{"operation"},
	)
)

// ── Kafka metrics ─────────────────────────────────────────────────────────────

var (
	// KafkaPublishTotal counts Kafka publish attempts
	KafkaPublishTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bid_service",
			Name:      "kafka_publish_total",
			Help:      "Total Kafka publish attempts",
		},
		[]string{"topic", "status"}, // status: "success" | "error"
	)

	// KafkaPublishDuration tracks how long Kafka writes take
	KafkaPublishDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "bid_service",
			Name:      "kafka_publish_duration_seconds",
			Help:      "Kafka publish latency in seconds",
			Buckets:   []float64{.001, .005, .01, .05, .1, .5, 1},
		},
		[]string{"topic"},
	)
)

// ── Database metrics ──────────────────────────────────────────────────────────

var (
	// DBQueryDuration tracks database query latency
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "bid_service",
			Name:      "db_query_duration_seconds",
			Help:      "Database query latency in seconds",
			Buckets:   []float64{.001, .005, .01, .05, .1, .5, 1, 2},
		},
		[]string{"query"}, // "create_bid" | "get_highest_bid" | etc.
	)

	// DBErrorsTotal counts database errors
	DBErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bid_service",
			Name:      "db_errors_total",
			Help:      "Total database errors",
		},
		[]string{"query"},
	)

	// DBPoolAcquireDuration tracks how long it takes to get a DB connection
	DBPoolAcquireDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "bid_service",
			Name:      "db_pool_acquire_duration_seconds",
			Help:      "Time to acquire a DB connection from the pool",
			Buckets:   []float64{.0001, .001, .01, .1, .5},
		},
	)
)

// ── Lock metrics ──────────────────────────────────────────────────────────────

var (
	// LockAcquireTotal counts distributed lock acquisition attempts
	LockAcquireTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "bid_service",
			Name:      "lock_acquire_total",
			Help:      "Total distributed lock acquisition attempts",
		},
		[]string{"status"}, // "acquired" | "failed"
	)
)
