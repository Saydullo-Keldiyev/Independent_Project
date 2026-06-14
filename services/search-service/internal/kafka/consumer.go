package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/auction-system/search-service/internal/elastic"
	"github.com/auction-system/search-service/internal/model"
)

const (
	EventAuctionCreated = "auction.created"
	EventAuctionUpdated = "auction.updated"
	EventAuctionEnded   = "auction.ended"
	EventAuctionDeleted = "auction.deleted"
	EventBidPlaced      = "bid.placed"
)

type IndexConsumer struct {
	reader *kafka.Reader
	index  string
	log    *zap.Logger
}

func NewIndexConsumer(brokers, topics []string, groupID, index string, log *zap.Logger) *IndexConsumer {
	topic := "auction-events"
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

	return &IndexConsumer{reader: reader, index: index, log: log}
}

// Run starts consuming indexing events. Blocks until ctx cancelled.
func (c *IndexConsumer) Run(ctx context.Context) error {
	c.log.Info("search indexing consumer started")

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

		c.processMessage(ctx, msg)

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("commit error", zap.Error(err))
		}
	}
}

func (c *IndexConsumer) processMessage(ctx context.Context, msg kafka.Message) {
	eventType := string(msg.Key)

	switch eventType {
	case EventAuctionCreated, EventAuctionUpdated:
		var doc model.AuctionDocument
		if err := json.Unmarshal(msg.Value, &doc); err != nil {
			c.log.Error("unmarshal auction doc", zap.Error(err))
			return
		}
		if err := elastic.IndexDocument(ctx, c.index, doc); err != nil {
			c.log.Error("index document failed", zap.String("auction_id", doc.AuctionID), zap.Error(err))
			return
		}
		c.log.Debug("document indexed", zap.String("auction_id", doc.AuctionID), zap.String("event", eventType))

	case EventAuctionEnded:
		var payload struct {
			AuctionID string `json:"auction_id"`
		}
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			return
		}
		// Update state to "ended" instead of deleting
		doc := model.AuctionDocument{AuctionID: payload.AuctionID, State: "ended", UpdatedAt: time.Now()}
		elastic.IndexDocument(ctx, c.index, doc)

	case EventAuctionDeleted:
		var payload struct {
			AuctionID string `json:"auction_id"`
		}
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			return
		}
		elastic.DeleteDocument(ctx, c.index, payload.AuctionID)
		c.log.Debug("document deleted", zap.String("auction_id", payload.AuctionID))

	case EventBidPlaced:
		// Update bid count and current price
		var payload struct {
			AuctionID    string  `json:"auction_id"`
			Amount       float64 `json:"amount"`
		}
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			return
		}
		// Partial update via script (simplified: re-index full doc in production)
		c.log.Debug("bid placed — price update", zap.String("auction_id", payload.AuctionID))
	}
}

func (c *IndexConsumer) Close() error {
	return c.reader.Close()
}
