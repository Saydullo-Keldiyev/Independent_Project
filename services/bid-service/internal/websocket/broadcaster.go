package websocket

import (
	"encoding/json"
	"time"
)

// BidBroadcastMessage is the payload sent to WebSocket clients
type BidBroadcastMessage struct {
	Type      string    `json:"type"`
	AuctionID string    `json:"auction_id"`
	BidID     string    `json:"bid_id"`
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

// BroadcastNewBid sends a new bid event to all clients watching the auction
func BroadcastNewBid(auctionID, bidID, userID string, amount float64) error {
	msg := BidBroadcastMessage{
		Type:      "new_bid",
		AuctionID: auctionID,
		BidID:     bidID,
		UserID:    userID,
		Amount:    amount,
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	H.BroadcastToAuction(auctionID, payload)
	return nil
}
