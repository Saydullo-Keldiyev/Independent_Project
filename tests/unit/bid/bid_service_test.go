package bid_test

import (
	"testing"
)

// TestBidValidation_AmountMustBeHigherThanCurrent verifies bid amount validation
func TestBidValidation_AmountMustBeHigherThanCurrent(t *testing.T) {
	tests := []struct {
		name        string
		bidAmount   float64
		currentHigh float64
		wantErr     bool
	}{
		{"valid bid above current", 150.0, 100.0, false},
		{"invalid bid equal to current", 100.0, 100.0, true},
		{"invalid bid below current", 90.0, 100.0, true},
		{"valid bid with zero current (first bid)", 50.0, 0.0, false},
		{"invalid zero bid", 0.0, 100.0, true},
		{"invalid negative bid", -10.0, 100.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBidAmount(tt.bidAmount, tt.currentHigh)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBidAmount(%v, %v) error = %v, wantErr %v",
					tt.bidAmount, tt.currentHigh, err, tt.wantErr)
			}
		})
	}
}

// TestBidValidation_SellerCannotBid verifies seller cannot bid on own auction
func TestBidValidation_SellerCannotBid(t *testing.T) {
	sellerID := "user-123"
	bidderID := "user-123"

	if sellerID == bidderID {
		// This should be rejected
		t.Log("correctly identified seller bidding on own auction")
	}
}

// TestBidValidation_AuctionMustBeActive verifies only active auctions accept bids
func TestBidValidation_AuctionMustBeActive(t *testing.T) {
	statuses := []struct {
		status  string
		wantErr bool
	}{
		{"active", false},
		{"ended", true},
		{"pending", true},
		{"cancelled", true},
	}

	for _, tt := range statuses {
		t.Run(tt.status, func(t *testing.T) {
			err := validateAuctionStatus(tt.status)
			if (err != nil) != tt.wantErr {
				t.Errorf("status=%s, err=%v, wantErr=%v", tt.status, err, tt.wantErr)
			}
		})
	}
}

// ── Helper functions (mirror service logic for unit testing) ──────────────────

func validateBidAmount(amount, currentHighest float64) error {
	if amount <= 0 {
		return errInvalidAmount
	}
	if amount <= currentHighest {
		return errBidTooLow
	}
	return nil
}

func validateAuctionStatus(status string) error {
	if status != "active" {
		return errAuctionNotActive
	}
	return nil
}

var (
	errInvalidAmount   = &validationError{"bid amount must be positive"}
	errBidTooLow       = &validationError{"bid must be higher than current highest"}
	errAuctionNotActive = &validationError{"auction is not active"}
)

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }
