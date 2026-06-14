package model

import "time"

type Wallet struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Balance   float64   `db:"balance"`
	Currency  string    `db:"currency"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type TransactionType string

const (
	TxDeposit  TransactionType = "deposit"
	TxWithdraw TransactionType = "withdraw"
	TxHold     TransactionType = "hold"
	TxRelease  TransactionType = "release"
	TxSettle   TransactionType = "settle"
	TxCredit   TransactionType = "credit"
)

type WalletTransaction struct {
	ID          string          `db:"id"`
	WalletID    string          `db:"wallet_id"`
	Type        TransactionType `db:"type"`
	Amount      float64         `db:"amount"`
	BalanceBefore float64       `db:"balance_before"`
	BalanceAfter  float64       `db:"balance_after"`
	Description string          `db:"description"`
	CreatedAt   time.Time       `db:"created_at"`
}
