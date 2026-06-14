package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ActiveAuctions = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "auction_service", Name: "active_auctions",
		Help: "Number of active auctions",
	})
	AuctionCreationTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "auction_service", Name: "auction_creation_total",
		Help: "Auctions created",
	})
	AuctionEndingLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "auction_service", Name: "auction_ending_latency_seconds",
		Help: "Time to complete auction ending flow", Buckets: prometheus.DefBuckets,
	})
	SchedulerErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "auction_service", Name: "scheduler_errors_total",
		Help: "Scheduler tick errors",
	})
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "auction_service", Name: "http_requests_total",
	}, []string{"method", "path", "status"})
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "auction_service", Name: "http_request_duration_seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)
