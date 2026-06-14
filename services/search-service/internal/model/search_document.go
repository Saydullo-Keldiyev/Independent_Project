package model

import "time"

// AuctionDocument is the Elasticsearch document structure.
// This is the search index projection — NOT the source of truth.
type AuctionDocument struct {
	AuctionID    string    `json:"auction_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	Tags         []string  `json:"tags,omitempty"`
	SellerID     string    `json:"seller_id"`
	SellerName   string    `json:"seller_name"`
	StartPrice   float64   `json:"start_price"`
	CurrentPrice float64   `json:"current_price"`
	TotalBids    int       `json:"total_bids"`
	State        string    `json:"state"` // active, ended, cancelled
	ImageURL     string    `json:"image_url,omitempty"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
