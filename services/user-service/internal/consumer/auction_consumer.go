package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/auction-system/user-service/internal/service"
)

const EventAuctionEnded = "auction.ended"

type AuctionEndedEvent struct {
	EventType string    `json:"event_type"`
	AuctionID string    `json:"auction_id"`
	SellerID  string    `json:"seller_id"`
	Title     string    `json:"title"`
	WinnerID  string    `json:"winner_id,omitempty"`
	Amount    float64   `json:"amount,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AuctionConsumer listens for auction.ended events to settle/credit wallets
type AuctionConsumer struct {
	reader    *kafka.Reader
	walletSvc *service.WalletService
	log       *zap.Logger
}

func NewAuctionConsumer(brokers []string, topic, groupID string, walletSvc *service.WalletService, log *zap.Logger) *AuctionConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0,
		StartOffset:    kafka.LastOffset,
	})

	return &AuctionConsumer{
		reader:    reader,
		walletSvc: walletSvc,
		log:       log,
	}
}

// Run starts consuming. Blocks until ctx is cancelled.
func (c *AuctionConsumer) Run(ctx context.Context) error {
	c.log.Info("auction wallet consumer started")

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.log.Error("fetch error", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		eventType := string(msg.Key)
		if eventType == EventAuctionEnded {
			if err := c.handleAuctionEnded(ctx, msg.Value); err != nil {
				c.log.Error("failed to handle auction.ended",
					zap.Int64("offset", msg.Offset),
					zap.Error(err),
				)
			}
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("commit failed", zap.Error(err))
		}
	}
}

func (c *AuctionConsumer) handleAuctionEnded(ctx context.Context, data []byte) error {
	var event AuctionEndedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if event.WinnerID == "" {
		c.log.Info("auction ended without winner, no wallet action needed",
			zap.String("auction_id", event.AuctionID),
		)
		return nil
	}

	// 1. Settle winner's held funds (mark as permanently charged)
	if err := c.walletSvc.Settle(ctx, event.WinnerID, event.Amount, event.AuctionID); err != nil {
		c.log.Error("settle winner failed",
			zap.String("winner_id", event.WinnerID),
			zap.String("auction_id", event.AuctionID),
			zap.Error(err),
		)
		// Don't return — still try to credit seller
	}

	// 2. Credit seller's wallet with the winning amount
	if err := c.walletSvc.Credit(ctx, event.SellerID, event.Amount, event.AuctionID); err != nil {
		c.log.Error("credit seller failed",
			zap.String("seller_id", event.SellerID),
			zap.String("auction_id", event.AuctionID),
			zap.Error(err),
		)
		return err
	}

	c.log.Info("auction settlement completed",
		zap.String("auction_id", event.AuctionID),
		zap.String("winner_id", event.WinnerID),
		zap.String("seller_id", event.SellerID),
		zap.Float64("amount", event.Amount),
	)

	return nil
}

func (c *AuctionConsumer) Close() error {
	return c.reader.Close()
}
