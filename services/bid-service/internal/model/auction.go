package model

import "time"

type AuctionStatus string

const (
	AuctionStatusActive  AuctionStatus = "active"
	AuctionStatusEnded   AuctionStatus = "ended"
	AuctionStatusPending AuctionStatus = "pending"
)

type Auction struct {
	ID           string        `db:"id"            json:"id"`
	Title        string        `db:"title"         json:"title"`
	StartPrice   float64       `db:"start_price"   json:"start_price"`
	CurrentPrice float64       `db:"current_price" json:"current_price"`
	Status       AuctionStatus `db:"status"        json:"status"`
	SellerID     string        `db:"seller_id"     json:"seller_id"`
	WinnerID     *string       `db:"winner_id"     json:"winner_id,omitempty"`
	StartAt      time.Time     `db:"start_at"      json:"start_at"`
	EndAt        time.Time     `db:"end_at"        json:"end_at"`
	CreatedAt    time.Time     `db:"created_at"    json:"created_at"`
}
