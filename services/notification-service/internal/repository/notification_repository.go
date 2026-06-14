package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auction-system/notification-service/internal/database"
	"github.com/auction-system/notification-service/internal/model"
)

// Create inserts a new notification. Returns error if event_id already exists (idempotency).
func Create(ctx context.Context, n model.Notification) error {
	query := `
		INSERT INTO notifications (id, user_id, type, title, message, metadata, event_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (event_id) DO NOTHING
	`
	_, err := database.DB.Exec(ctx, query,
		n.ID, n.UserID, n.Type, n.Title, n.Message, n.Metadata, n.EventID, n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// CreateDelivery inserts a delivery attempt record
func CreateDelivery(ctx context.Context, d model.NotificationDelivery) error {
	query := `
		INSERT INTO notification_deliveries (id, notification_id, channel, status, max_retries, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := database.DB.Exec(ctx, query,
		d.ID, d.NotificationID, d.Channel, d.Status, d.MaxRetries, d.CreatedAt,
	)
	return err
}

// UpdateDeliveryStatus updates a delivery attempt
func UpdateDeliveryStatus(ctx context.Context, id string, status model.DeliveryStatus, errMsg string) error {
	now := time.Now()
	query := `
		UPDATE notification_deliveries
		SET status = $1, last_attempt = $2, error_message = $3, retry_count = retry_count + 1
		WHERE id = $4
	`
	_, err := database.DB.Exec(ctx, query, status, now, errMsg, id)
	return err
}

// GetByUser returns notifications for a user (paginated, newest first)
func GetByUser(ctx context.Context, userID string, limit, offset int) ([]model.Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, is_read, metadata, event_id, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := database.DB.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifs []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.IsRead, &n.Metadata, &n.EventID, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifs = append(notifs, n)
	}
	return notifs, rows.Err()
}

// MarkAsRead marks a notification as read
func MarkAsRead(ctx context.Context, id, userID string) error {
	query := `UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`
	_, err := database.DB.Exec(ctx, query, id, userID)
	return err
}

// MarkAllRead marks all notifications as read for a user
func MarkAllRead(ctx context.Context, userID string) error {
	query := `UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`
	_, err := database.DB.Exec(ctx, query, userID)
	return err
}

// UnreadCount returns the number of unread notifications for a user
func UnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`
	err := database.DB.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// GetPendingRetries returns deliveries that need retry
func GetPendingRetries(ctx context.Context, limit int) ([]model.NotificationDelivery, error) {
	query := `
		SELECT id, notification_id, channel, status, retry_count, max_retries, last_attempt, next_retry, error_message, created_at
		FROM notification_deliveries
		WHERE status IN ('failed', 'retrying')
		  AND retry_count < max_retries
		  AND (next_retry IS NULL OR next_retry <= NOW())
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := database.DB.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []model.NotificationDelivery
	for rows.Next() {
		var d model.NotificationDelivery
		if err := rows.Scan(&d.ID, &d.NotificationID, &d.Channel, &d.Status, &d.RetryCount, &d.MaxRetries, &d.LastAttempt, &d.NextRetry, &d.ErrorMessage, &d.CreatedAt); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// EventExists checks if an event was already processed (idempotency)
func EventExists(ctx context.Context, eventID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM notifications WHERE event_id = $1`
	err := database.DB.QueryRow(ctx, query, eventID).Scan(&count)
	return count > 0, err
}
