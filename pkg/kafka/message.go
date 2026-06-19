package kafka

import (
	"context"
	"time"
)

const (
	// HeaderEventID is the Kafka header key for the unique event identifier.
	HeaderEventID = "X-Event-ID"

	// HeaderCorrelationID is the Kafka header key for the correlation identifier.
	HeaderCorrelationID = "X-Correlation-ID"
)

// Message represents a Kafka message with metadata for idempotency and tracing.
type Message struct {
	EventID       string            // UUID v4, auto-assigned at production time
	CorrelationID string            // Propagated X-Correlation-ID
	Topic         string            // Target topic (uses producer default if empty)
	Key           string            // Partition key
	Payload       []byte            // Serialized message body
	Headers       map[string]string // Additional headers
	Timestamp     time.Time         // Message production time
}

// MessageHandler is the function signature for processing consumed Kafka messages.
// It receives a context (with correlation ID) and the message.
// Return nil on success, or an error to trigger retry/DLQ logic.
type MessageHandler func(ctx context.Context, msg *Message) error

// Consumer defines the interface for a Kafka consumer with idempotency and DLQ.
type Consumer interface {
	// Start begins consuming messages, calling the handler for each unique message.
	Start(ctx context.Context, handler MessageHandler) error
	// Stop gracefully stops the consumer.
	Stop() error
}

// context key type to avoid collisions.
type contextKey string

const (
	correlationIDCtxKey contextKey = "correlation_id"
)

// WithCorrelationID stores a correlation ID in context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDCtxKey, id)
}

// CorrelationIDFromContext retrieves the correlation ID from context.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(correlationIDCtxKey).(string)
	return id
}
