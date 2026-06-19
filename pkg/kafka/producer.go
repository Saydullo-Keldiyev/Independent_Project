// Package kafka provides an idempotent Kafka producer/consumer with DLQ support,
// Redis-based deduplication, and Prometheus metrics instrumentation.
package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ProducerConfig holds configuration for the Kafka producer.
type ProducerConfig struct {
	Brokers []string
	Topic   string
}

// Producer wraps a Sarama SyncProducer with idempotent message production.
type Producer struct {
	producer sarama.SyncProducer
	topic    string
	logger   *zap.Logger
}

// NewProducer creates a production-ready idempotent Kafka producer.
func NewProducer(cfg ProducerConfig, logger *zap.Logger) (*Producer, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	saramaCfg := sarama.NewConfig()
	saramaCfg.Producer.Return.Successes = true
	saramaCfg.Producer.RequiredAcks = sarama.WaitForAll
	saramaCfg.Producer.Idempotent = true
	saramaCfg.Net.MaxOpenRequests = 1
	saramaCfg.Producer.Retry.Max = 3
	saramaCfg.Producer.Retry.Backoff = 100 * time.Millisecond

	syncProducer, err := sarama.NewSyncProducer(cfg.Brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	return &Producer{
		producer: syncProducer,
		topic:    cfg.Topic,
		logger:   logger,
	}, nil
}

// NewProducerFromSyncProducer creates a Producer from an existing sarama.SyncProducer.
// This is useful for testing with mock producers.
func NewProducerFromSyncProducer(syncProducer sarama.SyncProducer, topic string, logger *zap.Logger) *Producer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Producer{
		producer: syncProducer,
		topic:    topic,
		logger:   logger,
	}
}

// Publish sends a message to Kafka with an auto-generated UUID v4 event_id
// and propagates the X-Correlation-ID from context or explicit message field.
func (p *Producer) Publish(ctx context.Context, msg *Message) error {
	// Assign UUID v4 event_id if not already set.
	if msg.EventID == "" {
		msg.EventID = uuid.New().String()
	}

	// Set timestamp if not already set.
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Use producer's topic if message topic is not specified.
	topic := msg.Topic
	if topic == "" {
		topic = p.topic
	}

	// Build Sarama message headers.
	headers := buildHeaders(msg)

	saramaMsg := &sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(msg.Key),
		Value:     sarama.ByteEncoder(msg.Payload),
		Headers:   headers,
		Timestamp: msg.Timestamp,
	}

	partition, offset, err := p.producer.SendMessage(saramaMsg)
	if err != nil {
		p.logger.Error("failed to publish kafka message",
			zap.String("event_id", msg.EventID),
			zap.String("topic", topic),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	p.logger.Debug("kafka message published",
		zap.String("event_id", msg.EventID),
		zap.String("topic", topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
		zap.String("correlation_id", msg.CorrelationID),
	)

	return nil
}

// Close shuts down the producer, flushing any pending messages.
func (p *Producer) Close() error {
	return p.producer.Close()
}

// buildHeaders converts message metadata to Sarama record headers.
func buildHeaders(msg *Message) []sarama.RecordHeader {
	headers := []sarama.RecordHeader{
		{
			Key:   []byte(HeaderEventID),
			Value: []byte(msg.EventID),
		},
	}

	if msg.CorrelationID != "" {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte(HeaderCorrelationID),
			Value: []byte(msg.CorrelationID),
		})
	}

	// Add any custom headers from the message.
	for k, v := range msg.Headers {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	return headers
}
