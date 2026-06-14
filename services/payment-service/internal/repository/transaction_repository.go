package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/model"
)

// CreateTransaction inserts a transaction record within a DB transaction.
// Uses idempotency_key UNIQUE constraint to prevent duplicates.
func CreateTransaction(ctx context.Context, tx pgx.Tx, t model.Transaction) error {
	query := `
		INSERT INTO transactions (id, wallet_id, type, amount, reference_id, idempotency_key, status, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (idempotency_key) DO NOTHING
	`
	_, err := tx.Exec(ctx, query,
		t.ID, t.WalletID, t.Type, t.Amount, t.ReferenceID, t.IdempotencyKey, t.Status, t.Metadata, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	return nil
}

// GetByIdempotencyKey checks if a transaction already exists (idempotency check)
func GetByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error) {
	query := `
		SELECT id, wallet_id, type, amount, reference_id, idempotency_key, status, metadata, created_at
		FROM transactions WHERE idempotency_key = $1
	`
	var t model.Transaction
	err := database.DB.QueryRow(ctx, query, key).Scan(
		&t.ID, &t.WalletID, &t.Type, &t.Amount, &t.ReferenceID, &t.IdempotencyKey, &t.Status, &t.Metadata, &t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetByWalletID returns transactions for a wallet (paginated)
func GetByWalletID(ctx context.Context, walletID string, limit, offset int) ([]model.Transaction, error) {
	query := `
		SELECT id, wallet_id, type, amount, reference_id, idempotency_key, status, metadata, created_at
		FROM transactions WHERE wallet_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := database.DB.Query(ctx, query, walletID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []model.Transaction
	for rows.Next() {
		var t model.Transaction
		if err := rows.Scan(&t.ID, &t.WalletID, &t.Type, &t.Amount, &t.ReferenceID, &t.IdempotencyKey, &t.Status, &t.Metadata, &t.CreatedAt); err != nil {
			return nil, err
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}
