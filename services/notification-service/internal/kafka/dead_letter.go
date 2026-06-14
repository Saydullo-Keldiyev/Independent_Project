package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// DeadLetterQueue publishes failed messages to a DLQ topic for later inspection.
// Messages that fail after all retries end up here — never lost.
type DeadLetterQueue struct {
	writer *kafka.Writer
	log    *zap.Logger
}

func NewDeadLetterQueue(brokers []string, log *zap.Logger) *DeadLetterQueue {
	return &DeadLetterQueue{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  "notification-dlq",
			Balancer:               &kafka.LeastBytes{},
			WriteTimeout:           10 * time.Second,
			AllowAutoTopicCreation: true,
		},
		log: log,
	}
}

// Send publishes a failed message to the DLQ with error context
func (dlq *DeadLetterQueue) Send(ctx context.Context, originalKey, originalValue []byte, reason string) error {
	headers := []kafka.Header{
		{Key: "dlq-reason", Value: []byte(reason)},
		{Key: "dlq-timestamp", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		{Key: "original-key", Value: originalKey},
	}

	err := dlq.writer.WriteMessages(ctx, kafka.Message{
		Key:     originalKey,
		Value:   originalValue,
		Headers: headers,
		Time:    time.Now(),
	})

	if err != nil {
		dlq.log.Error("failed to write to DLQ",
			zap.String("reason", reason),
			zap.Error(err),
		)
		return fmt.Errorf("dlq write failed: %w", err)
	}

	dlq.log.Warn("message sent to DLQ",
		zap.String("key", string(originalKey)),
		zap.String("reason", reason),
	)
	return nil
}

func (dlq *DeadLetterQueue) Close() error {
	if dlq.writer != nil {
		return dlq.writer.Close()
	}
	return nil
}
