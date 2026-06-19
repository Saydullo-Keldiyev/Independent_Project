package model

import "time"

// OutboxEvent represents an event written in the same DB transaction as a business
// operation. A background publisher reads unpublished events and publishes them to Kafka.
// This guarantees at-least-once delivery without 2PC (Outbox Pattern).
//
// Requirements: 8.6
type OutboxEvent struct {
	ID             string     `db:"id"              json:"id"`
	AggregateType  string     `db:"aggregate_type"  json:"aggregate_type"`  // "wallet", "payment", "hold"
	AggregateID    string     `db:"aggregate_id"    json:"aggregate_id"`
	EventType      string     `db:"event_type"      json:"event_type"`
	Payload        string     `db:"payload"         json:"payload"`         // JSON
	IdempotencyKey *string    `db:"idempotency_key" json:"idempotency_key"` // Optional dedup key
	CreatedAt      time.Time  `db:"created_at"      json:"created_at"`
	PublishedAt    *time.Time `db:"published_at"    json:"published_at"`    // nil = not yet published
	RetryCount     int        `db:"retry_count"     json:"retry_count"`
	LastError      *string    `db:"last_error"      json:"last_error"`

	// Deprecated: use PublishedAt != nil instead. Kept for backward compat with old queries.
	Processed bool `db:"processed" json:"-"`
}

// IsPublished returns true if the event has been successfully published to Kafka.
func (e *OutboxEvent) IsPublished() bool {
	return e.PublishedAt != nil
}

// MaxRetries is the maximum number of publish attempts before the event is considered failed.
const MaxOutboxRetries = 10
