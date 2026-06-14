package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	NotificationsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "notification_service",
			Name:      "notifications_sent_total",
			Help:      "Total notifications sent by type and channel",
		},
		[]string{"type", "channel"}, // type: bid_placed, outbid, etc. channel: websocket, email
	)

	NotificationFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "notification_service",
			Name:      "notification_failures_total",
			Help:      "Total notification delivery failures",
		},
		[]string{"type", "channel"},
	)

	EmailRetryTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "notification_service",
			Name:      "email_retry_total",
			Help:      "Total email retry attempts",
		},
	)

	ActiveWSConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "notification_service",
			Name:      "active_ws_connections",
			Help:      "Current active WebSocket connections",
		},
	)

	KafkaMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "notification_service",
			Name:      "kafka_messages_processed_total",
			Help:      "Total Kafka messages processed",
		},
		[]string{"event_type", "status"}, // status: success, failed, dlq
	)

	DLQMessagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "notification_service",
			Name:      "dlq_messages_total",
			Help:      "Total messages sent to Dead Letter Queue",
		},
	)
)
