package service

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/auction-system/payment-service/internal/model"
	"github.com/auction-system/payment-service/internal/repository"
)

// OutboxWorker polls the outbox table and publishes events to Kafka.
// It guarantees at-least-once delivery without distributed transactions.
// Handles retry_count and last_error tracking for failed publish attempts.
//
// Requirements: 8.6
type OutboxWorker struct {
	writer   *kafka.Writer
	log      *zap.Logger
	interval time.Duration
}

func NewOutboxWorker(brokers []string, topic string, log *zap.Logger) *OutboxWorker {
	return &OutboxWorker{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.LeastBytes{},
			WriteTimeout:           10 * time.Second,
			RequiredAcks:           kafka.RequireAll,
			AllowAutoTopicCreation: true,
		},
		log:      log,
		interval: 2 * time.Second,
	}
}

// Run starts the outbox polling loop. Blocks until ctx is cancelled.
func (w *OutboxWorker) Run(ctx context.Context) {
	w.log.Info("outbox worker started", zap.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("outbox worker stopped")
			return
		case <-ticker.C:
			w.processOutbox(ctx)
		}
	}
}

func (w *OutboxWorker) processOutbox(ctx context.Context) {
	events, err := repository.GetUnprocessedEvents(ctx, 50)
	if err != nil {
		w.log.Error("failed to get outbox events", zap.Error(err))
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

		msg := kafka.Message{
			Key:   []byte(event.AggregateID),
			Value: []byte(event.Payload),
			Time:  event.CreatedAt,
			Headers: []kafka.Header{
				{Key: "aggregate_type", Value: []byte(event.AggregateType)},
				{Key: "aggregate_id", Value: []byte(event.AggregateID)},
				{Key: "event_type", Value: []byte(event.EventType)},
				{Key: "outbox_id", Value: []byte(event.ID)},
			},
		}

		if err := w.writer.WriteMessages(ctx, msg); err != nil {
			failed++
			w.log.Warn("outbox event publish failed",
				zap.String("event_id", event.ID),
				zap.String("event_type", event.EventType),
				zap.Int("retry_count", event.RetryCount+1),
				zap.Int("max_retries", model.MaxOutboxRetries),
				zap.Error(err),
			)
			// Record the failure: increment retry_count and store last_error
			if recordErr := repository.RecordOutboxFailure(ctx, event.ID, err.Error()); recordErr != nil {
				w.log.Error("failed to record outbox failure",
					zap.String("event_id", event.ID),
					zap.Error(recordErr),
				)
			}
			continue
		}

		// Mark as published only after successful Kafka publish
		published++
		if err := repository.MarkProcessed(ctx, event.ID); err != nil {
			w.log.Error("failed to mark event published",
				zap.String("event_id", event.ID),
				zap.Error(err),
			)
		}
	}

	if published > 0 || failed > 0 {
		w.log.Info("outbox batch completed",
			zap.Int("published", published),
			zap.Int("failed", failed),
			zap.Int("total", len(events)),
		)
	}
}

func (w *OutboxWorker) Close() error {
	if w.writer != nil {
		return w.writer.Close()
	}
	return nil
}
