package model

import "time"

type Bid struct {
	ID        string    `db:"id"         json:"id"`
	AuctionID string    `db:"auction_id" json:"auction_id"`
	UserID    string    `db:"user_id"    json:"user_id"`
	Amount    float64   `db:"amount"     json:"amount"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
