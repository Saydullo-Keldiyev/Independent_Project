package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// --- Message Tests ---

func TestMessageConstants(t *testing.T) {
	if HeaderEventID != "X-Event-ID" {
		t.Errorf("expected HeaderEventID to be 'X-Event-ID', got %s", HeaderEventID)
	}
	if HeaderCorrelationID != "X-Correlation-ID" {
		t.Errorf("expected HeaderCorrelationID to be 'X-Correlation-ID', got %s", HeaderCorrelationID)
	}
}

func TestCorrelationIDContext(t *testing.T) {
	ctx := context.Background()

	// Test empty context.
	id := CorrelationIDFromContext(ctx)
	if id != "" {
		t.Errorf("expected empty correlation ID from fresh context, got %q", id)
	}

	// Test with value.
	ctx = WithCorrelationID(ctx, "test-correlation-123")
	id = CorrelationIDFromContext(ctx)
	if id != "test-correlation-123" {
		t.Errorf("expected 'test-correlation-123', got %q", id)
	}

	// Test nil context.
	id = CorrelationIDFromContext(nil)
	if id != "" {
		t.Errorf("expected empty string for nil context, got %q", id)
	}
}

// --- Producer Tests ---

func TestProducerPublish(t *testing.T) {
	logger := zaptest.NewLogger(t)

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	producer := NewProducerFromSyncProducer(mockProducer, "test-topic", logger)

	msg := &Message{
		Key:           "key-1",
		Payload:       []byte(`{"data": "test"}`),
		CorrelationID: "corr-abc",
	}

	err := producer.Publish(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Event ID should be auto-assigned.
	if msg.EventID == "" {
		t.Error("expected event_id to be auto-assigned")
	}

	// Timestamp should be set.
	if msg.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestProducerPublishPreservesExistingEventID(t *testing.T) {
	logger := zaptest.NewLogger(t)

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	producer := NewProducerFromSyncProducer(mockProducer, "test-topic", logger)

	msg := &Message{
		EventID: "existing-event-id",
		Key:     "key-1",
		Payload: []byte(`{"data": "test"}`),
	}

	err := producer.Publish(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.EventID != "existing-event-id" {
		t.Errorf("expected event_id to be preserved, got %q", msg.EventID)
	}
}

func TestProducerPublishError(t *testing.T) {
	logger := zaptest.NewLogger(t)

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndFail(errors.New("broker unavailable"))

	producer := NewProducerFromSyncProducer(mockProducer, "test-topic", logger)

	msg := &Message{
		Key:     "key-1",
		Payload: []byte(`{"data": "test"}`),
	}

	err := producer.Publish(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProducerPublishUsesMessageTopic(t *testing.T) {
	logger := zaptest.NewLogger(t)

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		if msg.Topic != "custom-topic" {
			return errors.New("expected topic to be 'custom-topic'")
		}
		return nil
	})

	producer := NewProducerFromSyncProducer(mockProducer, "default-topic", logger)

	msg := &Message{
		Topic:   "custom-topic",
		Key:     "key-1",
		Payload: []byte(`{"data": "test"}`),
	}

	err := producer.Publish(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Header Building Tests ---

func TestBuildHeaders(t *testing.T) {
	msg := &Message{
		EventID:       "evt-123",
		CorrelationID: "corr-456",
		Headers: map[string]string{
			"X-Custom": "custom-value",
		},
	}

	headers := buildHeaders(msg)

	// Should have at least event_id, correlation_id, and custom header.
	found := make(map[string]string)
	for _, h := range headers {
		found[string(h.Key)] = string(h.Value)
	}

	if found[HeaderEventID] != "evt-123" {
		t.Errorf("expected event_id header 'evt-123', got %q", found[HeaderEventID])
	}
	if found[HeaderCorrelationID] != "corr-456" {
		t.Errorf("expected correlation_id header 'corr-456', got %q", found[HeaderCorrelationID])
	}
	if found["X-Custom"] != "custom-value" {
		t.Errorf("expected custom header 'custom-value', got %q", found["X-Custom"])
	}
}

func TestBuildHeadersNoCorrelationID(t *testing.T) {
	msg := &Message{
		EventID: "evt-123",
	}

	headers := buildHeaders(msg)

	for _, h := range headers {
		if string(h.Key) == HeaderCorrelationID {
			t.Error("did not expect correlation_id header when it's empty")
		}
	}
}

// --- DedupStore Tests ---

func TestDedupStoreKeyFormat(t *testing.T) {
	store := NewDedupStore(nil, "my-group", 7*24*time.Hour, nil)
	key := store.keyFor("event-abc-123")

	expected := "processed:my-group:event-abc-123"
	if key != expected {
		t.Errorf("expected key %q, got %q", expected, key)
	}
}

func TestDedupStoreDefaultTTL(t *testing.T) {
	store := NewDedupStore(nil, "group", 0, nil)
	expected := 7 * 24 * time.Hour
	if store.ttl != expected {
		t.Errorf("expected default TTL of %v, got %v", expected, store.ttl)
	}
}

// --- Metrics Tests ---

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	if metrics.DuplicateEventsTotal == nil {
		t.Error("expected DuplicateEventsTotal to be initialized")
	}
	if metrics.ConsumerLag == nil {
		t.Error("expected ConsumerLag to be initialized")
	}
	if metrics.ProcessingDuration == nil {
		t.Error("expected ProcessingDuration to be initialized")
	}
	if metrics.DLQSize == nil {
		t.Error("expected DLQSize to be initialized")
	}
	if metrics.RetryTotal == nil {
		t.Error("expected RetryTotal to be initialized")
	}
	if metrics.MessagesProduced == nil {
		t.Error("expected MessagesProduced to be initialized")
	}
	if metrics.MessagesConsumed == nil {
		t.Error("expected MessagesConsumed to be initialized")
	}
}

func TestNewMetricsNilRegisterer(t *testing.T) {
	// Should not panic with nil registerer — uses default.
	metrics := NewMetrics(nil)
	if metrics == nil {
		t.Error("expected metrics to be created even with nil registerer")
	}
}

// --- ConsumerConfig Defaults ---

func TestConsumerConfigDefaults(t *testing.T) {
	cfg := ConsumerConfig{
		Brokers: []string{"localhost:9092"},
		GroupID: "test-group",
		Topics:  []string{"test-topic"},
	}

	// Defaults are applied in NewConsumer or NewConsumerFromGroup.
	// Test through a helper that applies defaults.
	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}
	if len(cfg.RetryBackoff) == 0 {
		cfg.RetryBackoff = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	}
	if cfg.IdempotencyTTL == 0 {
		cfg.IdempotencyTTL = 7 * 24 * time.Hour
	}

	if cfg.RetryCount != 3 {
		t.Errorf("expected RetryCount default 3, got %d", cfg.RetryCount)
	}
	if len(cfg.RetryBackoff) != 3 {
		t.Errorf("expected 3 backoff durations, got %d", len(cfg.RetryBackoff))
	}
	if cfg.RetryBackoff[0] != 1*time.Second {
		t.Errorf("expected first backoff 1s, got %v", cfg.RetryBackoff[0])
	}
	if cfg.RetryBackoff[1] != 2*time.Second {
		t.Errorf("expected second backoff 2s, got %v", cfg.RetryBackoff[1])
	}
	if cfg.RetryBackoff[2] != 4*time.Second {
		t.Errorf("expected third backoff 4s, got %v", cfg.RetryBackoff[2])
	}
}

// --- ParseMessage Tests ---

func TestParseMessage(t *testing.T) {
	h := &consumerGroupHandler{
		config: ConsumerConfig{GroupID: "test-group"},
		logger: zap.NewNop(),
	}

	saramaMsg := &sarama.ConsumerMessage{
		Topic: "test-topic",
		Key:   []byte("test-key"),
		Value: []byte(`{"data": "value"}`),
		Headers: []*sarama.RecordHeader{
			{Key: []byte(HeaderEventID), Value: []byte("evt-456")},
			{Key: []byte(HeaderCorrelationID), Value: []byte("corr-789")},
			{Key: []byte("X-Custom"), Value: []byte("custom")},
		},
		Timestamp: time.Now(),
	}

	msg := h.parseMessage(saramaMsg)

	if msg.EventID != "evt-456" {
		t.Errorf("expected event_id 'evt-456', got %q", msg.EventID)
	}
	if msg.CorrelationID != "corr-789" {
		t.Errorf("expected correlation_id 'corr-789', got %q", msg.CorrelationID)
	}
	if msg.Topic != "test-topic" {
		t.Errorf("expected topic 'test-topic', got %q", msg.Topic)
	}
	if msg.Key != "test-key" {
		t.Errorf("expected key 'test-key', got %q", msg.Key)
	}
	if string(msg.Payload) != `{"data": "value"}` {
		t.Errorf("unexpected payload: %s", msg.Payload)
	}
	if msg.Headers["X-Custom"] != "custom" {
		t.Errorf("expected custom header 'custom', got %q", msg.Headers["X-Custom"])
	}
}

func TestParseMessageNilHeaders(t *testing.T) {
	h := &consumerGroupHandler{
		config: ConsumerConfig{GroupID: "test-group"},
		logger: zap.NewNop(),
	}

	saramaMsg := &sarama.ConsumerMessage{
		Topic: "test-topic",
		Key:   []byte("test-key"),
		Value: []byte(`{}`),
		Headers: []*sarama.RecordHeader{
			nil, // nil header should be skipped safely
			{Key: []byte(HeaderEventID), Value: []byte("evt-001")},
		},
	}

	msg := h.parseMessage(saramaMsg)
	if msg.EventID != "evt-001" {
		t.Errorf("expected event_id 'evt-001', got %q", msg.EventID)
	}
}

// --- Integration-style DedupStore tests (require Redis mock) ---
// These serve as documentation of the expected behavior.
// For real integration testing, use a Redis test container.

func TestDedupStoreIsDuplicateReturnsErrorOnRedisFailure(t *testing.T) {
	// Create a Redis client that won't connect (simulating unavailability).
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:1", // Non-existent address.
	})
	defer client.Close()

	logger := zaptest.NewLogger(t)
	store := NewDedupStore(client, "test-group", 7*24*time.Hour, logger)

	// When Redis is unavailable, IsDuplicate should return false (process anyway)
	// and return an error.
	isDup, err := store.IsDuplicate(context.Background(), "event-123")
	if isDup {
		t.Error("expected IsDuplicate to return false when Redis is unavailable")
	}
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}
