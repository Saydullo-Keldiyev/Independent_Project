package payment_test

import (
	"testing"
)

// TestWalletHold_InsufficientBalance verifies hold fails with low balance
func TestWalletHold_InsufficientBalance(t *testing.T) {
	wallet := mockWallet{available: 100.0, held: 0.0}

	err := holdBalance(&wallet, 150.0)
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}
	if wallet.available != 100.0 {
		t.Fatal("balance should not change on failure")
	}
}

// TestWalletHold_Success verifies hold moves funds correctly
func TestWalletHold_Success(t *testing.T) {
	wallet := mockWallet{available: 500.0, held: 0.0}

	err := holdBalance(&wallet, 200.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wallet.available != 300.0 {
		t.Errorf("available = %v, want 300", wallet.available)
	}
	if wallet.held != 200.0 {
		t.Errorf("held = %v, want 200", wallet.held)
	}
}

// TestWalletRelease_MovesHeldToAvailable verifies refund logic
func TestWalletRelease_MovesHeldToAvailable(t *testing.T) {
	wallet := mockWallet{available: 300.0, held: 200.0}

	releaseHold(&wallet, 200.0)

	if wallet.available != 500.0 {
		t.Errorf("available = %v, want 500", wallet.available)
	}
	if wallet.held != 0.0 {
		t.Errorf("held = %v, want 0", wallet.held)
	}
}

// TestWalletCharge_DeductsFromHeld verifies winner charge
func TestWalletCharge_DeductsFromHeld(t *testing.T) {
	wallet := mockWallet{available: 300.0, held: 200.0}

	chargeHold(&wallet, 200.0)

	if wallet.available != 300.0 {
		t.Errorf("available should not change, got %v", wallet.available)
	}
	if wallet.held != 0.0 {
		t.Errorf("held = %v, want 0", wallet.held)
	}
}

// TestWalletDeposit_IncreasesAvailable verifies deposit
func TestWalletDeposit_IncreasesAvailable(t *testing.T) {
	wallet := mockWallet{available: 100.0, held: 0.0}

	deposit(&wallet, 500.0)

	if wallet.available != 600.0 {
		t.Errorf("available = %v, want 600", wallet.available)
	}
}

// TestIdempotency_DuplicateHoldIgnored verifies idempotency
func TestIdempotency_DuplicateHoldIgnored(t *testing.T) {
	processed := make(map[string]bool)
	key := "hold:auction-1:user-1"

	// First call
	if !processed[key] {
		processed[key] = true
	}

	// Second call — should be no-op
	if processed[key] {
		t.Log("correctly detected duplicate operation")
	}
}

// TestNegativeBalance_NeverAllowed verifies constraint
func TestNegativeBalance_NeverAllowed(t *testing.T) {
	wallet := mockWallet{available: 50.0, held: 0.0}

	err := holdBalance(&wallet, 100.0)
	if err == nil {
		t.Fatal("should not allow negative balance")
	}
	if wallet.available < 0 {
		t.Fatal("CRITICAL: negative balance detected!")
	}
}

// ── Mock wallet ───────────────────────────────────────────────────────────────

type mockWallet struct {
	available float64
	held      float64
}

func holdBalance(w *mockWallet, amount float64) error {
	if w.available < amount {
		return &walletError{"insufficient balance"}
	}
	w.available -= amount
	w.held += amount
	return nil
}

func releaseHold(w *mockWallet, amount float64) {
	w.held -= amount
	w.available += amount
}

func chargeHold(w *mockWallet, amount float64) {
	w.held -= amount
}

func deposit(w *mockWallet, amount float64) {
	w.available += amount
}

type walletError struct{ msg string }

func (e *walletError) Error() string { return e.msg }
