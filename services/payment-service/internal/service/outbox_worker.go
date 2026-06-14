package service

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/auction-system/payment-service/internal/repository"
)

// OutboxWorker polls the outbox table and publishes events to Kafka.
// Guarantees at-least-once delivery without distributed transactions.
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

	for _, event := range events {
		msg := kafka.Message{
			Key:   []byte(event.EventType),
			Value: []byte(event.Payload),
			Time:  event.CreatedAt,
			Headers: []kafka.Header{
				{Key: "aggregate_type", Value: []byte(event.AggregateType)},
				{Key: "aggregate_id", Value: []byte(event.AggregateID)},
			},
		}

		if err := w.writer.WriteMessages(ctx, msg); err != nil {
			w.log.Error("failed to publish outbox event",
				zap.String("event_id", event.ID),
				zap.String("event_type", event.EventType),
				zap.Error(err),
			)
			continue // retry next tick
		}

		// Mark as processed only after successful Kafka publish
		if err := repository.MarkProcessed(ctx, event.ID); err != nil {
			w.log.Error("failed to mark event processed", zap.String("event_id", event.ID), zap.Error(err))
		}
	}

	w.log.Debug("outbox batch processed", zap.Int("count", len(events)))
}

func (w *OutboxWorker) Close() error {
	if w.writer != nil {
		return w.writer.Close()
	}
	return nil
}
