package outbox

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/auction-system/payment-service/internal/model"
)

// KafkaPublisher defines the interface for publishing messages to Kafka.
// This allows the outbox publisher to work with any Kafka producer implementation.
type KafkaPublisher interface {
	// PublishMessage publishes a single message to Kafka.
	// topic: target Kafka topic
	// key: partition key (typically aggregate_id)
	// payload: message body (JSON)
	// headers: additional metadata headers
	PublishMessage(ctx context.Context, topic, key string, payload []byte, headers map[string]string) error
}

// PublisherConfig holds configuration for the outbox Publisher.
type PublisherConfig struct {
	// PollInterval is how often the publisher checks for unpublished events.
	// Recommended: 1-5 seconds.
	PollInterval time.Duration

	// BatchSize is the maximum number of events to fetch per poll cycle.
	BatchSize int

	// Topic is the default Kafka topic to publish events to.
	Topic string
}

// DefaultPublisherConfig returns a sensible default configuration.
func DefaultPublisherConfig() PublisherConfig {
	return PublisherConfig{
		PollInterval: 2 * time.Second,
		BatchSize:    50,
		Topic:        "payment-events",
	}
}

// Publisher polls the outbox_events table for unpublished events and publishes them
// to Kafka. It runs as a background goroutine and handles retry_count and last_error
// tracking for failed publish attempts.
//
// Requirements: 8.6
type Publisher struct {
	repo   *Repository
	kafka  KafkaPublisher
	config PublisherConfig
	log    *zap.Logger
}

// NewPublisher creates a new outbox Publisher.
func NewPublisher(repo *Repository, kafka KafkaPublisher, config PublisherConfig, log *zap.Logger) *Publisher {
	if log == nil {
		log = zap.NewNop()
	}
	return &Publisher{
		repo:   repo,
		kafka:  kafka,
		config: config,
		log:    log,
	}
}

// Run starts the outbox polling loop. It blocks until ctx is cancelled.
// This should be launched as a background goroutine.
func (p *Publisher) Run(ctx context.Context) {
	p.log.Info("outbox publisher started",
		zap.Duration("poll_interval", p.config.PollInterval),
		zap.Int("batch_size", p.config.BatchSize),
		zap.String("topic", p.config.Topic),
	)

	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("outbox publisher stopped")
			return
		case <-ticker.C:
			p.pollAndPublish(ctx)
		}
	}
}

// pollAndPublish fetches unpublished events and attempts to publish them to Kafka.
func (p *Publisher) pollAndPublish(ctx context.Context) {
	events, err := p.repo.GetUnpublishedEvents(ctx, p.config.BatchSize)
	if err != nil {
		p.log.Error("failed to fetch unpublished outbox events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	published := 0
	failed := 0

	for _, event := range events {
		if ctx.Err() != nil {
			return
		}

		if err := p.publishEvent(ctx, event); err != nil {
			failed++
			p.handlePublishFailure(ctx, event, err)
		} else {
			published++
			p.handlePublishSuccess(ctx, event)
		}
	}

	if published > 0 || failed > 0 {
		p.log.Info("outbox batch completed",
			zap.Int("published", published),
			zap.Int("failed", failed),
			zap.Int("total", len(events)),
		)
	}
}

// publishEvent publishes a single outbox event to Kafka.
func (p *Publisher) publishEvent(ctx context.Context, event model.OutboxEvent) error {
	headers := map[string]string{
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"event_type":     event.EventType,
		"outbox_id":      event.ID,
	}

	// Determine the topic - use event-type based routing or default topic
	topic := p.topicForEvent(event)

	// Use aggregate_id as partition key for ordering guarantees
	return p.kafka.PublishMessage(ctx, topic, event.AggregateID, []byte(event.Payload), headers)
}

// topicForEvent determines the Kafka topic for a given event.
// Allows event-type based routing: payment events go to payment-events topic.
func (p *Publisher) topicForEvent(event model.OutboxEvent) string {
	// Could be extended with a topic mapping, for now use default
	return p.config.Topic
}

// handlePublishSuccess marks the event as published.
func (p *Publisher) handlePublishSuccess(ctx context.Context, event model.OutboxEvent) {
	now := time.Now().UTC()
	if err := p.repo.MarkPublished(ctx, event.ID, now); err != nil {
		p.log.Error("failed to mark event as published",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.EventType),
			zap.Error(err),
		)
	}
}

// handlePublishFailure records the failure with retry tracking.
func (p *Publisher) handlePublishFailure(ctx context.Context, event model.OutboxEvent, publishErr error) {
	errMsg := publishErr.Error()

	p.log.Warn("outbox event publish failed",
		zap.String("event_id", event.ID),
		zap.String("event_type", event.EventType),
		zap.String("aggregate_type", event.AggregateType),
		zap.String("aggregate_id", event.AggregateID),
		zap.Int("retry_count", event.RetryCount+1),
		zap.Int("max_retries", model.MaxOutboxRetries),
		zap.Error(publishErr),
	)

	if err := p.repo.RecordFailure(ctx, event.ID, errMsg); err != nil {
		p.log.Error("failed to record publish failure",
			zap.String("event_id", event.ID),
			zap.Error(err),
		)
	}
}

// outboxEventPayload is used for structured payload serialization when creating outbox entries.
type outboxEventPayload struct {
	Data json.RawMessage `json:"data"`
}
