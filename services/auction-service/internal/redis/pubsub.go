package redis

import (
	"context"
	"encoding/json"

	"github.com/auction-system/auction-service/internal/model"
)

const AuctionEventsChannel = "auction:events"

type AuctionEventMessage struct {
	Type      string             `json:"type"`
	AuctionID string             `json:"auction_id"`
	State     model.AuctionState `json:"state"`
	Payload   map[string]any     `json:"payload,omitempty"`
}

func PublishAuctionEvent(ctx context.Context, msg AuctionEventMessage) error {
	if Client == nil {
		return nil
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return Client.Publish(ctx, AuctionEventsChannel, data).Err()
}
