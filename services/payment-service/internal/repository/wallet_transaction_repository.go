package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/model"
)

// CreateWalletTransaction inserts a wallet transaction audit record within a DB transaction.
// Uses idempotency_key UNIQUE constraint to prevent duplicate processing.
func CreateWalletTransaction(ctx context.Context, tx pgx.Tx, wt model.WalletTransaction) error {
	query := `
		INSERT INTO wallet_transactions (
			id, wallet_id, transaction_id, idempotency_key, operation_type,
			amount, balance_before, balance_after, reference_type, reference_id,
			status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (idempotency_key) DO NOTHING
	`
	_, err := tx.Exec(ctx, query,
		wt.ID, wt.WalletID, wt.TransactionID, wt.IdempotencyKey, wt.OperationType,
		wt.Amount, wt.BalanceBefore, wt.BalanceAfter, wt.ReferenceType, wt.ReferenceID,
		wt.Status, wt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create wallet transaction: %w", err)
	}
	return nil
}

// GetWalletTransactionByIdempotencyKey checks if a wallet transaction already exists.
// Returns the existing transaction if found, nil otherwise.
func GetWalletTransactionByIdempotencyKey(ctx context.Context, key string) (*model.WalletTransaction, error) {
	query := `
		SELECT id, wallet_id, transaction_id, idempotency_key, operation_type,
		       amount, balance_before, balance_after, reference_type, reference_id,
		       status, created_at
		FROM wallet_transactions WHERE idempotency_key = $1
	`
	var wt model.WalletTransaction
	err := database.DB.QueryRow(ctx, query, key).Scan(
		&wt.ID, &wt.WalletID, &wt.TransactionID, &wt.IdempotencyKey, &wt.OperationType,
		&wt.Amount, &wt.BalanceBefore, &wt.BalanceAfter, &wt.ReferenceType, &wt.ReferenceID,
		&wt.Status, &wt.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &wt, nil
}

// GetWalletTransactionsByWalletID returns audit trail for a wallet (paginated, newest first).
func GetWalletTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int) ([]model.WalletTransaction, error) {
	query := `
		SELECT id, wallet_id, transaction_id, idempotency_key, operation_type,
		       amount, balance_before, balance_after, reference_type, reference_id,
		       status, created_at
		FROM wallet_transactions WHERE wallet_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := database.DB.Query(ctx, query, walletID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []model.WalletTransaction
	for rows.Next() {
		var wt model.WalletTransaction
		if err := rows.Scan(
			&wt.ID, &wt.WalletID, &wt.TransactionID, &wt.IdempotencyKey, &wt.OperationType,
			&wt.Amount, &wt.BalanceBefore, &wt.BalanceAfter, &wt.ReferenceType, &wt.ReferenceID,
			&wt.Status, &wt.CreatedAt,
		); err != nil {
			return nil, err
		}
		txns = append(txns, wt)
	}
	return txns, rows.Err()
}
