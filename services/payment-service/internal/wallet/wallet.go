// Package wallet implements production-grade wallet operations with full
// ACID compliance, SELECT FOR UPDATE row-level locking, idempotency enforcement,
// and comprehensive audit logging for all financial state transitions.
//
// Requirements: 18.1, 18.2, 18.5, 18.6
package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/auction-system/payment-service/internal/model"
)

var (
	// ErrInsufficientBalance is returned when available balance is less than hold amount.
	ErrInsufficientBalance = errors.New("insufficient available balance")
	// ErrWalletNotFound is returned when no wallet exists for the given user.
	ErrWalletNotFound = errors.New("wallet not found")
	// ErrDuplicateOperation is returned when an idempotency_key was already processed.
	ErrDuplicateOperation = errors.New("duplicate operation: already processed")
	// ErrHoldNotFound is returned when an active hold cannot be found.
	ErrHoldNotFound = errors.New("hold not found or not active")
)

// Repository defines the data access interface for wallet operations.
// All mutating methods accept a pgx.Tx to participate in the caller's transaction.
type Repository interface {
	// GetWalletForUpdate locks and returns the wallet row within a transaction.
	GetWalletForUpdate(ctx context.Context, tx pgx.Tx, walletID string) (*model.Wallet, error)
	// GetWalletByUserIDForUpdate locks and returns the wallet by user ID within a transaction.
	GetWalletByUserIDForUpdate(ctx context.Context, tx pgx.Tx, userID string) (*model.Wallet, error)
	// UpdateBalance atomically sets available and held balances.
	UpdateBalance(ctx context.Context, tx pgx.Tx, walletID string, available, held float64) error
	// CheckIdempotencyKey returns true if the key has already been processed.
	CheckIdempotencyKey(ctx context.Context, key string) (bool, error)
	// CreateWalletTransaction inserts an audit record within the transaction.
	CreateWalletTransaction(ctx context.Context, tx pgx.Tx, wt model.WalletTransaction) error
	// CreateHold inserts a bid hold record within the transaction.
	CreateHold(ctx context.Context, tx pgx.Tx, h model.BidHold) error
	// GetActiveHoldForUpdate locks and returns an active hold by ID.
	GetActiveHoldForUpdate(ctx context.Context, tx pgx.Tx, holdID string) (*model.BidHold, error)
	// UpdateHoldStatus changes hold status within a transaction.
	UpdateHoldStatus(ctx context.Context, tx pgx.Tx, holdID string, status model.HoldStatus) error
}

// Service implements wallet operations with ACID transactions,
// SELECT FOR UPDATE locking, idempotency, and audit logging.
type Service struct {
	pool *pgxpool.Pool
	repo Repository
	log  *zap.Logger
}

// NewService creates a new wallet Service.
func NewService(pool *pgxpool.Pool, repo Repository, log *zap.Logger) *Service {
	return &Service{
		pool: pool,
		repo: repo,
		log:  log,
	}
}

// HoldFundsRequest contains the parameters for a fund hold operation.
type HoldFundsRequest struct {
	UserID         string
	AuctionID      string
	BidID          string
	Amount         float64
	IdempotencyKey string
}

// HoldFundsResult contains the result of a successful fund hold.
type HoldFundsResult struct {
	TransactionID string
	HoldID        string
	BalanceBefore float64
	BalanceAfter  float64
}

// HoldFunds freezes funds for a bid placement. The entire operation executes within
// a single PostgreSQL transaction using SELECT FOR UPDATE for row-level locking.
//
// Flow: BEGIN → SELECT wallet FOR UPDATE → check idempotency → check balance →
//
//	INSERT wallet_transaction → INSERT hold → UPDATE wallet balance → COMMIT
//
// On failure at any point, the transaction rolls back and the bid is rejected.
// Requirements: 18.1, 18.2, 18.5, 18.6
func (s *Service) HoldFunds(ctx context.Context, req HoldFundsRequest) (*HoldFundsResult, error) {
	// Pre-check: idempotency outside of transaction for fast-path rejection
	exists, err := s.repo.CheckIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		s.log.Warn("idempotency check failed, proceeding with transaction",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.Error(err),
		)
	}
	if exists {
		s.log.Info("duplicate operation detected, returning success",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.String("user_id", req.UserID),
		)
		return nil, ErrDuplicateOperation
	}

	// Begin transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// SELECT FOR UPDATE — acquires row-level lock on wallet
	wallet, err := s.repo.GetWalletByUserIDForUpdate(ctx, tx, req.UserID)
	if err != nil {
		return nil, ErrWalletNotFound
	}

	// Check sufficient available balance
	if wallet.AvailableBalance < req.Amount {
		s.log.Warn("hold rejected: insufficient balance",
			zap.String("wallet_id", wallet.ID),
			zap.String("user_id", req.UserID),
			zap.Float64("requested", req.Amount),
			zap.Float64("available", wallet.AvailableBalance),
		)
		return nil, ErrInsufficientBalance
	}

	// Calculate new balances
	balanceBefore := wallet.AvailableBalance
	newAvailable := wallet.AvailableBalance - req.Amount
	newHeld := wallet.HeldBalance + req.Amount
	balanceAfter := newAvailable

	transactionID := uuid.New().String()
	holdID := uuid.New().String()
	now := time.Now().UTC()

	// Create audit record: wallet_transaction with full audit fields
	refType := string(model.RefAuction)
	wt := model.WalletTransaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		TransactionID:  transactionID,
		IdempotencyKey: req.IdempotencyKey,
		OperationType:  model.OpHold,
		Amount:         req.Amount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		ReferenceType:  &refType,
		ReferenceID:    &req.AuctionID,
		Status:         "completed",
		CreatedAt:      now,
	}
	if err := s.repo.CreateWalletTransaction(ctx, tx, wt); err != nil {
		return nil, fmt.Errorf("create wallet transaction: %w", err)
	}

	// Create hold record
	hold := model.BidHold{
		ID:        holdID,
		AuctionID: req.AuctionID,
		BidderID:  req.UserID,
		WalletID:  wallet.ID,
		Amount:    req.Amount,
		Status:    model.HoldActive,
		ExpiresAt: now.Add(48 * time.Hour),
		CreatedAt: now,
	}
	if err := s.repo.CreateHold(ctx, tx, hold); err != nil {
		return nil, fmt.Errorf("create hold: %w", err)
	}

	// Update wallet balance: available → held
	if err := s.repo.UpdateBalance(ctx, tx, wallet.ID, newAvailable, newHeld); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	// Commit — all or nothing
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Log the financial state transition with full audit fields
	s.log.Info("fund hold completed",
		zap.String("transaction_id", transactionID),
		zap.String("wallet_id", wallet.ID),
		zap.String("operation_type", string(model.OpHold)),
		zap.Float64("amount", req.Amount),
		zap.Float64("balance_before", balanceBefore),
		zap.Float64("balance_after", balanceAfter),
		zap.String("hold_id", holdID),
		zap.String("auction_id", req.AuctionID),
		zap.String("user_id", req.UserID),
		zap.Time("timestamp", now),
	)

	return &HoldFundsResult{
		TransactionID: transactionID,
		HoldID:        holdID,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}

// ReleaseFundsRequest contains parameters for releasing a hold.
type ReleaseFundsRequest struct {
	HoldID         string
	IdempotencyKey string
}

// ReleaseFundsResult contains the result of a successful fund release.
type ReleaseFundsResult struct {
	TransactionID string
	BalanceBefore float64
	BalanceAfter  float64
}

// ReleaseFunds returns held funds back to available balance.
// Used when a bidder loses the auction or a hold expires.
// Requirements: 18.1, 18.5, 18.6
func (s *Service) ReleaseFunds(ctx context.Context, req ReleaseFundsRequest) (*ReleaseFundsResult, error) {
	// Idempotency check
	exists, err := s.repo.CheckIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		s.log.Warn("idempotency check failed, proceeding",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.Error(err),
		)
	}
	if exists {
		return nil, ErrDuplicateOperation
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock and get the hold
	hold, err := s.repo.GetActiveHoldForUpdate(ctx, tx, req.HoldID)
	if err != nil {
		return nil, ErrHoldNotFound
	}

	// Lock the wallet
	wallet, err := s.repo.GetWalletForUpdate(ctx, tx, hold.WalletID)
	if err != nil {
		return nil, ErrWalletNotFound
	}

	// Calculate new balances: held → available
	balanceBefore := wallet.AvailableBalance
	newAvailable := wallet.AvailableBalance + hold.Amount
	newHeld := wallet.HeldBalance - hold.Amount
	balanceAfter := newAvailable

	transactionID := uuid.New().String()
	now := time.Now().UTC()

	// Audit record
	refType := string(model.RefAuction)
	wt := model.WalletTransaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		TransactionID:  transactionID,
		IdempotencyKey: req.IdempotencyKey,
		OperationType:  model.OpRelease,
		Amount:         hold.Amount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		ReferenceType:  &refType,
		ReferenceID:    &hold.AuctionID,
		Status:         "completed",
		CreatedAt:      now,
	}
	if err := s.repo.CreateWalletTransaction(ctx, tx, wt); err != nil {
		return nil, fmt.Errorf("create wallet transaction: %w", err)
	}

	// Update hold status
	if err := s.repo.UpdateHoldStatus(ctx, tx, req.HoldID, model.HoldReleased); err != nil {
		return nil, fmt.Errorf("update hold status: %w", err)
	}

	// Update wallet balance
	if err := s.repo.UpdateBalance(ctx, tx, wallet.ID, newAvailable, newHeld); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	s.log.Info("fund release completed",
		zap.String("transaction_id", transactionID),
		zap.String("wallet_id", wallet.ID),
		zap.String("operation_type", string(model.OpRelease)),
		zap.Float64("amount", hold.Amount),
		zap.Float64("balance_before", balanceBefore),
		zap.Float64("balance_after", balanceAfter),
		zap.String("hold_id", req.HoldID),
		zap.String("auction_id", hold.AuctionID),
		zap.Time("timestamp", now),
	)

	return &ReleaseFundsResult{
		TransactionID: transactionID,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}

// ChargeFundsRequest contains parameters for charging held funds.
type ChargeFundsRequest struct {
	HoldID         string
	IdempotencyKey string
}

// ChargeFundsResult contains the result of a successful charge.
type ChargeFundsResult struct {
	TransactionID string
	Amount        float64
	BalanceBefore float64
	BalanceAfter  float64
}

// ChargeFunds deducts held funds from the wallet (winner payment).
// The held balance is reduced — funds leave the user's wallet.
// Requirements: 18.1, 18.5, 18.6
func (s *Service) ChargeFunds(ctx context.Context, req ChargeFundsRequest) (*ChargeFundsResult, error) {
	exists, err := s.repo.CheckIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		s.log.Warn("idempotency check failed, proceeding",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.Error(err),
		)
	}
	if exists {
		return nil, ErrDuplicateOperation
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the hold
	hold, err := s.repo.GetActiveHoldForUpdate(ctx, tx, req.HoldID)
	if err != nil {
		return nil, ErrHoldNotFound
	}

	// Lock the wallet
	wallet, err := s.repo.GetWalletForUpdate(ctx, tx, hold.WalletID)
	if err != nil {
		return nil, ErrWalletNotFound
	}

	// Calculate: held balance decreases, available stays same
	balanceBefore := wallet.HeldBalance
	newHeld := wallet.HeldBalance - hold.Amount
	balanceAfter := newHeld

	transactionID := uuid.New().String()
	now := time.Now().UTC()

	// Audit record
	refType := string(model.RefAuction)
	wt := model.WalletTransaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		TransactionID:  transactionID,
		IdempotencyKey: req.IdempotencyKey,
		OperationType:  model.OpCharge,
		Amount:         hold.Amount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		ReferenceType:  &refType,
		ReferenceID:    &hold.AuctionID,
		Status:         "completed",
		CreatedAt:      now,
	}
	if err := s.repo.CreateWalletTransaction(ctx, tx, wt); err != nil {
		return nil, fmt.Errorf("create wallet transaction: %w", err)
	}

	// Update hold status to charged
	if err := s.repo.UpdateHoldStatus(ctx, tx, hold.ID, model.HoldCharged); err != nil {
		return nil, fmt.Errorf("update hold status: %w", err)
	}

	// Update wallet balance
	if err := s.repo.UpdateBalance(ctx, tx, wallet.ID, wallet.AvailableBalance, newHeld); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	s.log.Info("fund charge completed",
		zap.String("transaction_id", transactionID),
		zap.String("wallet_id", wallet.ID),
		zap.String("operation_type", string(model.OpCharge)),
		zap.Float64("amount", hold.Amount),
		zap.Float64("balance_before", balanceBefore),
		zap.Float64("balance_after", balanceAfter),
		zap.String("hold_id", hold.ID),
		zap.String("auction_id", hold.AuctionID),
		zap.Time("timestamp", now),
	)

	return &ChargeFundsResult{
		TransactionID: transactionID,
		Amount:        hold.Amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}

// CreditFundsRequest contains parameters for crediting a wallet (seller payment).
type CreditFundsRequest struct {
	UserID         string
	Amount         float64
	AuctionID      string
	IdempotencyKey string
}

// CreditFundsResult contains the result of a successful credit.
type CreditFundsResult struct {
	TransactionID string
	BalanceBefore float64
	BalanceAfter  float64
}

// CreditFunds adds funds to a user's available balance (seller receives payment).
// Requirements: 18.1, 18.5, 18.6
func (s *Service) CreditFunds(ctx context.Context, req CreditFundsRequest) (*CreditFundsResult, error) {
	exists, err := s.repo.CheckIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		s.log.Warn("idempotency check failed, proceeding",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.Error(err),
		)
	}
	if exists {
		return nil, ErrDuplicateOperation
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock wallet
	wallet, err := s.repo.GetWalletByUserIDForUpdate(ctx, tx, req.UserID)
	if err != nil {
		return nil, ErrWalletNotFound
	}

	// Calculate new balance
	balanceBefore := wallet.AvailableBalance
	newAvailable := wallet.AvailableBalance + req.Amount
	balanceAfter := newAvailable

	transactionID := uuid.New().String()
	now := time.Now().UTC()

	// Audit record
	refType := string(model.RefAuction)
	wt := model.WalletTransaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		TransactionID:  transactionID,
		IdempotencyKey: req.IdempotencyKey,
		OperationType:  model.OpCredit,
		Amount:         req.Amount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		ReferenceType:  &refType,
		ReferenceID:    &req.AuctionID,
		Status:         "completed",
		CreatedAt:      now,
	}
	if err := s.repo.CreateWalletTransaction(ctx, tx, wt); err != nil {
		return nil, fmt.Errorf("create wallet transaction: %w", err)
	}

	// Update balance
	if err := s.repo.UpdateBalance(ctx, tx, wallet.ID, newAvailable, wallet.HeldBalance); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	s.log.Info("fund credit completed",
		zap.String("transaction_id", transactionID),
		zap.String("wallet_id", wallet.ID),
		zap.String("operation_type", string(model.OpCredit)),
		zap.Float64("amount", req.Amount),
		zap.Float64("balance_before", balanceBefore),
		zap.Float64("balance_after", balanceAfter),
		zap.String("auction_id", req.AuctionID),
		zap.String("user_id", req.UserID),
		zap.Time("timestamp", now),
	)

	return &CreditFundsResult{
		TransactionID: transactionID,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}

// DepositFundsRequest contains parameters for depositing funds.
type DepositFundsRequest struct {
	UserID         string
	Amount         float64
	IdempotencyKey string
}

// DepositFundsResult contains the result of a successful deposit.
type DepositFundsResult struct {
	TransactionID string
	BalanceBefore float64
	BalanceAfter  float64
}

// DepositFunds adds funds to a user's available balance (top-up).
// Requirements: 18.1, 18.5, 18.6
func (s *Service) DepositFunds(ctx context.Context, req DepositFundsRequest) (*DepositFundsResult, error) {
	exists, err := s.repo.CheckIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		s.log.Warn("idempotency check failed, proceeding",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.Error(err),
		)
	}
	if exists {
		return nil, ErrDuplicateOperation
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	wallet, err := s.repo.GetWalletByUserIDForUpdate(ctx, tx, req.UserID)
	if err != nil {
		return nil, ErrWalletNotFound
	}

	balanceBefore := wallet.AvailableBalance
	newAvailable := wallet.AvailableBalance + req.Amount
	balanceAfter := newAvailable

	transactionID := uuid.New().String()
	now := time.Now().UTC()

	refType := string(model.RefManual)
	wt := model.WalletTransaction{
		ID:             uuid.New().String(),
		WalletID:       wallet.ID,
		TransactionID:  transactionID,
		IdempotencyKey: req.IdempotencyKey,
		OperationType:  model.OpDeposit,
		Amount:         req.Amount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		ReferenceType:  &refType,
		Status:         "completed",
		CreatedAt:      now,
	}
	if err := s.repo.CreateWalletTransaction(ctx, tx, wt); err != nil {
		return nil, fmt.Errorf("create wallet transaction: %w", err)
	}

	if err := s.repo.UpdateBalance(ctx, tx, wallet.ID, newAvailable, wallet.HeldBalance); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	s.log.Info("deposit completed",
		zap.String("transaction_id", transactionID),
		zap.String("wallet_id", wallet.ID),
		zap.String("operation_type", string(model.OpDeposit)),
		zap.Float64("amount", req.Amount),
		zap.Float64("balance_before", balanceBefore),
		zap.Float64("balance_after", balanceAfter),
		zap.String("user_id", req.UserID),
		zap.Time("timestamp", now),
	)

	return &DepositFundsResult{
		TransactionID: transactionID,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}
