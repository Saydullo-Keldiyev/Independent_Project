package model

import "time"

// OutboxEvent is written in the same DB transaction as the business operation.
// A background worker reads unprocessed events and publishes them to Kafka.
// This guarantees at-least-once delivery without 2PC.
type OutboxEvent struct {
	ID            string    `db:"id"`
	AggregateType string    `db:"aggregate_type"` // "wallet", "payment", "hold"
	AggregateID   string    `db:"aggregate_id"`
	EventType     string    `db:"event_type"`
	Payload       string    `db:"payload"` // JSON
	Processed     bool      `db:"processed"`
	CreatedAt     time.Time `db:"created_at"`
}
