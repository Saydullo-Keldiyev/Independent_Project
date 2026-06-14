package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

var Writer *kafka.Writer

type ProducerConfig struct {
	Brokers []string
	Topic   string
}

func InitProducer(cfg ProducerConfig) {
	Writer = &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.Topic,
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           10 * time.Second,
		ReadTimeout:            10 * time.Second,
		AllowAutoTopicCreation: true,
		// Async for high throughput — set to false for guaranteed delivery
		Async: false,
	}
}

// Publish sends a raw string message to Kafka
func Publish(message string) error {
	if Writer == nil {
		return fmt.Errorf("kafka writer not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return Writer.WriteMessages(ctx, kafka.Message{
		Value: []byte(message),
		Time:  time.Now(),
	})
}

// PublishEvent serializes an event struct and publishes it to Kafka.
// Records Prometheus metrics for latency and error rate.
func PublishEvent(eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	topic := Writer.Topic

	err = Writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(eventType),
		Value: data,
		Time:  time.Now(),
	})

	duration := time.Since(start).Seconds()

	// Record metrics (import cycle avoided — metrics accessed via interface)
	recordKafkaMetrics(topic, duration, err)

	return err
}

// recordKafkaMetrics is a thin wrapper so kafka package doesn't import observability
// (avoids circular imports). Set this from main.go after init.
var recordKafkaMetrics = func(topic string, duration float64, err error) {}

// SetMetricsRecorder injects the metrics recorder from main to avoid import cycles
func SetMetricsRecorder(fn func(topic string, duration float64, err error)) {
	recordKafkaMetrics = fn
}

func Close() {
	if Writer != nil {
		Writer.Close()
	}
}
