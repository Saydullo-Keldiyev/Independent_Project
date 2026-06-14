package model

import "time"

type TransactionType string

const (
	TxDeposit    TransactionType = "deposit"
	TxWithdraw   TransactionType = "withdraw"
	TxHold       TransactionType = "hold"
	TxRelease    TransactionType = "release"
	TxCharge     TransactionType = "charge"
	TxRefund     TransactionType = "refund"
	TxPayout     TransactionType = "payout"
)

type TransactionStatus string

const (
	TxStatusPending   TransactionStatus = "pending"
	TxStatusCompleted TransactionStatus = "completed"
	TxStatusFailed    TransactionStatus = "failed"
	TxStatusReversed  TransactionStatus = "reversed"
)

type Transaction struct {
	ID             string            `db:"id"              json:"id"`
	WalletID       string            `db:"wallet_id"       json:"wallet_id"`
	Type           TransactionType   `db:"type"            json:"type"`
	Amount         float64           `db:"amount"          json:"amount"`
	ReferenceID    *string           `db:"reference_id"    json:"reference_id,omitempty"`
	IdempotencyKey string            `db:"idempotency_key" json:"idempotency_key"`
	Status         TransactionStatus `db:"status"          json:"status"`
	Metadata       string            `db:"metadata"        json:"metadata"`
	CreatedAt      time.Time         `db:"created_at"      json:"created_at"`
}
