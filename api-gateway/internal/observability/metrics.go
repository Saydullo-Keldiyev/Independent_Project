package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Namespace: "api_gateway", Name: "http_requests_total"},
		[]string{"method", "path", "status", "upstream"},
	)
	GatewayLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Namespace: "api_gateway", Name: "gateway_latency_seconds", Buckets: prometheus.DefBuckets},
		[]string{"upstream", "method"},
	)
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{Namespace: "api_gateway", Name: "rate_limit_hits_total"},
		[]string{"scope"},
	)
	JWTValidationErrors = promauto.NewCounter(
		prometheus.CounterOpts{Namespace: "api_gateway", Name: "jwt_validation_errors_total"},
	)
	ActiveWebSockets = promauto.NewGauge(
		prometheus.GaugeOpts{Namespace: "api_gateway", Name: "active_websockets"},
	)
	GatewayErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{Namespace: "api_gateway", Name: "gateway_errors_total"},
		[]string{"upstream", "reason"},
	)
)
