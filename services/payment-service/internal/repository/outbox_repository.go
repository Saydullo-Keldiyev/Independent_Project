package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/model"
)

// InsertOutboxEvent writes an event to the outbox table within the same DB transaction.
// This guarantees the event is persisted atomically with the business operation.
// Requirements: 8.6
func InsertOutboxEvent(ctx context.Context, tx pgx.Tx, e model.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, idempotency_key, created_at, published_at, retry_count, last_error, processed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := tx.Exec(ctx, query,
		e.ID,
		e.AggregateType,
		e.AggregateID,
		e.EventType,
		e.Payload,
		e.IdempotencyKey,
		e.CreatedAt,
		e.PublishedAt,
		e.RetryCount,
		e.LastError,
		e.PublishedAt != nil, // processed mirrors published_at for backward compat
	)
	return err
}

// GetUnprocessedEvents returns events that haven't been published to Kafka yet.
// Uses FOR UPDATE SKIP LOCKED for safe concurrent polling.
func GetUnprocessedEvents(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, idempotency_key,
		       created_at, published_at, retry_count, last_error
		FROM outbox_events
		WHERE published_at IS NULL AND retry_count < $1
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`
	rows, err := database.DB.Query(ctx, query, model.MaxOutboxRetries, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.OutboxEvent
	for rows.Next() {
		var e model.OutboxEvent
		if err := rows.Scan(
			&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload,
			&e.IdempotencyKey, &e.CreatedAt, &e.PublishedAt, &e.RetryCount, &e.LastError,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// MarkProcessed marks an outbox event as published with a timestamp.
func MarkProcessed(ctx context.Context, eventID string) error {
	query := `UPDATE outbox_events SET published_at = $1, processed = TRUE WHERE id = $2`
	_, err := database.DB.Exec(ctx, query, time.Now().UTC(), eventID)
	return err
}

// RecordOutboxFailure increments the retry count and records the error.
func RecordOutboxFailure(ctx context.Context, eventID string, errMsg string) error {
	query := `UPDATE outbox_events SET retry_count = retry_count + 1, last_error = $1 WHERE id = $2`
	_, err := database.DB.Exec(ctx, query, errMsg, eventID)
	return err
}
