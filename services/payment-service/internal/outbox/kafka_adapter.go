package outbox

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaGoAdapter implements KafkaPublisher using segmentio/kafka-go Writer.
// This bridges the outbox publisher with the Kafka client library used in payment-service.
type KafkaGoAdapter struct {
	writer *kafka.Writer
	log    *zap.Logger
}

// NewKafkaGoAdapter creates a KafkaPublisher backed by segmentio/kafka-go.
func NewKafkaGoAdapter(brokers []string, log *zap.Logger) *KafkaGoAdapter {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           10 * time.Second,
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
	}

	return &KafkaGoAdapter{
		writer: writer,
		log:    log,
	}
}

// PublishMessage publishes a message to the specified Kafka topic.
func (a *KafkaGoAdapter) PublishMessage(ctx context.Context, topic, key string, payload []byte, headers map[string]string) error {
	kafkaHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	msg := kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   payload,
		Headers: kafkaHeaders,
		Time:    time.Now().UTC(),
	}

	return a.writer.WriteMessages(ctx, msg)
}

// Close shuts down the Kafka writer.
func (a *KafkaGoAdapter) Close() error {
	if a.writer != nil {
		return a.writer.Close()
	}
	return nil
}
