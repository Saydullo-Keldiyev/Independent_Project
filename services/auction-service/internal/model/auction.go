package model

import "time"

type AuctionState string

const (
	StateDraft     AuctionState = "draft"
	StateScheduled AuctionState = "scheduled"
	StateActive    AuctionState = "active"
	StateEnding    AuctionState = "ending"
	StateEnded     AuctionState = "ended"
	StateArchived  AuctionState = "archived"
)

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[AuctionState][]AuctionState{
	StateDraft:     {StateScheduled, StateActive},
	StateScheduled: {StateActive, StateDraft},
	StateActive:    {StateEnding, StateEnded},
	StateEnding:    {StateEnded},
	StateEnded:     {StateArchived},
}

// CanTransition checks if a state transition is valid
func CanTransition(from, to AuctionState) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

type Auction struct {
	ID            string       `db:"id"             json:"id"`
	SellerID      string       `db:"seller_id"      json:"seller_id"`
	Title         string       `db:"title"          json:"title"`
	Description   string       `db:"description"    json:"description"`
	CategoryID    *string      `db:"category_id"    json:"category_id,omitempty"`
	StartingPrice float64      `db:"starting_price" json:"starting_price"`
	ReservePrice  float64      `db:"reserve_price"  json:"reserve_price"`
	CurrentPrice  float64      `db:"current_price"  json:"current_price"`
	State         AuctionState `db:"state"          json:"state"`
	StartTime     time.Time    `db:"start_time"     json:"start_time"`
	EndTime       time.Time    `db:"end_time"       json:"end_time"`
	WinnerID      *string      `db:"winner_id"      json:"winner_id,omitempty"`
	TotalBids     int          `db:"total_bids"     json:"total_bids"`
	CreatedAt     time.Time    `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time    `db:"updated_at"     json:"updated_at"`
	DeletedAt     *time.Time   `db:"deleted_at"     json:"deleted_at,omitempty"`
}

type Category struct {
	ID        string    `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type AuctionImage struct {
	ID        string `db:"id"         json:"id"`
	AuctionID string `db:"auction_id" json:"auction_id"`
	ImageURL  string `db:"image_url"  json:"image_url"`
	SortOrder int    `db:"sort_order" json:"sort_order"`
}
