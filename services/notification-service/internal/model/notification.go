package model

import "time"

type NotificationType string

const (
	TypeBidPlaced      NotificationType = "bid_placed"
	TypeOutbid         NotificationType = "outbid"
	TypeAuctionStarted NotificationType = "auction_started"
	TypeAuctionEnded   NotificationType = "auction_ended"
	TypeAuctionWon     NotificationType = "auction_won"
	TypePaymentSuccess NotificationType = "payment_success"
)

type DeliveryChannel string

const (
	ChannelWebSocket DeliveryChannel = "websocket"
	ChannelEmail     DeliveryChannel = "email"
	ChannelPush      DeliveryChannel = "push"
)

type DeliveryStatus string

const (
	StatusPending   DeliveryStatus = "pending"
	StatusSent      DeliveryStatus = "sent"
	StatusFailed    DeliveryStatus = "failed"
	StatusRetrying  DeliveryStatus = "retrying"
	StatusDeadLetter DeliveryStatus = "dead_letter"
)

// Notification is the core entity — persisted in DB
type Notification struct {
	ID        string           `db:"id"         json:"id"`
	UserID    string           `db:"user_id"    json:"user_id"`
	Type      NotificationType `db:"type"       json:"type"`
	Title     string           `db:"title"      json:"title"`
	Message   string           `db:"message"    json:"message"`
	IsRead    bool             `db:"is_read"    json:"is_read"`
	Metadata  string           `db:"metadata"   json:"metadata"` // JSON
	EventID   string           `db:"event_id"   json:"event_id"` // idempotency key
	CreatedAt time.Time        `db:"created_at" json:"created_at"`
}

// NotificationDelivery tracks delivery attempts per channel
type NotificationDelivery struct {
	ID             string          `db:"id"`
	NotificationID string          `db:"notification_id"`
	Channel        DeliveryChannel `db:"channel"`
	Status         DeliveryStatus  `db:"status"`
	RetryCount     int             `db:"retry_count"`
	MaxRetries     int             `db:"max_retries"`
	LastAttempt    *time.Time      `db:"last_attempt"`
	NextRetry      *time.Time      `db:"next_retry"`
	ErrorMessage   string          `db:"error_message"`
	CreatedAt      time.Time       `db:"created_at"`
}
