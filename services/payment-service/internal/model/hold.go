package model

import "time"

type HoldStatus string

const (
	HoldActive   HoldStatus = "active"
	HoldReleased HoldStatus = "released"
	HoldCharged  HoldStatus = "charged"
	HoldExpired  HoldStatus = "expired"
)

// BidHold represents money reserved for an active bid.
// When auction ends: winner's hold → charged, losers' holds → released.
type BidHold struct {
	ID        string     `db:"id"         json:"id"`
	AuctionID string     `db:"auction_id" json:"auction_id"`
	BidderID  string     `db:"bidder_id"  json:"bidder_id"`
	WalletID  string     `db:"wallet_id"  json:"wallet_id"`
	Amount    float64    `db:"amount"     json:"amount"`
	Status    HoldStatus `db:"status"     json:"status"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}
