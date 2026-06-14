package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/auction-system/analytics-service/internal/database"
	"github.com/auction-system/analytics-service/internal/model"
)

type AnalyticsRepository struct{}

func New() *AnalyticsRepository {
	return &AnalyticsRepository{}
}

// IncrAuctionBids updates bid count and highest bid for an auction
func (r *AnalyticsRepository) IncrAuctionBids(ctx context.Context, auctionID string, amount float64) {
	query := `
		INSERT INTO auction_metrics (auction_id, total_bids, highest_bid, unique_bidders, watch_count, updated_at)
		VALUES ($1, 1, $2, 1, 0, NOW())
		ON CONFLICT (auction_id) DO UPDATE SET
			total_bids = auction_metrics.total_bids + 1,
			highest_bid = GREATEST(auction_metrics.highest_bid, $2),
			updated_at = NOW()
	`
	database.DB.Exec(ctx, query, auctionID, amount)
}

// IncrDailyAuctions increments today's auction count
func (r *AnalyticsRepository) IncrDailyAuctions(ctx context.Context) {
	today := time.Now().Truncate(24 * time.Hour)
	query := `
		INSERT INTO daily_metrics (id, metric_date, total_auctions, total_bids, total_revenue, active_users, new_users, completed_auctions, created_at)
		VALUES ($1, $2, 1, 0, 0, 0, 0, 0, NOW())
		ON CONFLICT (metric_date) DO UPDATE SET
			total_auctions = daily_metrics.total_auctions + 1
	`
	database.DB.Exec(ctx, query, uuid.New().String(), today)
}

// IncrDailyNewUsers increments today's new user count
func (r *AnalyticsRepository) IncrDailyNewUsers(ctx context.Context) {
	today := time.Now().Truncate(24 * time.Hour)
	query := `
		INSERT INTO daily_metrics (id, metric_date, total_auctions, total_bids, total_revenue, active_users, new_users, completed_auctions, created_at)
		VALUES ($1, $2, 0, 0, 0, 0, 1, 0, NOW())
		ON CONFLICT (metric_date) DO UPDATE SET
			new_users = daily_metrics.new_users + 1
	`
	database.DB.Exec(ctx, query, uuid.New().String(), today)
}

// RecordRevenue inserts a revenue record
func (r *AnalyticsRepository) RecordRevenue(ctx context.Context, sellerID, auctionID string, gross, fee float64) {
	query := `
		INSERT INTO revenue_metrics (id, seller_id, auction_id, gross_revenue, platform_fee, net_revenue, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`
	database.DB.Exec(ctx, query, uuid.New().String(), sellerID, auctionID, gross, fee, gross-fee)
}

// GetDailyMetrics returns metrics for a date range
func (r *AnalyticsRepository) GetDailyMetrics(ctx context.Context, from, to time.Time) ([]model.DailyMetric, error) {
	query := `
		SELECT id, metric_date, total_revenue, total_bids, total_auctions, active_users, new_users, completed_auctions, created_at
		FROM daily_metrics
		WHERE metric_date BETWEEN $1 AND $2
		ORDER BY metric_date DESC
	`
	rows, err := database.DB.Query(ctx, query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []model.DailyMetric
	for rows.Next() {
		var m model.DailyMetric
		if err := rows.Scan(&m.ID, &m.MetricDate, &m.TotalRevenue, &m.TotalBids, &m.TotalAuctions, &m.ActiveUsers, &m.NewUsers, &m.CompletedAuctions, &m.CreatedAt); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// GetTotalRevenue returns total platform revenue
func (r *AnalyticsRepository) GetTotalRevenue(ctx context.Context) (float64, error) {
	var total float64
	err := database.DB.QueryRow(ctx, `SELECT COALESCE(SUM(platform_fee), 0) FROM revenue_metrics`).Scan(&total)
	return total, err
}

// GetSellerStats returns analytics for a specific seller
func (r *AnalyticsRepository) GetSellerStats(ctx context.Context, sellerID string) (*model.SellerStats, error) {
	var stats model.SellerStats
	stats.SellerID = sellerID

	// Total revenue
	database.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(net_revenue), 0) FROM revenue_metrics WHERE seller_id = $1`, sellerID,
	).Scan(&stats.TotalRevenue)

	// Auction count (from revenue records as proxy)
	database.DB.QueryRow(ctx,
		`SELECT COUNT(DISTINCT auction_id) FROM revenue_metrics WHERE seller_id = $1`, sellerID,
	).Scan(&stats.TotalAuctions)

	return &stats, nil
}

// GetAuctionMetric returns metrics for a specific auction
func (r *AnalyticsRepository) GetAuctionMetric(ctx context.Context, auctionID string) (*model.AuctionMetric, error) {
	query := `
		SELECT auction_id, total_bids, highest_bid, unique_bidders, watch_count, updated_at
		FROM auction_metrics WHERE auction_id = $1
	`
	var m model.AuctionMetric
	err := database.DB.QueryRow(ctx, query, auctionID).Scan(
		&m.AuctionID, &m.TotalBids, &m.HighestBid, &m.UniqueBidders, &m.WatchCount, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
