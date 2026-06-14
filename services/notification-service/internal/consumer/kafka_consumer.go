package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// ── Event types ───────────────────────────────────────────────────────────────

const (
	EventBidPlaced     = "bid.placed"
	EventOutbid        = "bid.outbid"
	EventAuctionWon    = "auction.won"
	EventAuctionEnded  = "auction.ended"
	EventPaymentSuccess = "payment.success"
)

// ── Event structs ─────────────────────────────────────────────────────────────

type BidPlacedEvent struct {
	EventType string    `json:"event_type"`
	BidID     string    `json:"bid_id"`
	AuctionID string    `json:"auction_id"`
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

type OutbidEvent struct {
	EventType      string  `json:"event_type"`
	AuctionID      string  `json:"auction_id"`
	OutbidUserID   string  `json:"outbid_user_id"`
	OutbidUserEmail string `json:"outbid_user_email"`
	NewAmount      float64 `json:"new_amount"`
	NewBidderID    string  `json:"new_bidder_id"`
}

type AuctionWonEvent struct {
	EventType   string    `json:"event_type"`
	AuctionID   string    `json:"auction_id"`
	WinnerID    string    `json:"winner_id"`
	WinnerEmail string    `json:"winner_email"`
	Amount      float64   `json:"amount"`
	Timestamp   time.Time `json:"timestamp"`
}

type AuctionEndedEvent struct {
	EventType string    `json:"event_type"`
	AuctionID string    `json:"auction_id"`
	SellerID  string    `json:"seller_id"`
	Title     string    `json:"title"`
	WinnerID  string    `json:"winner_id,omitempty"`
	Amount    float64   `json:"amount,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type PaymentSuccessEvent struct {
	EventType string  `json:"event_type"`
	AuctionID string  `json:"auction_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
}

// ── Handler interface ─────────────────────────────────────────────────────────

type NotificationHandler interface {
	HandleBidPlaced(ctx context.Context, event BidPlacedEvent) error
	HandleOutbid(ctx context.Context, event OutbidEvent) error
	HandleAuctionWon(ctx context.Context, event AuctionWonEvent) error
	HandleAuctionEnded(ctx context.Context, event AuctionEndedEvent) error
}

// ── Dead Letter Queue interface ───────────────────────────────────────────────

type DLQ interface {
	Send(ctx context.Context, key, value []byte, reason string) error
}

// ── Consumer ──────────────────────────────────────────────────────────────────

type Consumer struct {
	reader  *kafka.Reader
	handler NotificationHandler
	dlq     DLQ
	log     *zap.Logger
}

func New(brokers []string, topics []string, groupID string, handler NotificationHandler, dlq DLQ, log *zap.Logger) *Consumer {
	topic := "bid-events"
	if len(topics) > 0 && topics[0] != "" {
		topic = topics[0]
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0,
		StartOffset:    kafka.LastOffset,
	})

	return &Consumer{
		reader:  reader,
		handler: handler,
		dlq:     dlq,
		log:     log,
	}
}

// Run starts consuming. Blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("kafka consumer started")

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.log.Info("kafka consumer stopped")
				return nil
			}
			c.log.Error("fetch error", zap.Error(err))
			time.Sleep(time.Second) // backoff on fetch errors
			continue
		}

		if err := c.dispatch(ctx, msg); err != nil {
			c.log.Error("dispatch failed",
				zap.String("key", string(msg.Key)),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)

			// Send to DLQ after failure
			if c.dlq != nil {
				c.dlq.Send(ctx, msg.Key, msg.Value, err.Error())
			}
		}

		// Always commit — failed messages go to DLQ
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("commit failed", zap.Error(err))
		}
	}
}

func (c *Consumer) dispatch(ctx context.Context, msg kafka.Message) error {
	eventType := string(msg.Key)

	c.log.Debug("processing event",
		zap.String("type", eventType),
		zap.Int64("offset", msg.Offset),
		zap.Int("partition", msg.Partition),
	)

	switch eventType {
	case EventBidPlaced:
		var event BidPlacedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return fmt.Errorf("unmarshal BidPlacedEvent: %w", err)
		}
		return c.handler.HandleBidPlaced(ctx, event)

	case EventOutbid:
		var event OutbidEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return fmt.Errorf("unmarshal OutbidEvent: %w", err)
		}
		return c.handler.HandleOutbid(ctx, event)

	case EventAuctionWon:
		var event AuctionWonEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return fmt.Errorf("unmarshal AuctionWonEvent: %w", err)
		}
		return c.handler.HandleAuctionWon(ctx, event)

	case EventAuctionEnded:
		var event AuctionEndedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return fmt.Errorf("unmarshal AuctionEndedEvent: %w", err)
		}
		return c.handler.HandleAuctionEnded(ctx, event)

	default:
		c.log.Debug("unknown event type, skipping", zap.String("type", eventType))
		return nil
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
