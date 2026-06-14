package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/model"
	"github.com/auction-system/payment-service/internal/repository"
)

var (
	ErrInsufficientBalance = errors.New("insufficient available balance")
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrDuplicateOperation  = errors.New("duplicate operation (idempotency)")
	ErrHoldNotFound        = errors.New("hold not found")
)

type WalletService struct {
	log *zap.Logger
}

func NewWalletService(log *zap.Logger) *WalletService {
	return &WalletService{log: log}
}

// GetWallet returns the wallet for a user
func (s *WalletService) GetWallet(ctx context.Context, userID string) (*model.Wallet, error) {
	return repository.GetWalletByUserID(ctx, userID)
}

// Deposit adds funds to a user's wallet. ACID transaction.
func (s *WalletService) Deposit(ctx context.Context, userID string, amount float64, idempotencyKey string) (*model.Transaction, error) {
	// Idempotency check
	if existing, _ := repository.GetByIdempotencyKey(ctx, idempotencyKey); existing != nil {
		return existing, nil // already processed
	}

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock wallet row
	wallet, err := repository.GetWalletForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, ErrWalletNotFound
	}

	// Update balance
	if err := repository.Deposit(ctx, tx, wallet.ID, amount); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}

	// Record transaction
	txn := model.Transaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		Type:           model.TxDeposit,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		Status:         model.TxStatusCompleted,
		Metadata:       mustJSON(map[string]any{"user_id": userID}),
		CreatedAt:      time.Now().UTC(),
	}
	if err := repository.CreateTransaction(ctx, tx, txn); err != nil {
		return nil, err
	}

	// Outbox event
	outbox := model.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "wallet",
		AggregateID:   wallet.ID,
		EventType:     "wallet.deposited",
		Payload:       mustJSON(map[string]any{"wallet_id": wallet.ID, "amount": amount, "user_id": userID}),
		CreatedAt:     time.Now().UTC(),
	}
	if err := repository.InsertOutboxEvent(ctx, tx, outbox); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	s.log.Info("deposit completed",
		zap.String("user_id", userID),
		zap.Float64("amount", amount),
		zap.String("tx_id", txn.ID),
	)

	return &txn, nil
}

// HoldBalance freezes funds for a bid. ACID with SELECT FOR UPDATE.
func (s *WalletService) HoldBalance(ctx context.Context, userID, auctionID string, amount float64, idempotencyKey string) error {
	// Idempotency
	if existing, _ := repository.GetByIdempotencyKey(ctx, idempotencyKey); existing != nil {
		return nil
	}

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock wallet — prevents concurrent balance modifications
	wallet, err := repository.GetWalletForUpdate(ctx, tx, userID)
	if err != nil {
		return ErrWalletNotFound
	}

	// Check sufficient balance
	if wallet.AvailableBalance < amount {
		return ErrInsufficientBalance
	}

	// Move: available → held
	newAvailable := wallet.AvailableBalance - amount
	newHeld := wallet.HeldBalance + amount
	if err := repository.UpdateBalance(ctx, tx, wallet.ID, newAvailable, newHeld); err != nil {
		return err
	}

	// Create hold record
	hold := model.BidHold{
		ID:        uuid.New().String(),
		AuctionID: auctionID,
		BidderID:  userID,
		WalletID:  wallet.ID,
		Amount:    amount,
		Status:    model.HoldActive,
		ExpiresAt: time.Now().Add(48 * time.Hour), // 48h expiry
		CreatedAt: time.Now().UTC(),
	}
	if err := repository.CreateHold(ctx, tx, hold); err != nil {
		return err
	}

	// Transaction record
	txn := model.Transaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		Type:           model.TxHold,
		Amount:         amount,
		ReferenceID:    &hold.ID,
		IdempotencyKey: idempotencyKey,
		Status:         model.TxStatusCompleted,
		Metadata:       mustJSON(map[string]any{"auction_id": auctionID, "hold_id": hold.ID}),
		CreatedAt:      time.Now().UTC(),
	}
	if err := repository.CreateTransaction(ctx, tx, txn); err != nil {
		return err
	}

	// Outbox
	outbox := model.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "hold",
		AggregateID:   hold.ID,
		EventType:     "balance.held",
		Payload:       mustJSON(map[string]any{"hold_id": hold.ID, "auction_id": auctionID, "bidder_id": userID, "amount": amount}),
		CreatedAt:     time.Now().UTC(),
	}
	repository.InsertOutboxEvent(ctx, tx, outbox)

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.log.Info("balance held",
		zap.String("user_id", userID),
		zap.String("auction_id", auctionID),
		zap.Float64("amount", amount),
	)

	return nil
}

// ReleaseHold refunds a hold back to available balance (loser refund).
func (s *WalletService) ReleaseHold(ctx context.Context, holdID string, idempotencyKey string) error {
	if existing, _ := repository.GetByIdempotencyKey(ctx, idempotencyKey); existing != nil {
		return nil
	}

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get hold details
	hold, err := repository.GetActiveHold(ctx, "", "") // need to get by ID
	_ = hold
	// Simplified: use direct query
	var h model.BidHold
	err = tx.QueryRow(ctx,
		`SELECT id, auction_id, bidder_id, wallet_id, amount, status FROM bid_holds WHERE id = $1 AND status = 'active' FOR UPDATE`,
		holdID,
	).Scan(&h.ID, &h.AuctionID, &h.BidderID, &h.WalletID, &h.Amount, &h.Status)
	if err != nil {
		return ErrHoldNotFound
	}

	// Lock wallet
	wallet, err := repository.GetWalletForUpdate(ctx, tx, h.BidderID)
	if err != nil {
		return err
	}

	// Move: held → available
	newAvailable := wallet.AvailableBalance + h.Amount
	newHeld := wallet.HeldBalance - h.Amount
	if err := repository.UpdateBalance(ctx, tx, wallet.ID, newAvailable, newHeld); err != nil {
		return err
	}

	// Update hold status
	if err := repository.UpdateHoldStatus(ctx, tx, holdID, model.HoldReleased); err != nil {
		return err
	}

	// Transaction record
	txn := model.Transaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		Type:           model.TxRelease,
		Amount:         h.Amount,
		ReferenceID:    &holdID,
		IdempotencyKey: idempotencyKey,
		Status:         model.TxStatusCompleted,
		Metadata:       mustJSON(map[string]any{"hold_id": holdID, "auction_id": h.AuctionID}),
		CreatedAt:      time.Now().UTC(),
	}
	repository.CreateTransaction(ctx, tx, txn)

	// Outbox
	outbox := model.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "hold",
		AggregateID:   holdID,
		EventType:     "balance.refunded",
		Payload:       mustJSON(map[string]any{"hold_id": holdID, "bidder_id": h.BidderID, "amount": h.Amount}),
		CreatedAt:     time.Now().UTC(),
	}
	repository.InsertOutboxEvent(ctx, tx, outbox)

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.log.Info("hold released (refund)",
		zap.String("hold_id", holdID),
		zap.String("bidder_id", h.BidderID),
		zap.Float64("amount", h.Amount),
	)

	return nil
}

// SettleAuction charges the winner and refunds all losers. Called when auction ends.
func (s *WalletService) SettleAuction(ctx context.Context, auctionID, winnerID string) error {
	holds, err := repository.GetActiveHoldsByAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("get holds: %w", err)
	}

	for _, hold := range holds {
		if hold.BidderID == winnerID {
			// Winner: charge (held → gone)
			if err := s.chargeHold(ctx, hold); err != nil {
				s.log.Error("charge winner failed", zap.String("hold_id", hold.ID), zap.Error(err))
				return err
			}
		} else {
			// Loser: refund (held → available)
			key := fmt.Sprintf("refund:%s:%s", auctionID, hold.BidderID)
			if err := s.ReleaseHold(ctx, hold.ID, key); err != nil {
				s.log.Error("refund loser failed", zap.String("hold_id", hold.ID), zap.Error(err))
				// Continue refunding others
			}
		}
	}

	s.log.Info("auction settled",
		zap.String("auction_id", auctionID),
		zap.String("winner_id", winnerID),
		zap.Int("total_holds", len(holds)),
	)

	return nil
}

func (s *WalletService) chargeHold(ctx context.Context, hold model.BidHold) error {
	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock wallet
	wallet, err := repository.GetWalletForUpdate(ctx, tx, hold.BidderID)
	if err != nil {
		return err
	}

	// Deduct from held balance (money leaves the system → goes to seller)
	newHeld := wallet.HeldBalance - hold.Amount
	if err := repository.UpdateBalance(ctx, tx, wallet.ID, wallet.AvailableBalance, newHeld); err != nil {
		return err
	}

	// Mark hold as charged
	if err := repository.UpdateHoldStatus(ctx, tx, hold.ID, model.HoldCharged); err != nil {
		return err
	}

	// Transaction
	txn := model.Transaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		Type:           model.TxCharge,
		Amount:         hold.Amount,
		ReferenceID:    &hold.ID,
		IdempotencyKey: fmt.Sprintf("charge:%s:%s", hold.AuctionID, hold.BidderID),
		Status:         model.TxStatusCompleted,
		Metadata:       mustJSON(map[string]any{"auction_id": hold.AuctionID, "hold_id": hold.ID}),
		CreatedAt:      time.Now().UTC(),
	}
	repository.CreateTransaction(ctx, tx, txn)

	// Outbox
	outbox := model.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "payment",
		AggregateID:   hold.ID,
		EventType:     "payment.settled",
		Payload:       mustJSON(map[string]any{"auction_id": hold.AuctionID, "winner_id": hold.BidderID, "amount": hold.Amount}),
		CreatedAt:     time.Now().UTC(),
	}
	repository.InsertOutboxEvent(ctx, tx, outbox)

	return tx.Commit(ctx)
}

// GetTransactions returns transaction history for a wallet
func (s *WalletService) GetTransactions(ctx context.Context, walletID string, limit, offset int) ([]model.Transaction, error) {
	return repository.GetByWalletID(ctx, walletID, limit, offset)
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
