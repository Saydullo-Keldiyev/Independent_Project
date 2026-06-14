package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/auction-system/analytics-service/internal/cache"
	"github.com/auction-system/analytics-service/internal/repository"
)

const (
	EventBidPlaced       = "bid.placed"
	EventAuctionCreated  = "auction.created"
	EventAuctionEnded    = "auction.ended"
	EventPaymentSettled  = "payment.settled"
	EventUserRegistered  = "user.registered"
)

type Consumer struct {
	reader *kafka.Reader
	repo   *repository.AnalyticsRepository
	log    *zap.Logger
}

func New(brokers, topics []string, groupID string, repo *repository.AnalyticsRepository, log *zap.Logger) *Consumer {
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

	return &Consumer{reader: reader, repo: repo, log: log}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("analytics consumer started")

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

		c.process(ctx, msg)
		c.reader.CommitMessages(ctx, msg)
	}
}

func (c *Consumer) process(ctx context.Context, msg kafka.Message) {
	eventType := string(msg.Key)

	switch eventType {
	case EventBidPlaced:
		var payload struct {
			AuctionID string  `json:"auction_id"`
			UserID    string  `json:"user_id"`
			Amount    float64 `json:"amount"`
		}
		if json.Unmarshal(msg.Value, &payload) == nil {
			cache.IncrBidsToday()
			cache.IncrBidderActivity(payload.UserID)
			cache.IncrActiveUsers(payload.UserID)
			c.repo.IncrAuctionBids(ctx, payload.AuctionID, payload.Amount)
		}

	case EventAuctionCreated:
		var payload struct {
			AuctionID string `json:"auction_id"`
			Category  string `json:"category"`
			SellerID  string `json:"seller_id"`
		}
		if json.Unmarshal(msg.Value, &payload) == nil {
			cache.IncrCategoryActivity(payload.Category)
			c.repo.IncrDailyAuctions(ctx)
		}

	case EventPaymentSettled:
		var payload struct {
			AuctionID string  `json:"auction_id"`
			SellerID  string  `json:"seller_id"`
			Amount    float64 `json:"amount"`
		}
		if json.Unmarshal(msg.Value, &payload) == nil {
			platformFee := payload.Amount * 0.05 // 5% platform fee
			cache.IncrRevenue(platformFee)
			cache.IncrSellerRevenue(payload.SellerID, payload.Amount-platformFee)
			c.repo.RecordRevenue(ctx, payload.SellerID, payload.AuctionID, payload.Amount, platformFee)
		}

	case EventUserRegistered:
		c.repo.IncrDailyNewUsers(ctx)

	default:
		c.log.Debug("unknown event", zap.String("type", eventType))
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
