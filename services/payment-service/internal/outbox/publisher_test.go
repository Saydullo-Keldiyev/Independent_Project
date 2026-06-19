package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/auction-system/payment-service/internal/model"
	"go.uber.org/zap"
)

// mockKafkaPublisher is a test double for KafkaPublisher.
type mockKafkaPublisher struct {
	mu       sync.Mutex
	messages []publishedMessage
	failOn   map[string]error // eventID -> error
}

type publishedMessage struct {
	topic   string
	key     string
	payload []byte
	headers map[string]string
}

func newMockKafkaPublisher() *mockKafkaPublisher {
	return &mockKafkaPublisher{
		failOn: make(map[string]error),
	}
}

func (m *mockKafkaPublisher) PublishMessage(ctx context.Context, topic, key string, payload []byte, headers map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this message should fail based on outbox_id header
	if outboxID, ok := headers["outbox_id"]; ok {
		if err, shouldFail := m.failOn[outboxID]; shouldFail {
			return err
		}
	}

	m.messages = append(m.messages, publishedMessage{
		topic:   topic,
		key:     key,
		payload: payload,
		headers: headers,
	})
	return nil
}

func (m *mockKafkaPublisher) getMessages() []publishedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]publishedMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

// mockOutboxRepository is a test double for Repository operations.
type mockOutboxRepository struct {
	mu             sync.Mutex
	events         []model.OutboxEvent
	publishedIDs   map[string]time.Time
	failureRecords map[string]string
}

func newMockOutboxRepository(events []model.OutboxEvent) *mockOutboxRepository {
	return &mockOutboxRepository{
		events:         events,
		publishedIDs:   make(map[string]time.Time),
		failureRecords: make(map[string]string),
	}
}

func TestPublisher_PublishesUnpublishedEvents(t *testing.T) {
	kafka := newMockKafkaPublisher()

	events := []model.OutboxEvent{
		{
			ID:            "evt-1",
			AggregateType: "wallet",
			AggregateID:   "wallet-001",
			EventType:     "wallet.deposited",
			Payload:       `{"amount": 100.00}`,
			CreatedAt:     time.Now().UTC(),
			RetryCount:    0,
		},
		{
			ID:            "evt-2",
			AggregateType: "payment",
			AggregateID:   "pay-001",
			EventType:     "payment.settled",
			Payload:       `{"auction_id": "auc-1"}`,
			CreatedAt:     time.Now().UTC(),
			RetryCount:    0,
		},
	}

	// Test the publishEvent method directly
	log := zap.NewNop()
	pub := &Publisher{
		kafka:  kafka,
		config: DefaultPublisherConfig(),
		log:    log,
	}

	for _, event := range events {
		err := pub.publishEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error publishing event %s: %v", event.ID, err)
		}
	}

	messages := kafka.getMessages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Verify first message
	if messages[0].key != "wallet-001" {
		t.Errorf("expected key 'wallet-001', got %q", messages[0].key)
	}
	if messages[0].headers["aggregate_type"] != "wallet" {
		t.Errorf("expected aggregate_type header 'wallet', got %q", messages[0].headers["aggregate_type"])
	}
	if messages[0].headers["event_type"] != "wallet.deposited" {
		t.Errorf("expected event_type header 'wallet.deposited', got %q", messages[0].headers["event_type"])
	}

	// Verify second message
	if messages[1].key != "pay-001" {
		t.Errorf("expected key 'pay-001', got %q", messages[1].key)
	}
	if messages[1].headers["event_type"] != "payment.settled" {
		t.Errorf("expected event_type header 'payment.settled', got %q", messages[1].headers["event_type"])
	}
}

func TestPublisher_HandlesPublishFailure(t *testing.T) {
	kafka := newMockKafkaPublisher()
	kafka.failOn["evt-fail"] = errors.New("kafka broker unavailable")

	event := model.OutboxEvent{
		ID:            "evt-fail",
		AggregateType: "wallet",
		AggregateID:   "wallet-001",
		EventType:     "wallet.deposited",
		Payload:       `{"amount": 50.00}`,
		CreatedAt:     time.Now().UTC(),
		RetryCount:    2,
	}

	log := zap.NewNop()
	pub := &Publisher{
		kafka:  kafka,
		config: DefaultPublisherConfig(),
		log:    log,
	}

	err := pub.publishEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error from publish, got nil")
	}
	if err.Error() != "kafka broker unavailable" {
		t.Errorf("expected 'kafka broker unavailable' error, got %q", err.Error())
	}

	// Verify no messages were published
	messages := kafka.getMessages()
	if len(messages) != 0 {
		t.Errorf("expected 0 messages after failure, got %d", len(messages))
	}
}

func TestPublisher_UsesAggregateIDAsPartitionKey(t *testing.T) {
	kafka := newMockKafkaPublisher()

	event := model.OutboxEvent{
		ID:            "evt-key",
		AggregateType: "hold",
		AggregateID:   "hold-xyz-123",
		EventType:     "balance.held",
		Payload:       `{"hold_id": "hold-xyz-123"}`,
		CreatedAt:     time.Now().UTC(),
	}

	pub := &Publisher{
		kafka:  kafka,
		config: DefaultPublisherConfig(),
		log:    zap.NewNop(),
	}

	err := pub.publishEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := kafka.getMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].key != "hold-xyz-123" {
		t.Errorf("expected partition key 'hold-xyz-123', got %q", messages[0].key)
	}
}

func TestPublisher_IncludesAllRequiredHeaders(t *testing.T) {
	kafka := newMockKafkaPublisher()

	event := model.OutboxEvent{
		ID:            "evt-headers",
		AggregateType: "payment",
		AggregateID:   "pay-456",
		EventType:     "payment.charged",
		Payload:       `{"charged": true}`,
		CreatedAt:     time.Now().UTC(),
	}

	pub := &Publisher{
		kafka:  kafka,
		config: DefaultPublisherConfig(),
		log:    zap.NewNop(),
	}

	err := pub.publishEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := kafka.getMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	requiredHeaders := map[string]string{
		"aggregate_type": "payment",
		"aggregate_id":   "pay-456",
		"event_type":     "payment.charged",
		"outbox_id":      "evt-headers",
	}

	for key, expected := range requiredHeaders {
		got, ok := messages[0].headers[key]
		if !ok {
			t.Errorf("missing required header %q", key)
			continue
		}
		if got != expected {
			t.Errorf("header %q: expected %q, got %q", key, expected, got)
		}
	}
}

func TestPublisher_UsesConfiguredTopic(t *testing.T) {
	kafka := newMockKafkaPublisher()

	config := PublisherConfig{
		PollInterval: 1 * time.Second,
		BatchSize:    10,
		Topic:        "custom-payment-topic",
	}

	pub := &Publisher{
		kafka:  kafka,
		config: config,
		log:    zap.NewNop(),
	}

	event := model.OutboxEvent{
		ID:            "evt-topic",
		AggregateType: "wallet",
		AggregateID:   "w-1",
		EventType:     "wallet.credited",
		Payload:       `{}`,
		CreatedAt:     time.Now().UTC(),
	}

	err := pub.publishEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := kafka.getMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].topic != "custom-payment-topic" {
		t.Errorf("expected topic 'custom-payment-topic', got %q", messages[0].topic)
	}
}

func TestPublisher_RespectsContextCancellation(t *testing.T) {
	kafka := newMockKafkaPublisher()

	pub := &Publisher{
		kafka:  kafka,
		config: DefaultPublisherConfig(),
		log:    zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run should return quickly without blocking
	done := make(chan struct{})
	go func() {
		pub.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good, returned quickly
	case <-time.After(3 * time.Second):
		t.Fatal("publisher did not stop after context cancellation")
	}
}

func TestDefaultPublisherConfig(t *testing.T) {
	cfg := DefaultPublisherConfig()

	if cfg.PollInterval != 2*time.Second {
		t.Errorf("expected PollInterval 2s, got %v", cfg.PollInterval)
	}
	if cfg.BatchSize != 50 {
		t.Errorf("expected BatchSize 50, got %d", cfg.BatchSize)
	}
	if cfg.Topic != "payment-events" {
		t.Errorf("expected Topic 'payment-events', got %q", cfg.Topic)
	}
}

func TestOutboxEvent_IsPublished(t *testing.T) {
	t.Run("not published when PublishedAt is nil", func(t *testing.T) {
		e := model.OutboxEvent{ID: "e1"}
		if e.IsPublished() {
			t.Error("expected IsPublished() = false for nil PublishedAt")
		}
	})

	t.Run("published when PublishedAt is set", func(t *testing.T) {
		now := time.Now()
		e := model.OutboxEvent{ID: "e2", PublishedAt: &now}
		if !e.IsPublished() {
			t.Error("expected IsPublished() = true for non-nil PublishedAt")
		}
	})
}

func TestMaxOutboxRetries(t *testing.T) {
	if model.MaxOutboxRetries != 10 {
		t.Errorf("expected MaxOutboxRetries=10, got %d", model.MaxOutboxRetries)
	}
}
