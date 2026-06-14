package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/auction-system/payment-service/internal/database"
	"github.com/auction-system/payment-service/internal/model"
)

// CreateHold inserts a bid hold record
func CreateHold(ctx context.Context, tx pgx.Tx, h model.BidHold) error {
	query := `
		INSERT INTO bid_holds (id, auction_id, bidder_id, wallet_id, amount, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := tx.Exec(ctx, query,
		h.ID, h.AuctionID, h.BidderID, h.WalletID, h.Amount, h.Status, h.ExpiresAt, h.CreatedAt,
	)
	return err
}

// GetActiveHoldsByAuction returns all active holds for an auction
func GetActiveHoldsByAuction(ctx context.Context, auctionID string) ([]model.BidHold, error) {
	query := `
		SELECT id, auction_id, bidder_id, wallet_id, amount, status, expires_at, created_at
		FROM bid_holds
		WHERE auction_id = $1 AND status = 'active'
	`
	rows, err := database.DB.Query(ctx, query, auctionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var holds []model.BidHold
	for rows.Next() {
		var h model.BidHold
		if err := rows.Scan(&h.ID, &h.AuctionID, &h.BidderID, &h.WalletID, &h.Amount, &h.Status, &h.ExpiresAt, &h.CreatedAt); err != nil {
			return nil, err
		}
		holds = append(holds, h)
	}
	return holds, rows.Err()
}

// GetActiveHold returns the active hold for a specific bidder on an auction
func GetActiveHold(ctx context.Context, auctionID, bidderID string) (*model.BidHold, error) {
	query := `
		SELECT id, auction_id, bidder_id, wallet_id, amount, status, expires_at, created_at
		FROM bid_holds
		WHERE auction_id = $1 AND bidder_id = $2 AND status = 'active'
	`
	var h model.BidHold
	err := database.DB.QueryRow(ctx, query, auctionID, bidderID).Scan(
		&h.ID, &h.AuctionID, &h.BidderID, &h.WalletID, &h.Amount, &h.Status, &h.ExpiresAt, &h.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("hold not found: %w", err)
	}
	return &h, nil
}

// UpdateHoldStatus changes the hold status (within a transaction)
func UpdateHoldStatus(ctx context.Context, tx pgx.Tx, holdID string, status model.HoldStatus) error {
	query := `UPDATE bid_holds SET status = $1 WHERE id = $2`
	_, err := tx.Exec(ctx, query, status, holdID)
	return err
}
