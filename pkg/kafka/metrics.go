package kafka

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the Kafka consumer/producer.
type Metrics struct {
	// DuplicateEventsTotal counts the number of duplicate events skipped.
	DuplicateEventsTotal *prometheus.CounterVec

	// ConsumerLag tracks the current consumer lag per consumer group/topic.
	ConsumerLag *prometheus.GaugeVec

	// ProcessingDuration tracks the message processing duration histogram per topic.
	ProcessingDuration *prometheus.HistogramVec

	// DLQSize tracks the number of messages sent to the Dead Letter Queue per topic.
	DLQSize *prometheus.CounterVec

	// RetryTotal counts the number of retries per topic.
	RetryTotal *prometheus.CounterVec

	// MessagesProduced counts the total number of messages produced.
	MessagesProduced *prometheus.CounterVec

	// MessagesConsumed counts the total number of messages consumed.
	MessagesConsumed *prometheus.CounterVec
}

// NewMetrics creates and registers Prometheus metrics for the kafka package.
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	factory := promauto.With(registerer)

	return &Metrics{
		DuplicateEventsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "auction",
				Subsystem: "kafka",
				Name:      "duplicate_events_total",
				Help:      "Total number of duplicate events skipped by idempotency check.",
			},
			[]string{"consumer_group", "topic"},
		),

		ConsumerLag: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "auction",
				Subsystem: "kafka",
				Name:      "consumer_lag",
				Help:      "Current consumer lag per consumer group and topic.",
			},
			[]string{"consumer_group", "topic", "partition"},
		),

		ProcessingDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "auction",
				Subsystem: "kafka",
				Name:      "processing_duration_seconds",
				Help:      "Histogram of message processing duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"consumer_group", "topic"},
		),

		DLQSize: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "auction",
				Subsystem: "kafka",
				Name:      "dlq_messages_total",
				Help:      "Total number of messages sent to the Dead Letter Queue.",
			},
			[]string{"consumer_group", "topic"},
		),

		RetryTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "auction",
				Subsystem: "kafka",
				Name:      "retry_total",
				Help:      "Total number of message processing retries.",
			},
			[]string{"consumer_group", "topic"},
		),

		MessagesProduced: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "auction",
				Subsystem: "kafka",
				Name:      "messages_produced_total",
				Help:      "Total number of messages produced.",
			},
			[]string{"topic"},
		),

		MessagesConsumed: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "auction",
				Subsystem: "kafka",
				Name:      "messages_consumed_total",
				Help:      "Total number of messages consumed.",
			},
			[]string{"consumer_group", "topic"},
		),
	}
}
