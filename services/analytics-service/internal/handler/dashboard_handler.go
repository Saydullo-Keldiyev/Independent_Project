package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/analytics-service/internal/cache"
	"github.com/auction-system/analytics-service/internal/model"
	"github.com/auction-system/analytics-service/internal/repository"
)

type DashboardHandler struct {
	repo *repository.AnalyticsRepository
}

func NewDashboardHandler(repo *repository.AnalyticsRepository) *DashboardHandler {
	return &DashboardHandler{repo: repo}
}

// AdminDashboard returns platform-wide KPIs
func (h *DashboardHandler) AdminDashboard(c *gin.Context) {
	ctx := c.Request.Context()

	totalRevenue, _ := h.repo.GetTotalRevenue(ctx)

	stats := model.DashboardStats{
		TotalRevenue:    totalRevenue,
		TodayRevenue:    cache.GetRevenueToday(),
		TodayBids:       cache.GetBidsToday(),
		ConcurrentUsers: cache.GetActiveUsersToday(),
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// Revenue returns revenue metrics for a date range
func (h *DashboardHandler) Revenue(c *gin.Context) {
	ctx := c.Request.Context()

	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()

	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t
		}
	}

	metrics, err := h.repo.GetDailyMetrics(ctx, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metrics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": metrics})
}

// SellerDashboard returns seller-specific analytics
func (h *DashboardHandler) SellerDashboard(c *gin.Context) {
	sellerID := c.Param("id")
	if sellerID == "" {
		sellerID = c.GetString("user_id")
	}

	stats, err := h.repo.GetSellerStats(c.Request.Context(), sellerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// Trending returns leaderboards
func (h *DashboardHandler) Trending(c *gin.Context) {
	sellers, _ := cache.GetTopSellers(10)
	bidders, _ := cache.GetTopBidders(10)
	categories, _ := cache.GetTopCategories(10)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"top_sellers":    sellers,
			"top_bidders":    bidders,
			"top_categories": categories,
		},
	})
}

// AuctionAnalytics returns metrics for a specific auction
func (h *DashboardHandler) AuctionAnalytics(c *gin.Context) {
	auctionID := c.Param("id")

	metric, err := h.repo.GetAuctionMetric(c.Request.Context(), auctionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auction metrics not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": metric})
}
