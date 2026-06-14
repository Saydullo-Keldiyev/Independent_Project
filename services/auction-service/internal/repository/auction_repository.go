package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/auction-system/auction-service/internal/database"
	"github.com/auction-system/auction-service/internal/model"
)

func Create(ctx context.Context, a model.Auction) error {
	query := `
		INSERT INTO auctions (id, seller_id, title, description, category_id, starting_price, reserve_price, current_price, state, start_time, end_time, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`
	_, err := database.DB.Exec(ctx, query,
		a.ID, a.SellerID, a.Title, a.Description, a.CategoryID,
		a.StartingPrice, a.ReservePrice, a.CurrentPrice, a.State,
		a.StartTime, a.EndTime, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func GetByID(ctx context.Context, id string) (*model.Auction, error) {
	query := `
		SELECT id, seller_id, title, description, category_id, starting_price, reserve_price, current_price,
		       state, start_time, end_time, winner_id, total_bids, created_at, updated_at, deleted_at
		FROM auctions WHERE id = $1 AND deleted_at IS NULL
	`
	var a model.Auction
	err := database.DB.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.SellerID, &a.Title, &a.Description, &a.CategoryID,
		&a.StartingPrice, &a.ReservePrice, &a.CurrentPrice,
		&a.State, &a.StartTime, &a.EndTime, &a.WinnerID, &a.TotalBids,
		&a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("auction not found: %w", err)
	}
	return &a, nil
}

func Update(ctx context.Context, a model.Auction) error {
	query := `
		UPDATE auctions SET title=$1, description=$2, reserve_price=$3, updated_at=$4
		WHERE id=$5 AND deleted_at IS NULL
	`
	_, err := database.DB.Exec(ctx, query, a.Title, a.Description, a.ReservePrice, time.Now(), a.ID)
	return err
}

func UpdateState(ctx context.Context, id string, state model.AuctionState) error {
	query := `UPDATE auctions SET state=$1, updated_at=$2 WHERE id=$3`
	_, err := database.DB.Exec(ctx, query, state, time.Now(), id)
	return err
}

func SetWinner(ctx context.Context, id, winnerID string, amount float64) error {
	query := `UPDATE auctions SET winner_id=$1, current_price=$2, state=$3, updated_at=$4 WHERE id=$5`
	_, err := database.DB.Exec(ctx, query, winnerID, amount, model.StateEnded, time.Now(), id)
	return err
}

func SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE auctions SET deleted_at=$1 WHERE id=$2`
	_, err := database.DB.Exec(ctx, query, time.Now(), id)
	return err
}

func IncrBidCount(ctx context.Context, id string, newPrice float64) error {
	query := `UPDATE auctions SET total_bids = total_bids + 1, current_price = $1, updated_at = $2 WHERE id = $3`
	_, err := database.DB.Exec(ctx, query, newPrice, time.Now(), id)
	return err
}

// ── Scheduler queries ─────────────────────────────────────────────────────────

func GetScheduledToActivate(ctx context.Context) ([]model.Auction, error) {
	query := `
		SELECT id, seller_id, title, description, category_id, starting_price, reserve_price, current_price,
		       state, start_time, end_time, winner_id, total_bids, created_at, updated_at, deleted_at
		FROM auctions
		WHERE state = 'scheduled' AND start_time <= $1 AND deleted_at IS NULL
		FOR UPDATE SKIP LOCKED
	`
	return queryAuctions(ctx, query, time.Now())
}

func GetExpiredActive(ctx context.Context) ([]model.Auction, error) {
	query := `
		SELECT id, seller_id, title, description, category_id, starting_price, reserve_price, current_price,
		       state, start_time, end_time, winner_id, total_bids, created_at, updated_at, deleted_at
		FROM auctions
		WHERE state = 'active' AND end_time <= $1 AND deleted_at IS NULL
		FOR UPDATE SKIP LOCKED
	`
	return queryAuctions(ctx, query, time.Now())
}

// ── Search/List ───────────────────────────────────────────────────────────────

func List(ctx context.Context, state string, limit, offset int) ([]model.Auction, int, error) {
	countQuery := `SELECT COUNT(*) FROM auctions WHERE deleted_at IS NULL`
	args := []any{}
	where := " AND 1=1"

	if state != "" {
		where = " AND state = $1"
		args = append(args, state)
	}

	var total int
	database.DB.QueryRow(ctx, countQuery+where, args...).Scan(&total)

	query := `
		SELECT id, seller_id, title, description, category_id, starting_price, reserve_price, current_price,
		       state, start_time, end_time, winner_id, total_bids, created_at, updated_at, deleted_at
		FROM auctions WHERE deleted_at IS NULL` + where + `
		ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)

	args = append(args, limit, offset)
	auctions, err := queryAuctions(ctx, query, args...)
	return auctions, total, err
}

func GetBySeller(ctx context.Context, sellerID string) ([]model.Auction, error) {
	query := `
		SELECT id, seller_id, title, description, category_id, starting_price, reserve_price, current_price,
		       state, start_time, end_time, winner_id, total_bids, created_at, updated_at, deleted_at
		FROM auctions WHERE seller_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	return queryAuctions(ctx, query, sellerID)
}

// GetHighestBid returns the highest bid for an auction (from bids table)
func GetHighestBid(ctx context.Context, auctionID string) (string, float64, error) {
	query := `SELECT user_id, amount FROM bids WHERE auction_id = $1 ORDER BY amount DESC LIMIT 1`
	var userID string
	var amount float64
	err := database.DB.QueryRow(ctx, query, auctionID).Scan(&userID, &amount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", 0, nil
		}
		return "", 0, err
	}
	return userID, amount, nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func queryAuctions(ctx context.Context, query string, args ...any) ([]model.Auction, error) {
	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var auctions []model.Auction
	for rows.Next() {
		var a model.Auction
		if err := rows.Scan(
			&a.ID, &a.SellerID, &a.Title, &a.Description, &a.CategoryID,
			&a.StartingPrice, &a.ReservePrice, &a.CurrentPrice,
			&a.State, &a.StartTime, &a.EndTime, &a.WinnerID, &a.TotalBids,
			&a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
		); err != nil {
			return nil, err
		}
		auctions = append(auctions, a)
	}
	return auctions, rows.Err()
}
