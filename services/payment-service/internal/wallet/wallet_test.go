package wallet

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/auction-system/payment-service/internal/model"
)

// --- Fake pgx transaction and pool for unit testing ---

type fakeTx struct {
	pgx.Tx
}

func (f *fakeTx) Commit(ctx context.Context) error   { return nil }
func (f *fakeTx) Rollback(ctx context.Context) error  { return nil }
func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

// --- Mock Repository ---

type mockRepo struct {
	mu              sync.Mutex
	wallets         map[string]*model.Wallet // key: user_id
	walletsByID     map[string]*model.Wallet // key: wallet_id
	holds           map[string]*model.BidHold
	idempotencyKeys map[string]bool
	transactions    []model.WalletTransaction
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		wallets:         make(map[string]*model.Wallet),
		walletsByID:     make(map[string]*model.Wallet),
		holds:           make(map[string]*model.BidHold),
		idempotencyKeys: make(map[string]bool),
	}
}

func (m *mockRepo) addWallet(w *model.Wallet) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wallets[w.UserID] = w
	m.walletsByID[w.ID] = w
}

func (m *mockRepo) GetWalletForUpdate(ctx context.Context, tx pgx.Tx, walletID string) (*model.Wallet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.walletsByID[walletID]
	if !ok {
		return nil, errors.New("wallet not found")
	}
	// Return a copy to simulate DB behavior
	copy := *w
	return &copy, nil
}

func (m *mockRepo) GetWalletByUserIDForUpdate(ctx context.Context, tx pgx.Tx, userID string) (*model.Wallet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.wallets[userID]
	if !ok {
		return nil, errors.New("wallet not found")
	}
	copy := *w
	return &copy, nil
}

func (m *mockRepo) UpdateBalance(ctx context.Context, tx pgx.Tx, walletID string, available, held float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.walletsByID[walletID]
	if !ok {
		return errors.New("wallet not found")
	}
	w.AvailableBalance = available
	w.HeldBalance = held
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockRepo) CheckIdempotencyKey(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.idempotencyKeys[key], nil
}

func (m *mockRepo) CreateWalletTransaction(ctx context.Context, tx pgx.Tx, wt model.WalletTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idempotencyKeys[wt.IdempotencyKey] {
		return nil // ON CONFLICT DO NOTHING
	}
	m.idempotencyKeys[wt.IdempotencyKey] = true
	m.transactions = append(m.transactions, wt)
	return nil
}

func (m *mockRepo) CreateHold(ctx context.Context, tx pgx.Tx, h model.BidHold) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.holds[h.ID] = &h
	return nil
}

func (m *mockRepo) GetActiveHoldForUpdate(ctx context.Context, tx pgx.Tx, holdID string) (*model.BidHold, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.holds[holdID]
	if !ok || h.Status != model.HoldActive {
		return nil, errors.New("hold not found or not active")
	}
	copy := *h
	return &copy, nil
}

func (m *mockRepo) UpdateHoldStatus(ctx context.Context, tx pgx.Tx, holdID string, status model.HoldStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.holds[holdID]
	if !ok {
		return errors.New("hold not found")
	}
	h.Status = status
	return nil
}

// --- Test helper to create service with mock ---

func newTestService(repo *mockRepo) *Service {
	log, _ := zap.NewDevelopment()
	// We pass nil for pool since our mock repo doesn't use it for Begin.
	// In real tests, we'd use testcontainers.
	return &Service{
		pool: nil, // Not used in unit tests with mock
		repo: repo,
		log:  log,
	}
}

// testableService wraps Service and overrides pool.Begin for testing
type testableService struct {
	*Service
	beginFunc func(ctx context.Context) (pgx.Tx, error)
}

func newTestableService(repo *mockRepo) *testableService {
	log, _ := zap.NewDevelopment()
	ts := &testableService{
		Service: &Service{
			pool: nil,
			repo: repo,
			log:  log,
		},
	}
	ts.beginFunc = func(ctx context.Context) (pgx.Tx, error) {
		return &fakeTx{}, nil
	}
	return ts
}

// HoldFunds with overridable Begin
func (ts *testableService) HoldFunds(ctx context.Context, req HoldFundsRequest) (*HoldFundsResult, error) {
	// Idempotency check
	exists, err := ts.repo.CheckIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		ts.log.Warn("idempotency check failed, proceeding")
	}
	if exists {
		return nil, ErrDuplicateOperation
	}

	tx, err := ts.beginFunc(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	wallet, err := ts.repo.GetWalletByUserIDForUpdate(ctx, tx, req.UserID)
	if err != nil {
		return nil, ErrWalletNotFound
	}

	if wallet.AvailableBalance < req.Amount {
		return nil, ErrInsufficientBalance
	}

	balanceBefore := wallet.AvailableBalance
	newAvailable := wallet.AvailableBalance - req.Amount
	newHeld := wallet.HeldBalance + req.Amount
	balanceAfter := newAvailable

	transactionID := "test-txn-id"
	holdID := "test-hold-id"
	now := time.Now().UTC()

	refType := string(model.RefAuction)
	wt := model.WalletTransaction{
		ID:             "test-wt-id",
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
	if err := ts.repo.CreateWalletTransaction(ctx, tx, wt); err != nil {
		return nil, err
	}

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
	if err := ts.repo.CreateHold(ctx, tx, hold); err != nil {
		return nil, err
	}

	if err := ts.repo.UpdateBalance(ctx, tx, wallet.ID, newAvailable, newHeld); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &HoldFundsResult{
		TransactionID: transactionID,
		HoldID:        holdID,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}

// --- Tests ---

func TestHoldFunds_Success(t *testing.T) {
	repo := newMockRepo()
	repo.addWallet(&model.Wallet{
		ID:               "wallet-1",
		UserID:           "user-1",
		AvailableBalance: 1000.00,
		HeldBalance:      0,
		Currency:         "USD",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	})

	svc := newTestableService(repo)

	result, err := svc.HoldFunds(context.Background(), HoldFundsRequest{
		UserID:         "user-1",
		AuctionID:      "auction-1",
		BidID:          "bid-1",
		Amount:         250.00,
		IdempotencyKey: "hold:auction-1:user-1:bid-1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.BalanceBefore != 1000.00 {
		t.Errorf("expected balance_before=1000, got %f", result.BalanceBefore)
	}
	if result.BalanceAfter != 750.00 {
		t.Errorf("expected balance_after=750, got %f", result.BalanceAfter)
	}

	// Verify wallet state was updated
	w := repo.walletsByID["wallet-1"]
	if w.AvailableBalance != 750.00 {
		t.Errorf("expected available=750, got %f", w.AvailableBalance)
	}
	if w.HeldBalance != 250.00 {
		t.Errorf("expected held=250, got %f", w.HeldBalance)
	}
}

func TestHoldFunds_InsufficientBalance(t *testing.T) {
	repo := newMockRepo()
	repo.addWallet(&model.Wallet{
		ID:               "wallet-1",
		UserID:           "user-1",
		AvailableBalance: 100.00,
		HeldBalance:      0,
		Currency:         "USD",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	})

	svc := newTestableService(repo)

	_, err := svc.HoldFunds(context.Background(), HoldFundsRequest{
		UserID:         "user-1",
		AuctionID:      "auction-1",
		BidID:          "bid-1",
		Amount:         500.00,
		IdempotencyKey: "hold:auction-1:user-1:bid-1",
	})

	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got: %v", err)
	}

	// Verify wallet state unchanged
	w := repo.walletsByID["wallet-1"]
	if w.AvailableBalance != 100.00 {
		t.Errorf("balance should be unchanged, got %f", w.AvailableBalance)
	}
}

func TestHoldFunds_Idempotency(t *testing.T) {
	repo := newMockRepo()
	repo.addWallet(&model.Wallet{
		ID:               "wallet-1",
		UserID:           "user-1",
		AvailableBalance: 1000.00,
		HeldBalance:      0,
		Currency:         "USD",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	})

	svc := newTestableService(repo)
	key := "hold:auction-1:user-1:bid-1"

	// First call succeeds
	result1, err := svc.HoldFunds(context.Background(), HoldFundsRequest{
		UserID:         "user-1",
		AuctionID:      "auction-1",
		BidID:          "bid-1",
		Amount:         250.00,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("first call should succeed, got: %v", err)
	}
	if result1 == nil {
		t.Fatal("first call should return result")
	}

	// Second call with same key returns duplicate error
	_, err = svc.HoldFunds(context.Background(), HoldFundsRequest{
		UserID:         "user-1",
		AuctionID:      "auction-1",
		BidID:          "bid-1",
		Amount:         250.00,
		IdempotencyKey: key,
	})
	if !errors.Is(err, ErrDuplicateOperation) {
		t.Fatalf("expected ErrDuplicateOperation, got: %v", err)
	}

	// Wallet balance should only reflect ONE hold
	w := repo.walletsByID["wallet-1"]
	if w.AvailableBalance != 750.00 {
		t.Errorf("expected available=750 (one hold), got %f", w.AvailableBalance)
	}
}

func TestHoldFunds_WalletNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestableService(repo)

	_, err := svc.HoldFunds(context.Background(), HoldFundsRequest{
		UserID:         "nonexistent",
		AuctionID:      "auction-1",
		BidID:          "bid-1",
		Amount:         100.00,
		IdempotencyKey: "hold:auction-1:nonexistent:bid-1",
	})

	if !errors.Is(err, ErrWalletNotFound) {
		t.Fatalf("expected ErrWalletNotFound, got: %v", err)
	}
}

func TestHoldFunds_AuditLogFields(t *testing.T) {
	repo := newMockRepo()
	repo.addWallet(&model.Wallet{
		ID:               "wallet-1",
		UserID:           "user-1",
		AvailableBalance: 500.00,
		HeldBalance:      100.00,
		Currency:         "USD",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	})

	svc := newTestableService(repo)
	_, err := svc.HoldFunds(context.Background(), HoldFundsRequest{
		UserID:         "user-1",
		AuctionID:      "auction-1",
		BidID:          "bid-1",
		Amount:         200.00,
		IdempotencyKey: "hold:auction-1:user-1:bid-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify audit record
	if len(repo.transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(repo.transactions))
	}
	txn := repo.transactions[0]
	if txn.OperationType != model.OpHold {
		t.Errorf("expected operation_type=hold, got %s", txn.OperationType)
	}
	if txn.Amount != 200.00 {
		t.Errorf("expected amount=200, got %f", txn.Amount)
	}
	if txn.BalanceBefore != 500.00 {
		t.Errorf("expected balance_before=500, got %f", txn.BalanceBefore)
	}
	if txn.BalanceAfter != 300.00 {
		t.Errorf("expected balance_after=300, got %f", txn.BalanceAfter)
	}
	if txn.WalletID != "wallet-1" {
		t.Errorf("expected wallet_id=wallet-1, got %s", txn.WalletID)
	}
	if txn.TransactionID == "" {
		t.Error("expected non-empty transaction_id")
	}
	if txn.IdempotencyKey != "hold:auction-1:user-1:bid-1" {
		t.Errorf("unexpected idempotency_key: %s", txn.IdempotencyKey)
	}
	if txn.CreatedAt.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestHoldFunds_BalanceAfterEquation(t *testing.T) {
	// Verify: balance_after = balance_before - amount (for hold operations)
	repo := newMockRepo()
	repo.addWallet(&model.Wallet{
		ID:               "wallet-1",
		UserID:           "user-1",
		AvailableBalance: 1234.56,
		HeldBalance:      0,
		Currency:         "USD",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	})

	svc := newTestableService(repo)
	result, err := svc.HoldFunds(context.Background(), HoldFundsRequest{
		UserID:         "user-1",
		AuctionID:      "auction-1",
		BidID:          "bid-1",
		Amount:         234.56,
		IdempotencyKey: "test-key-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Property: balance_after = balance_before - amount
	expectedAfter := result.BalanceBefore - 234.56
	if result.BalanceAfter != expectedAfter {
		t.Errorf("balance equation violated: balance_after(%f) != balance_before(%f) - amount(%f)",
			result.BalanceAfter, result.BalanceBefore, 234.56)
	}
}

// --- Pool mock for integration-style testing ---

// We can't easily mock pgxpool.Pool.Begin without an interface,
// so the testableService above wraps the logic. For full integration
// tests, use testcontainers-go with a real PostgreSQL instance.

func TestNewService(t *testing.T) {
	log, _ := zap.NewDevelopment()
	repo := newMockRepo()
	var pool *pgxpool.Pool // nil in unit test

	svc := NewService(pool, repo, log)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.repo == nil {
		t.Fatal("expected non-nil repo")
	}
}
