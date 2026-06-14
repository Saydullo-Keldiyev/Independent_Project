package model

import "time"

// DailyMetric — aggregated platform-wide metrics per day
type DailyMetric struct {
	ID             string    `db:"id"              json:"id"`
	MetricDate     time.Time `db:"metric_date"     json:"metric_date"`
	TotalRevenue   float64   `db:"total_revenue"   json:"total_revenue"`
	TotalBids      int64     `db:"total_bids"      json:"total_bids"`
	TotalAuctions  int64     `db:"total_auctions"  json:"total_auctions"`
	ActiveUsers    int64     `db:"active_users"    json:"active_users"`
	NewUsers       int64     `db:"new_users"       json:"new_users"`
	CompletedAuctions int64  `db:"completed_auctions" json:"completed_auctions"`
	CreatedAt      time.Time `db:"created_at"      json:"created_at"`
}

// AuctionMetric — per-auction analytics
type AuctionMetric struct {
	AuctionID     string    `db:"auction_id"      json:"auction_id"`
	TotalBids     int64     `db:"total_bids"      json:"total_bids"`
	HighestBid    float64   `db:"highest_bid"     json:"highest_bid"`
	UniqueBidders int64     `db:"unique_bidders"  json:"unique_bidders"`
	WatchCount    int64     `db:"watch_count"     json:"watch_count"`
	UpdatedAt     time.Time `db:"updated_at"      json:"updated_at"`
}

// RevenueMetric — per-seller revenue tracking
type RevenueMetric struct {
	ID           string    `db:"id"             json:"id"`
	SellerID     string    `db:"seller_id"      json:"seller_id"`
	AuctionID    string    `db:"auction_id"     json:"auction_id"`
	GrossRevenue float64   `db:"gross_revenue"  json:"gross_revenue"`
	PlatformFee  float64   `db:"platform_fee"   json:"platform_fee"`
	NetRevenue   float64   `db:"net_revenue"    json:"net_revenue"`
	CreatedAt    time.Time `db:"created_at"     json:"created_at"`
}

// DashboardStats — aggregated stats for admin dashboard
type DashboardStats struct {
	TotalRevenue      float64 `json:"total_revenue"`
	TodayRevenue      float64 `json:"today_revenue"`
	ActiveAuctions    int64   `json:"active_auctions"`
	TotalUsers        int64   `json:"total_users"`
	TodayBids         int64   `json:"today_bids"`
	BidsPerMinute     float64 `json:"bids_per_minute"`
	ConcurrentUsers   int64   `json:"concurrent_users"`
	ConversionRate    float64 `json:"conversion_rate"`
	AvgAuctionPrice   float64 `json:"avg_auction_price"`
	PlatformGrowth    float64 `json:"platform_growth_pct"`
}

// SellerStats — seller-specific analytics
type SellerStats struct {
	SellerID        string  `json:"seller_id"`
	TotalAuctions   int64   `json:"total_auctions"`
	ActiveAuctions  int64   `json:"active_auctions"`
	TotalRevenue    float64 `json:"total_revenue"`
	AvgBidsPerAuction float64 `json:"avg_bids_per_auction"`
	SuccessRate     float64 `json:"success_rate"`
	TopCategory     string  `json:"top_category"`
}

// TimeSeriesPoint — for charts
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}
