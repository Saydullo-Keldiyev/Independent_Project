package wallet

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/auction-system/payment-service/internal/model"
)

// PgRepository implements Repository using PostgreSQL via pgx.
type PgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository creates a new PostgreSQL-backed repository.
func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

// GetWalletForUpdate locks the wallet row by wallet ID using SELECT FOR UPDATE.
func (r *PgRepository) GetWalletForUpdate(ctx context.Context, tx pgx.Tx, walletID string) (*model.Wallet, error) {
	query := `
		SELECT id, user_id, available_balance, held_balance, currency, created_at, updated_at
		FROM wallets WHERE id = $1
		FOR UPDATE
	`
	var w model.Wallet
	err := tx.QueryRow(ctx, query, walletID).Scan(
		&w.ID, &w.UserID, &w.AvailableBalance, &w.HeldBalance, &w.Currency, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet lock by id failed: %w", err)
	}
	return &w, nil
}

// GetWalletByUserIDForUpdate locks the wallet row by user ID using SELECT FOR UPDATE.
func (r *PgRepository) GetWalletByUserIDForUpdate(ctx context.Context, tx pgx.Tx, userID string) (*model.Wallet, error) {
	query := `
		SELECT id, user_id, available_balance, held_balance, currency, created_at, updated_at
		FROM wallets WHERE user_id = $1
		FOR UPDATE
	`
	var w model.Wallet
	err := tx.QueryRow(ctx, query, userID).Scan(
		&w.ID, &w.UserID, &w.AvailableBalance, &w.HeldBalance, &w.Currency, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet lock by user_id failed: %w", err)
	}
	return &w, nil
}

// UpdateBalance atomically updates both available and held balances.
func (r *PgRepository) UpdateBalance(ctx context.Context, tx pgx.Tx, walletID string, available, held float64) error {
	query := `
		UPDATE wallets
		SET available_balance = $1, held_balance = $2, updated_at = $3
		WHERE id = $4
	`
	tag, err := tx.Exec(ctx, query, available, held, time.Now().UTC(), walletID)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("wallet not found: %s", walletID)
	}
	return nil
}

// CheckIdempotencyKey returns true if the idempotency key already exists in wallet_transactions.
func (r *PgRepository) CheckIdempotencyKey(ctx context.Context, key string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM wallet_transactions WHERE idempotency_key = $1)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, key).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check idempotency key: %w", err)
	}
	return exists, nil
}

// CreateWalletTransaction inserts a wallet transaction audit record.
func (r *PgRepository) CreateWalletTransaction(ctx context.Context, tx pgx.Tx, wt model.WalletTransaction) error {
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
		return fmt.Errorf("insert wallet transaction: %w", err)
	}
	return nil
}

// CreateHold inserts a bid hold record within the transaction.
func (r *PgRepository) CreateHold(ctx context.Context, tx pgx.Tx, h model.BidHold) error {
	query := `
		INSERT INTO bid_holds (id, auction_id, bidder_id, wallet_id, amount, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := tx.Exec(ctx, query,
		h.ID, h.AuctionID, h.BidderID, h.WalletID, h.Amount, h.Status, h.ExpiresAt, h.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert hold: %w", err)
	}
	return nil
}

// GetActiveHoldForUpdate locks and returns an active hold by ID.
func (r *PgRepository) GetActiveHoldForUpdate(ctx context.Context, tx pgx.Tx, holdID string) (*model.BidHold, error) {
	query := `
		SELECT id, auction_id, bidder_id, wallet_id, amount, status, expires_at, created_at
		FROM bid_holds
		WHERE id = $1 AND status = 'active'
		FOR UPDATE
	`
	var h model.BidHold
	err := tx.QueryRow(ctx, query, holdID).Scan(
		&h.ID, &h.AuctionID, &h.BidderID, &h.WalletID, &h.Amount, &h.Status, &h.ExpiresAt, &h.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("hold not found or not active: %w", err)
	}
	return &h, nil
}

// UpdateHoldStatus changes the hold status within a transaction.
func (r *PgRepository) UpdateHoldStatus(ctx context.Context, tx pgx.Tx, holdID string, status model.HoldStatus) error {
	query := `UPDATE bid_holds SET status = $1 WHERE id = $2`
	tag, err := tx.Exec(ctx, query, status, holdID)
	if err != nil {
		return fmt.Errorf("update hold status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("hold not found: %s", holdID)
	}
	return nil
}
