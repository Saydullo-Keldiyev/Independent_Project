package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// SettlementData holds the data required for the auction settlement saga.
// It is serialized to the saga's `data` JSON column for durability.
type SettlementData struct {
	AuctionID     string   `json:"auction_id"`
	WinnerID      string   `json:"winner_id"`
	SellerID      string   `json:"seller_id"`
	WinningAmount float64  `json:"winning_amount"`
	WinnerHoldID  string   `json:"winner_hold_id"`
	LoserHoldIDs  []string `json:"loser_hold_ids"`
}

// NewSettlementSaga creates the settlement saga definition with the standard
// ordered steps: charge winner → credit seller → release loser holds → publish events.
//
// Each step has a corresponding compensate function that reverses the action.
//
// Requirement 18.3: Settlement order (charge → credit → release → publish)
// Requirement 18.4: Reverse compensation on failure within 60s
func NewSettlementSaga(db *pgxpool.Pool, log *zap.Logger) *Definition {
	return &Definition{
		Type: "auction_settlement",
		Steps: []Step{
			{
				Name:       "charge_winner",
				Execute:    chargeWinnerStep(db, log),
				Compensate: reverseChargeStep(db, log),
				Timeout:    30 * time.Second,
			},
			{
				Name:       "credit_seller",
				Execute:    creditSellerStep(db, log),
				Compensate: reverseCreditStep(db, log),
				Timeout:    30 * time.Second,
			},
			{
				Name:       "release_loser_holds",
				Execute:    releaseLoserHoldsStep(db, log),
				Compensate: reverseReleaseStep(db, log),
				Timeout:    30 * time.Second,
			},
			{
				Name:       "publish_settlement_events",
				Execute:    publishSettlementEventsStep(db, log),
				Compensate: nil, // Event publication has no compensation (idempotent consumers)
				Timeout:    30 * time.Second,
			},
		},
	}
}

// ToDataMap converts SettlementData to the generic map used by the saga orchestrator.
func (sd *SettlementData) ToDataMap() map[string]interface{} {
	return map[string]interface{}{
		"auction_id":     sd.AuctionID,
		"winner_id":      sd.WinnerID,
		"seller_id":      sd.SellerID,
		"winning_amount": sd.WinningAmount,
		"winner_hold_id": sd.WinnerHoldID,
		"loser_hold_ids": sd.LoserHoldIDs,
	}
}

// ParseSettlementData extracts SettlementData from the generic data map.
func ParseSettlementData(data map[string]interface{}) (*SettlementData, error) {
	sd := &SettlementData{}

	if v, ok := data["auction_id"].(string); ok {
		sd.AuctionID = v
	} else {
		return nil, fmt.Errorf("missing or invalid auction_id")
	}

	if v, ok := data["winner_id"].(string); ok {
		sd.WinnerID = v
	} else {
		return nil, fmt.Errorf("missing or invalid winner_id")
	}

	if v, ok := data["seller_id"].(string); ok {
		sd.SellerID = v
	} else {
		return nil, fmt.Errorf("missing or invalid seller_id")
	}

	switch v := data["winning_amount"].(type) {
	case float64:
		sd.WinningAmount = v
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return nil, fmt.Errorf("invalid winning_amount: %w", err)
		}
		sd.WinningAmount = f
	default:
		return nil, fmt.Errorf("missing or invalid winning_amount")
	}

	if v, ok := data["winner_hold_id"].(string); ok {
		sd.WinnerHoldID = v
	} else {
		return nil, fmt.Errorf("missing or invalid winner_hold_id")
	}

	if v, ok := data["loser_hold_ids"]; ok {
		switch ids := v.(type) {
		case []string:
			sd.LoserHoldIDs = ids
		case []interface{}:
			for _, id := range ids {
				if s, ok := id.(string); ok {
					sd.LoserHoldIDs = append(sd.LoserHoldIDs, s)
				}
			}
		}
	}

	return sd, nil
}

// Step 1: Charge the winner's held balance
func chargeWinnerStep(db *pgxpool.Pool, log *zap.Logger) func(ctx context.Context, data map[string]interface{}) error {
	return func(ctx context.Context, data map[string]interface{}) error {
		sd, err := ParseSettlementData(data)
		if err != nil {
			return fmt.Errorf("parse settlement data: %w", err)
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)

		// Lock wallet
		var walletID string
		var availableBalance, heldBalance float64
		err = tx.QueryRow(ctx,
			`SELECT w.id, w.available_balance, w.held_balance FROM wallets w WHERE w.user_id = $1 FOR UPDATE`,
			sd.WinnerID,
		).Scan(&walletID, &availableBalance, &heldBalance)
		if err != nil {
			return fmt.Errorf("lock winner wallet: %w", err)
		}

		// Deduct from held balance (charge the winner)
		newHeld := heldBalance - sd.WinningAmount
		if newHeld < 0 {
			return fmt.Errorf("insufficient held balance: have %.2f, need %.2f", heldBalance, sd.WinningAmount)
		}

		_, err = tx.Exec(ctx,
			`UPDATE wallets SET held_balance = $1, updated_at = NOW() WHERE id = $2`,
			newHeld, walletID,
		)
		if err != nil {
			return fmt.Errorf("update winner balance: %w", err)
		}

		// Mark hold as charged
		_, err = tx.Exec(ctx,
			`UPDATE bid_holds SET status = 'charged' WHERE id = $1`,
			sd.WinnerHoldID,
		)
		if err != nil {
			return fmt.Errorf("mark hold charged: %w", err)
		}

		// Audit log via wallet_transactions
		idempKey := fmt.Sprintf("saga:charge:%s:%s", sd.AuctionID, sd.WinnerID)
		_, err = tx.Exec(ctx,
			`INSERT INTO wallet_transactions (id, wallet_id, transaction_id, idempotency_key, operation_type, amount, balance_before, balance_after, reference_type, reference_id, status, created_at)
			 VALUES ($1, $2, $3, $4, 'charge', $5, $6, $7, 'auction', $8, 'completed', NOW())
			 ON CONFLICT (idempotency_key) DO NOTHING`,
			uuid.New().String(), walletID, uuid.New().String(), idempKey,
			sd.WinningAmount, heldBalance, newHeld,
			sd.AuctionID,
		)
		if err != nil {
			return fmt.Errorf("audit log charge: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit charge: %w", err)
		}

		log.Info("saga: winner charged",
			zap.String("auction_id", sd.AuctionID),
			zap.String("winner_id", sd.WinnerID),
			zap.Float64("amount", sd.WinningAmount),
		)
		return nil
	}
}

// Compensate step 1: Reverse the charge (restore held balance)
func reverseChargeStep(db *pgxpool.Pool, log *zap.Logger) func(ctx context.Context, data map[string]interface{}) error {
	return func(ctx context.Context, data map[string]interface{}) error {
		sd, err := ParseSettlementData(data)
		if err != nil {
			return err
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)

		// Lock wallet and restore held balance
		var walletID string
		var heldBalance float64
		err = tx.QueryRow(ctx,
			`SELECT id, held_balance FROM wallets WHERE user_id = $1 FOR UPDATE`,
			sd.WinnerID,
		).Scan(&walletID, &heldBalance)
		if err != nil {
			return fmt.Errorf("lock winner wallet for compensation: %w", err)
		}

		newHeld := heldBalance + sd.WinningAmount
		_, err = tx.Exec(ctx,
			`UPDATE wallets SET held_balance = $1, updated_at = NOW() WHERE id = $2`,
			newHeld, walletID,
		)
		if err != nil {
			return fmt.Errorf("restore held balance: %w", err)
		}

		// Revert hold status back to active
		_, err = tx.Exec(ctx,
			`UPDATE bid_holds SET status = 'active' WHERE id = $1`,
			sd.WinnerHoldID,
		)
		if err != nil {
			return fmt.Errorf("revert hold status: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit reverse charge: %w", err)
		}

		log.Info("saga: winner charge reversed (compensation)",
			zap.String("auction_id", sd.AuctionID),
			zap.String("winner_id", sd.WinnerID),
			zap.Float64("amount", sd.WinningAmount),
		)
		return nil
	}
}

// Step 2: Credit the seller's available balance
func creditSellerStep(db *pgxpool.Pool, log *zap.Logger) func(ctx context.Context, data map[string]interface{}) error {
	return func(ctx context.Context, data map[string]interface{}) error {
		sd, err := ParseSettlementData(data)
		if err != nil {
			return fmt.Errorf("parse settlement data: %w", err)
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)

		// Lock seller wallet
		var walletID string
		var availableBalance float64
		err = tx.QueryRow(ctx,
			`SELECT id, available_balance FROM wallets WHERE user_id = $1 FOR UPDATE`,
			sd.SellerID,
		).Scan(&walletID, &availableBalance)
		if err != nil {
			return fmt.Errorf("lock seller wallet: %w", err)
		}

		// Credit seller
		newAvailable := availableBalance + sd.WinningAmount
		_, err = tx.Exec(ctx,
			`UPDATE wallets SET available_balance = $1, updated_at = NOW() WHERE id = $2`,
			newAvailable, walletID,
		)
		if err != nil {
			return fmt.Errorf("credit seller: %w", err)
		}

		// Audit log
		idempKey := fmt.Sprintf("saga:credit:%s:%s", sd.AuctionID, sd.SellerID)
		_, err = tx.Exec(ctx,
			`INSERT INTO wallet_transactions (id, wallet_id, transaction_id, idempotency_key, operation_type, amount, balance_before, balance_after, reference_type, reference_id, status, created_at)
			 VALUES ($1, $2, $3, $4, 'credit', $5, $6, $7, 'auction', $8, 'completed', NOW())
			 ON CONFLICT (idempotency_key) DO NOTHING`,
			uuid.New().String(), walletID, uuid.New().String(), idempKey,
			sd.WinningAmount, availableBalance, newAvailable,
			sd.AuctionID,
		)
		if err != nil {
			return fmt.Errorf("audit log credit: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit credit: %w", err)
		}

		log.Info("saga: seller credited",
			zap.String("auction_id", sd.AuctionID),
			zap.String("seller_id", sd.SellerID),
			zap.Float64("amount", sd.WinningAmount),
		)
		return nil
	}
}

// Compensate step 2: Reverse the credit (deduct from seller)
func reverseCreditStep(db *pgxpool.Pool, log *zap.Logger) func(ctx context.Context, data map[string]interface{}) error {
	return func(ctx context.Context, data map[string]interface{}) error {
		sd, err := ParseSettlementData(data)
		if err != nil {
			return err
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)

		var walletID string
		var availableBalance float64
		err = tx.QueryRow(ctx,
			`SELECT id, available_balance FROM wallets WHERE user_id = $1 FOR UPDATE`,
			sd.SellerID,
		).Scan(&walletID, &availableBalance)
		if err != nil {
			return fmt.Errorf("lock seller wallet for compensation: %w", err)
		}

		newAvailable := availableBalance - sd.WinningAmount
		if newAvailable < 0 {
			newAvailable = 0 // Safety guard
		}

		_, err = tx.Exec(ctx,
			`UPDATE wallets SET available_balance = $1, updated_at = NOW() WHERE id = $2`,
			newAvailable, walletID,
		)
		if err != nil {
			return fmt.Errorf("reverse credit: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit reverse credit: %w", err)
		}

		log.Info("saga: seller credit reversed (compensation)",
			zap.String("auction_id", sd.AuctionID),
			zap.String("seller_id", sd.SellerID),
			zap.Float64("amount", sd.WinningAmount),
		)
		return nil
	}
}

// Step 3: Release all loser holds (held → available)
func releaseLoserHoldsStep(db *pgxpool.Pool, log *zap.Logger) func(ctx context.Context, data map[string]interface{}) error {
	return func(ctx context.Context, data map[string]interface{}) error {
		sd, err := ParseSettlementData(data)
		if err != nil {
			return fmt.Errorf("parse settlement data: %w", err)
		}

		for _, holdID := range sd.LoserHoldIDs {
			if err := releaseOneHold(ctx, db, holdID, sd.AuctionID); err != nil {
				return fmt.Errorf("release hold %s: %w", holdID, err)
			}
		}

		log.Info("saga: loser holds released",
			zap.String("auction_id", sd.AuctionID),
			zap.Int("count", len(sd.LoserHoldIDs)),
		)
		return nil
	}
}

func releaseOneHold(ctx context.Context, db *pgxpool.Pool, holdID, auctionID string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get hold details with lock
	var bidderID, walletID string
	var amount float64
	var status string
	err = tx.QueryRow(ctx,
		`SELECT bidder_id, wallet_id, amount, status FROM bid_holds WHERE id = $1 FOR UPDATE`,
		holdID,
	).Scan(&bidderID, &walletID, &amount, &status)
	if err != nil {
		return fmt.Errorf("get hold: %w", err)
	}

	// Skip if already released
	if status != "active" {
		return nil
	}

	// Lock wallet
	var availableBalance, heldBalance float64
	err = tx.QueryRow(ctx,
		`SELECT available_balance, held_balance FROM wallets WHERE id = $1 FOR UPDATE`,
		walletID,
	).Scan(&availableBalance, &heldBalance)
	if err != nil {
		return fmt.Errorf("lock wallet: %w", err)
	}

	// Move: held → available
	newAvailable := availableBalance + amount
	newHeld := heldBalance - amount
	_, err = tx.Exec(ctx,
		`UPDATE wallets SET available_balance = $1, held_balance = $2, updated_at = NOW() WHERE id = $3`,
		newAvailable, newHeld, walletID,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	// Mark hold released
	_, err = tx.Exec(ctx,
		`UPDATE bid_holds SET status = 'released' WHERE id = $1`,
		holdID,
	)
	if err != nil {
		return fmt.Errorf("mark released: %w", err)
	}

	// Audit log
	idempKey := fmt.Sprintf("saga:release:%s:%s", auctionID, holdID)
	_, err = tx.Exec(ctx,
		`INSERT INTO wallet_transactions (id, wallet_id, transaction_id, idempotency_key, operation_type, amount, balance_before, balance_after, reference_type, reference_id, status, created_at)
		 VALUES ($1, $2, $3, $4, 'release', $5, $6, $7, 'auction', $8, 'completed', NOW())
		 ON CONFLICT (idempotency_key) DO NOTHING`,
		uuid.New().String(), walletID, uuid.New().String(), idempKey,
		amount, heldBalance, newHeld,
		auctionID,
	)
	if err != nil {
		return fmt.Errorf("audit log release: %w", err)
	}

	return tx.Commit(ctx)
}

// Compensate step 3: Re-hold the released funds (available → held)
func reverseReleaseStep(db *pgxpool.Pool, log *zap.Logger) func(ctx context.Context, data map[string]interface{}) error {
	return func(ctx context.Context, data map[string]interface{}) error {
		sd, err := ParseSettlementData(data)
		if err != nil {
			return err
		}

		for _, holdID := range sd.LoserHoldIDs {
			if err := reHoldOne(ctx, db, holdID); err != nil {
				log.Error("reverse release failed for hold",
					zap.String("hold_id", holdID),
					zap.Error(err),
				)
				// Continue with other holds
			}
		}

		log.Info("saga: loser holds re-held (compensation)",
			zap.String("auction_id", sd.AuctionID),
			zap.Int("count", len(sd.LoserHoldIDs)),
		)
		return nil
	}
}

func reHoldOne(ctx context.Context, db *pgxpool.Pool, holdID string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var walletID string
	var amount float64
	var status string
	err = tx.QueryRow(ctx,
		`SELECT wallet_id, amount, status FROM bid_holds WHERE id = $1 FOR UPDATE`,
		holdID,
	).Scan(&walletID, &amount, &status)
	if err != nil {
		return err
	}

	// Only reverse if it was released
	if status != "released" {
		return nil
	}

	var availableBalance, heldBalance float64
	err = tx.QueryRow(ctx,
		`SELECT available_balance, held_balance FROM wallets WHERE id = $1 FOR UPDATE`,
		walletID,
	).Scan(&availableBalance, &heldBalance)
	if err != nil {
		return err
	}

	// Move back: available → held
	newAvailable := availableBalance - amount
	newHeld := heldBalance + amount
	_, err = tx.Exec(ctx,
		`UPDATE wallets SET available_balance = $1, held_balance = $2, updated_at = NOW() WHERE id = $3`,
		newAvailable, newHeld, walletID,
	)
	if err != nil {
		return err
	}

	// Revert hold to active
	_, err = tx.Exec(ctx,
		`UPDATE bid_holds SET status = 'active' WHERE id = $1`,
		holdID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Step 4: Publish settlement events via outbox
func publishSettlementEventsStep(db *pgxpool.Pool, log *zap.Logger) func(ctx context.Context, data map[string]interface{}) error {
	return func(ctx context.Context, data map[string]interface{}) error {
		sd, err := ParseSettlementData(data)
		if err != nil {
			return fmt.Errorf("parse settlement data: %w", err)
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)

		// Publish auction.settled event
		settledPayload, _ := json.Marshal(map[string]interface{}{
			"auction_id":     sd.AuctionID,
			"winner_id":      sd.WinnerID,
			"seller_id":      sd.SellerID,
			"winning_amount": sd.WinningAmount,
			"settled_at":     time.Now().UTC().Format(time.RFC3339),
		})

		idempKey := fmt.Sprintf("saga:event:settled:%s", sd.AuctionID)
		_, err = tx.Exec(ctx,
			`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, idempotency_key, created_at)
			 VALUES ($1, 'auction', $2, 'auction.settled', $3, $4, NOW())
			 ON CONFLICT (idempotency_key) DO NOTHING`,
			uuid.New().String(), sd.AuctionID, settledPayload, idempKey,
		)
		if err != nil {
			return fmt.Errorf("insert settled event: %w", err)
		}

		// Publish payment.completed event
		paymentPayload, _ := json.Marshal(map[string]interface{}{
			"auction_id": sd.AuctionID,
			"winner_id":  sd.WinnerID,
			"seller_id":  sd.SellerID,
			"amount":     sd.WinningAmount,
		})

		paymentIdempKey := fmt.Sprintf("saga:event:payment:%s", sd.AuctionID)
		_, err = tx.Exec(ctx,
			`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, idempotency_key, created_at)
			 VALUES ($1, 'payment', $2, 'payment.completed', $3, $4, NOW())
			 ON CONFLICT (idempotency_key) DO NOTHING`,
			uuid.New().String(), sd.AuctionID, paymentPayload, paymentIdempKey,
		)
		if err != nil {
			return fmt.Errorf("insert payment event: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit outbox events: %w", err)
		}

		log.Info("saga: settlement events published to outbox",
			zap.String("auction_id", sd.AuctionID),
		)
		return nil
	}
}
