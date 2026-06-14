package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/model"
)

// GetWalletByUserID returns the wallet for a user
func GetWalletByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	query := `
		SELECT id, user_id, available_balance, held_balance, currency, created_at, updated_at
		FROM wallets WHERE user_id = $1
	`
	var w model.Wallet
	err := database.DB.QueryRow(ctx, query, userID).Scan(
		&w.ID, &w.UserID, &w.AvailableBalance, &w.HeldBalance, &w.Currency, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}
	return &w, nil
}

// GetWalletForUpdate locks the wallet row for the duration of the transaction.
// MUST be called within a pgx.Tx — prevents race conditions on balance updates.
func GetWalletForUpdate(ctx context.Context, tx pgx.Tx, userID string) (*model.Wallet, error) {
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
		return nil, fmt.Errorf("wallet lock failed: %w", err)
	}
	return &w, nil
}

// UpdateBalance updates available and held balances atomically
func UpdateBalance(ctx context.Context, tx pgx.Tx, walletID string, available, held float64) error {
	query := `
		UPDATE wallets
		SET available_balance = $1, held_balance = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := tx.Exec(ctx, query, available, held, time.Now(), walletID)
	return err
}

// CreateWallet creates a new wallet for a user
func CreateWallet(ctx context.Context, w model.Wallet) error {
	query := `
		INSERT INTO wallets (id, user_id, available_balance, held_balance, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO NOTHING
	`
	_, err := database.DB.Exec(ctx, query,
		w.ID, w.UserID, w.AvailableBalance, w.HeldBalance, w.Currency, w.CreatedAt, w.UpdatedAt,
	)
	return err
}

// Deposit adds funds to available balance (no lock needed — single operation)
func Deposit(ctx context.Context, tx pgx.Tx, walletID string, amount float64) error {
	query := `
		UPDATE wallets
		SET available_balance = available_balance + $1, updated_at = $2
		WHERE id = $3
	`
	_, err := tx.Exec(ctx, query, amount, time.Now(), walletID)
	return err
}
