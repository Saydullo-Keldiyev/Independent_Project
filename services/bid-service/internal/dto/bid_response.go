package dto

import "time"

type BidResponse struct {
	ID        string    `json:"id"`
	AuctionID string    `json:"auction_id"`
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

type BidListResponse struct {
	Bids  []BidResponse `json:"bids"`
	Total int           `json:"total"`
}
