package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/model"
)

var ErrWalletNotFound = errors.New("wallet not found")
var ErrInsufficientFunds = errors.New("insufficient balance")

type WalletRepository struct{}

func NewWalletRepository() *WalletRepository { return &WalletRepository{} }

func (r *WalletRepository) Create(ctx context.Context, tx pgx.Tx, userID string) (*model.Wallet, error) {
	w := &model.Wallet{
		ID:        uuid.NewString(),
		UserID:    userID,
		Balance:   0,
		Currency:  "USD",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO wallets (id, user_id, balance, currency, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		w.ID, w.UserID, w.Balance, w.Currency, w.CreatedAt, w.UpdatedAt)
	return w, err
}

func (r *WalletRepository) FindByUserID(ctx context.Context, userID string) (*model.Wallet, error) {
	row := database.DB.QueryRow(ctx, `
		SELECT id, user_id, balance, currency, created_at, updated_at
		FROM wallets WHERE user_id = $1`, userID)
	var w model.Wallet
	err := row.Scan(&w.ID, &w.UserID, &w.Balance, &w.Currency, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWalletNotFound
	}
	return &w, err
}

func (r *WalletRepository) Deposit(ctx context.Context, tx pgx.Tx, walletID string, amount float64, desc string) (*model.WalletTransaction, error) {
	var w model.Wallet
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, balance, currency, created_at, updated_at
		FROM wallets WHERE id = $1 FOR UPDATE`, walletID).Scan(
		&w.ID, &w.UserID, &w.Balance, &w.Currency, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}

	before := w.Balance
	after := before + amount
	_, err = tx.Exec(ctx, `UPDATE wallets SET balance = $2, updated_at = NOW() WHERE id = $1`, walletID, after)
	if err != nil {
		return nil, err
	}

	txn := &model.WalletTransaction{
		ID:            uuid.NewString(),
		WalletID:      walletID,
		Type:          model.TxDeposit,
		Amount:        amount,
		BalanceBefore: before,
		BalanceAfter:  after,
		Description:   desc,
		CreatedAt:     time.Now(),
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO wallet_transactions (id, wallet_id, type, amount, balance_before, balance_after, description, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		txn.ID, txn.WalletID, txn.Type, txn.Amount, txn.BalanceBefore, txn.BalanceAfter, txn.Description, txn.CreatedAt)
	return txn, err
}

func (r *WalletRepository) ListTransactions(ctx context.Context, walletID string, limit, offset int) ([]model.WalletTransaction, int, error) {
	var total int
	if err := database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1`, walletID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := database.DB.Query(ctx, `
		SELECT id, wallet_id, type, amount, balance_before, balance_after, description, created_at
		FROM wallet_transactions WHERE wallet_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		walletID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []model.WalletTransaction
	for rows.Next() {
		var t model.WalletTransaction
		if err := rows.Scan(&t.ID, &t.WalletID, &t.Type, &t.Amount, &t.BalanceBefore, &t.BalanceAfter, &t.Description, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, t)
	}
	return list, total, rows.Err()
}

func (r *WalletRepository) AdjustBalance(ctx context.Context, tx pgx.Tx, walletID string, delta float64, txType model.TransactionType, desc string) error {
	var w model.Wallet
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, balance, currency, created_at, updated_at
		FROM wallets WHERE id = $1 FOR UPDATE`, walletID).Scan(
		&w.ID, &w.UserID, &w.Balance, &w.Currency, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWalletNotFound
	}
	if err != nil {
		return err
	}
	after := w.Balance + delta
	if after < 0 {
		return ErrInsufficientFunds
	}
	_, err = tx.Exec(ctx, `UPDATE wallets SET balance = $2, updated_at = NOW() WHERE id = $1`, walletID, after)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO wallet_transactions (id, wallet_id, type, amount, balance_before, balance_after, description, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uuid.NewString(), walletID, txType, delta, w.Balance, after, desc, time.Now())
	return err
}
