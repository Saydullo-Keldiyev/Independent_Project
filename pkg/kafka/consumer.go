package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// ConsumerConfig holds Kafka consumer settings
type ConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

// NewConsumer creates a Kafka reader (consumer group)
func NewConsumer(cfg ConsumerConfig) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,    // manual commit for at-least-once delivery
	})
}

// MessageHandler is the function signature for processing Kafka messages
type MessageHandler func(ctx context.Context, msg kafka.Message) error

// Consume reads messages in a loop and calls handler for each.
// Commits offset only after successful handler execution.
// Returns when ctx is cancelled.
func Consume(ctx context.Context, r *kafka.Reader, handler MessageHandler) error {
	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // context cancelled — clean shutdown
			}
			return fmt.Errorf("failed to fetch message: %w", err)
		}

		if err := handler(ctx, msg); err != nil {
			// Log but don't commit — message will be redelivered
			continue
		}

		// Commit only after successful processing
		if err := r.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("failed to commit message: %w", err)
		}
	}
}
