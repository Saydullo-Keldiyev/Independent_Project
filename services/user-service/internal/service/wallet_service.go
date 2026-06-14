package service

import (
	"context"
	"fmt"

	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/dto"
	"github.com/auction-system/user-service/internal/model"
	"github.com/auction-system/user-service/internal/observability"
	"github.com/auction-system/user-service/internal/repository"
)

type WalletService struct {
	wallets *repository.WalletRepository
	audit   *repository.AuditRepository
}

func NewWalletService() *WalletService {
	return &WalletService{
		wallets: repository.NewWalletRepository(),
		audit:   repository.NewAuditRepository(),
	}
}

func (s *WalletService) GetWallet(ctx context.Context, userID string) (*dto.WalletResponse, error) {
	w, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &dto.WalletResponse{
		ID: w.ID, UserID: w.UserID, Balance: w.Balance, Currency: w.Currency,
	}, nil
}

func (s *WalletService) Deposit(ctx context.Context, userID string, amount float64) (*dto.WalletResponse, error) {
	observability.WalletOperationsTotal.WithLabelValues("deposit").Inc()
	w, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := s.wallets.Deposit(ctx, tx, w.ID, amount, "manual deposit"); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, userID, "WALLET_DEPOSIT", map[string]any{"amount": amount})
	updated, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &dto.WalletResponse{
		ID: updated.ID, UserID: updated.UserID, Balance: updated.Balance, Currency: updated.Currency,
	}, nil
}

func (s *WalletService) History(ctx context.Context, userID string, limit, offset int) (*dto.WalletHistoryResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	w, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	txs, total, err := s.wallets.ListTransactions(ctx, w.ID, limit, offset)
	if err != nil {
		return nil, err
	}
	resp := &dto.WalletHistoryResponse{Total: total, Transactions: make([]dto.WalletTransactionResponse, 0, len(txs))}
	for _, t := range txs {
		resp.Transactions = append(resp.Transactions, dto.WalletTransactionResponse{
			ID: t.ID, Type: string(t.Type), Amount: t.Amount,
			BalanceBefore: t.BalanceBefore, BalanceAfter: t.BalanceAfter,
			Description: t.Description, CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return resp, nil
}

// Hold reserves balance for bidding (ACID) — used by bid-service integration
func (s *WalletService) Hold(ctx context.Context, userID string, amount float64, ref string) error {
	observability.WalletOperationsTotal.WithLabelValues("hold").Inc()
	w, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if w.Balance < amount {
		return repository.ErrInsufficientFunds
	}
	// Debit as hold type
	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.wallets.AdjustBalance(ctx, tx, w.ID, -amount, model.TxHold, fmt.Sprintf("hold:%s", ref)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReleaseHold returns held funds (auction lost / cancelled)
func (s *WalletService) ReleaseHold(ctx context.Context, userID string, amount float64, ref string) error {
	observability.WalletOperationsTotal.WithLabelValues("release").Inc()
	w, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.wallets.AdjustBalance(ctx, tx, w.ID, amount, model.TxRelease, fmt.Sprintf("release:%s", ref)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Settle finalizes a winner's held funds into a permanent charge (auction won)
func (s *WalletService) Settle(ctx context.Context, userID string, amount float64, auctionID string) error {
	observability.WalletOperationsTotal.WithLabelValues("settle").Inc()
	w, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Settle doesn't change balance (funds were already deducted at hold time)
	// It records the transaction for audit trail
	if err := s.wallets.AdjustBalance(ctx, tx, w.ID, 0, model.TxSettle, fmt.Sprintf("settle:auction:%s", auctionID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Credit adds funds to seller's wallet after auction completion
func (s *WalletService) Credit(ctx context.Context, userID string, amount float64, auctionID string) error {
	observability.WalletOperationsTotal.WithLabelValues("credit").Inc()
	w, err := s.wallets.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.wallets.AdjustBalance(ctx, tx, w.ID, amount, model.TxCredit, fmt.Sprintf("credit:auction:%s", auctionID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
