package model

import "time"

type Wallet struct {
	ID               string    `db:"id"                json:"id"`
	UserID           string    `db:"user_id"           json:"user_id"`
	AvailableBalance float64   `db:"available_balance" json:"available_balance"`
	HeldBalance      float64   `db:"held_balance"      json:"held_balance"`
	Currency         string    `db:"currency"          json:"currency"`
	CreatedAt        time.Time `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"        json:"updated_at"`
}

// TotalBalance returns available + held
func (w *Wallet) TotalBalance() float64 {
	return w.AvailableBalance + w.HeldBalance
}
