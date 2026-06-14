package kafka

import "time"

const (
	EventBidPlaced    = "bid.placed"
	EventBidRejected  = "bid.rejected"
	EventAuctionWon   = "auction.won"
)

// BidPlacedEvent is published when a bid is successfully placed
type BidPlacedEvent struct {
	EventType string    `json:"event_type"`
	BidID     string    `json:"bid_id"`
	AuctionID string    `json:"auction_id"`
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

// BidRejectedEvent is published when a bid is rejected
type BidRejectedEvent struct {
	EventType string    `json:"event_type"`
	AuctionID string    `json:"auction_id"`
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}
