package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/model"
)

// InsertOutboxEvent writes an event to the outbox table within the same DB transaction.
// This guarantees the event is persisted atomically with the business operation.
func InsertOutboxEvent(ctx context.Context, tx pgx.Tx, e model.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, processed, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := tx.Exec(ctx, query,
		e.ID, e.AggregateType, e.AggregateID, e.EventType, e.Payload, e.Processed, e.CreatedAt,
	)
	return err
}

// GetUnprocessedEvents returns events that haven't been published to Kafka yet
func GetUnprocessedEvents(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, processed, created_at
		FROM outbox_events
		WHERE processed = FALSE
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := database.DB.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.OutboxEvent
	for rows.Next() {
		var e model.OutboxEvent
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.Payload, &e.Processed, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// MarkProcessed marks an outbox event as published
func MarkProcessed(ctx context.Context, eventID string) error {
	query := `UPDATE outbox_events SET processed = TRUE WHERE id = $1`
	_, err := database.DB.Exec(ctx, query, eventID)
	return err
}
