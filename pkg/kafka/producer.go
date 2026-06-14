// Package kafka provides shared Kafka producer/consumer for all services.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// ProducerConfig holds Kafka producer settings
type ProducerConfig struct {
	Brokers []string
	Topic   string
	Async   bool // true = higher throughput, false = guaranteed delivery
}

// NewProducer creates a production-ready Kafka writer
func NewProducer(cfg ProducerConfig) *kafka.Writer {
	return &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.Topic,
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           10 * time.Second,
		ReadTimeout:            10 * time.Second,
		AllowAutoTopicCreation: true,
		Async:                  cfg.Async,
		// Compression for large payloads
		Compression: kafka.Snappy,
	}
}

// Publish sends a raw message to Kafka
func Publish(ctx context.Context, w *kafka.Writer, value []byte) error {
	if w == nil {
		return fmt.Errorf("kafka writer is nil")
	}
	return w.WriteMessages(ctx, kafka.Message{
		Value: value,
		Time:  time.Now(),
	})
}

// PublishEvent serializes an event and publishes it with an event type key
func PublishEvent(ctx context.Context, w *kafka.Writer, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event %s: %w", eventType, err)
	}
	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(eventType),
		Value: data,
		Time:  time.Now(),
	})
}
