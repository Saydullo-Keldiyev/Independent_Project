package repository

import (
	"context"
	"fmt"

	"github.com/auction-system/bid-service/internal/database"
	"github.com/auction-system/bid-service/internal/model"
)

// GetAuctionByID fetches an auction by its ID
func GetAuctionByID(ctx context.Context, auctionID string) (*model.Auction, error) {
	query := `
		SELECT id, title, starting_price, current_price, state,
		       seller_id, winner_id, start_time, end_time, created_at
		FROM auctions
		WHERE id = $1 AND deleted_at IS NULL
	`
	var a model.Auction
	err := database.DB.QueryRow(ctx, query, auctionID).Scan(
		&a.ID,
		&a.Title,
		&a.StartPrice,
		&a.CurrentPrice,
		&a.Status,
		&a.SellerID,
		&a.WinnerID,
		&a.StartAt,
		&a.EndAt,
		&a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("auction not found: %w", err)
	}
	return &a, nil
}

// UpdateCurrentPrice updates the current highest price of an auction
func UpdateCurrentPrice(ctx context.Context, auctionID string, amount float64) error {
	query := `UPDATE auctions SET current_price = $1 WHERE id = $2`
	_, err := database.DB.Exec(ctx, query, amount, auctionID)
	if err != nil {
		return fmt.Errorf("failed to update auction price: %w", err)
	}
	return nil
}
