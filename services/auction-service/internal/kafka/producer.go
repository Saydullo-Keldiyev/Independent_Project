package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

var Writer *kafka.Writer

type ProducerConfig struct {
	Brokers []string
	Topic   string
}

func InitProducer(cfg ProducerConfig) {
	Writer = &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.Topic,
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           10 * time.Second,
		AllowAutoTopicCreation: true,
	}
}

func PublishEvent(eventType string, payload any) error {
	if Writer == nil {
		return fmt.Errorf("kafka writer not initialized")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return Writer.WriteMessages(ctx, kafka.Message{
		Key: []byte(eventType), Value: data, Time: time.Now(),
	})
}

func Close() { if Writer != nil { Writer.Close() } }

// ── Event types ───────────────────────────────────────────────────────────────

const (
	EventAuctionCreated = "auction.created"
	EventAuctionStarted = "auction.started"
	EventAuctionEnded   = "auction.ended"
	EventAuctionDeleted = "auction.deleted"
)

type AuctionEvent struct {
	EventType string    `json:"event_type"`
	AuctionID string    `json:"auction_id"`
	SellerID  string    `json:"seller_id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	WinnerID  string    `json:"winner_id,omitempty"`
	Amount    float64   `json:"amount,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
