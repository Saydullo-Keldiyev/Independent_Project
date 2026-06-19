package model

import "time"

// OperationType defines the type of wallet operation for audit logging.
type OperationType string

const (
	OpDeposit OperationType = "deposit"
	OpHold    OperationType = "hold"
	OpRelease OperationType = "release"
	OpCharge  OperationType = "charge"
	OpCredit  OperationType = "credit"
)

// ReferenceType defines what entity the transaction references.
type ReferenceType string

const (
	RefAuction ReferenceType = "auction"
	RefBid     ReferenceType = "bid"
	RefManual  ReferenceType = "manual"
)

// WalletTransaction represents a fully audited wallet operation.
// Every balance change produces exactly one WalletTransaction record.
// balance_after = balance_before ± amount (sign depends on operation_type).
type WalletTransaction struct {
	ID             string        `db:"id"              json:"id"`
	WalletID       string        `db:"wallet_id"       json:"wallet_id"`
	TransactionID  string        `db:"transaction_id"  json:"transaction_id"`
	IdempotencyKey string        `db:"idempotency_key" json:"idempotency_key"`
	OperationType  OperationType `db:"operation_type"  json:"operation_type"`
	Amount         float64       `db:"amount"          json:"amount"`
	BalanceBefore  float64       `db:"balance_before"  json:"balance_before"`
	BalanceAfter   float64       `db:"balance_after"   json:"balance_after"`
	ReferenceType  *string       `db:"reference_type"  json:"reference_type,omitempty"`
	ReferenceID    *string       `db:"reference_id"    json:"reference_id,omitempty"`
	Status         string        `db:"status"          json:"status"`
	CreatedAt      time.Time     `db:"created_at"      json:"created_at"`
}
