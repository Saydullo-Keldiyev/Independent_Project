package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auction-system/bid-service/internal/database"
	"github.com/auction-system/bid-service/internal/model"
)

// CreateBid inserts a new bid into the database
func CreateBid(ctx context.Context, bid model.Bid) error {
	query := `
		INSERT INTO bids (id, auction_id, user_id, amount, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := database.DB.Exec(ctx, query,
		bid.ID,
		bid.AuctionID,
		bid.UserID,
		bid.Amount,
		bid.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create bid: %w", err)
	}
	return nil
}

// GetHighestBid returns the highest bid amount for an auction from the DB
func GetHighestBid(ctx context.Context, auctionID string) (float64, error) {
	var amount float64
	query := `SELECT COALESCE(MAX(amount), 0) FROM bids WHERE auction_id = $1`
	err := database.DB.QueryRow(ctx, query, auctionID).Scan(&amount)
	if err != nil {
		return 0, fmt.Errorf("failed to get highest bid: %w", err)
	}
	return amount, nil
}

// GetBidsByAuction returns all bids for an auction ordered by amount desc
func GetBidsByAuction(ctx context.Context, auctionID string) ([]model.Bid, error) {
	query := `
		SELECT id, auction_id, user_id, amount, created_at
		FROM bids
		WHERE auction_id = $1
		ORDER BY amount DESC
	`
	rows, err := database.DB.Query(ctx, query, auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bids: %w", err)
	}
	defer rows.Close()

	var bids []model.Bid
	for rows.Next() {
		var b model.Bid
		if err := rows.Scan(&b.ID, &b.AuctionID, &b.UserID, &b.Amount, &b.CreatedAt); err != nil {
			return nil, err
		}
		bids = append(bids, b)
	}
	return bids, rows.Err()
}

// GetBidsByUser returns all bids placed by a user
func GetBidsByUser(ctx context.Context, userID string) ([]model.Bid, error) {
	query := `
		SELECT id, auction_id, user_id, amount, created_at
		FROM bids
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user bids: %w", err)
	}
	defer rows.Close()

	var bids []model.Bid
	for rows.Next() {
		var b model.Bid
		if err := rows.Scan(&b.ID, &b.AuctionID, &b.UserID, &b.Amount, &b.CreatedAt); err != nil {
			return nil, err
		}
		bids = append(bids, b)
	}
	return bids, rows.Err()
}

// UserAlreadyBid checks if a user has already placed a bid on an auction
func UserAlreadyBid(ctx context.Context, auctionID, userID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM bids WHERE auction_id = $1 AND user_id = $2`
	err := database.DB.QueryRow(ctx, query, auctionID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetBidByID returns a single bid by its ID
func GetBidByID(ctx context.Context, bidID string) (*model.Bid, error) {
	query := `
		SELECT id, auction_id, user_id, amount, created_at
		FROM bids WHERE id = $1
	`
	var b model.Bid
	err := database.DB.QueryRow(ctx, query, bidID).Scan(
		&b.ID, &b.AuctionID, &b.UserID, &b.Amount, &b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("bid not found: %w", err)
	}
	return &b, nil
}

// GetLatestBidTime returns the time of the most recent bid for an auction
func GetLatestBidTime(ctx context.Context, auctionID string) (*time.Time, error) {
	var t time.Time
	query := `SELECT MAX(created_at) FROM bids WHERE auction_id = $1`
	err := database.DB.QueryRow(ctx, query, auctionID).Scan(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetHighestBidder returns the user_id of the highest bidder excluding a specific bid
func GetHighestBidder(ctx context.Context, auctionID string, excludeBidID string) (string, error) {
	var userID string
	query := `SELECT user_id FROM bids WHERE auction_id = $1 AND id != $2 ORDER BY amount DESC LIMIT 1`
	err := database.DB.QueryRow(ctx, query, auctionID, excludeBidID).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}
