package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	LoginAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "user_service",
			Name:      "login_attempts_total",
			Help:      "Login attempts by outcome",
		},
		[]string{"status"}, // attempt | success | failed
	)

	JWTValidationErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "user_service",
			Name:      "jwt_validation_errors_total",
			Help:      "JWT validation failures",
		},
	)

	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "user_service",
			Name:      "active_sessions",
			Help:      "Sessions active in last 24h",
		},
	)

	WalletOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "user_service",
			Name:      "wallet_operations_total",
			Help:      "Wallet operations",
		},
		[]string{"operation"}, // deposit | hold | release
	)

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "user_service",
			Name:      "http_requests_total",
			Help:      "HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "user_service",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP latency",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)
