// Package outbox implements the Transactional Outbox pattern for the Payment Service.
// It ensures atomicity between database writes (financial state changes) and event
// publication to Kafka by writing outbox entries within the same DB transaction,
// then publishing them asynchronously via a background polling loop.
//
// Requirements: 8.6
package outbox

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/auction-system/payment-service/internal/model"
)

// Repository handles persistence of outbox events.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new outbox Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertEvent writes an outbox event within an existing DB transaction.
// This is called within the same transaction as the financial state change,
// guaranteeing atomicity between business data and the event record.
func (r *Repository) InsertEvent(ctx context.Context, tx pgx.Tx, event model.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, idempotency_key, created_at, published_at, retry_count, last_error, processed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := tx.Exec(ctx, query,
		event.ID,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		event.IdempotencyKey,
		event.CreatedAt,
		event.PublishedAt,
		event.RetryCount,
		event.LastError,
		event.PublishedAt != nil, // processed = true if already published
	)
	return err
}

// GetUnpublishedEvents retrieves events that haven't been published yet,
// ordered by creation time. Uses FOR UPDATE SKIP LOCKED to allow concurrent
// polling without contention.
func (r *Repository) GetUnpublishedEvents(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, idempotency_key,
		       created_at, published_at, retry_count, last_error
		FROM outbox_events
		WHERE published_at IS NULL AND retry_count < $1
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.pool.Query(ctx, query, model.MaxOutboxRetries, limit)
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

// MarkPublished sets the published_at timestamp and processed=true for an event.
func (r *Repository) MarkPublished(ctx context.Context, eventID string, publishedAt time.Time) error {
	query := `UPDATE outbox_events SET published_at = $1, processed = TRUE WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, publishedAt, eventID)
	return err
}

// RecordFailure increments retry_count and records the error message.
func (r *Repository) RecordFailure(ctx context.Context, eventID string, errMsg string) error {
	query := `UPDATE outbox_events SET retry_count = retry_count + 1, last_error = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, errMsg, eventID)
	return err
}
