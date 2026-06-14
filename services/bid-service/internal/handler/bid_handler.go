package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/bid-service/internal/dto"
	redisPkg "github.com/auction-system/bid-service/internal/redis"
	"github.com/auction-system/bid-service/internal/repository"
	"github.com/auction-system/bid-service/internal/service"
	"github.com/auction-system/bid-service/internal/utils"
)

// PlaceBid handles POST /bids
func PlaceBid(c *gin.Context) {
	var req dto.CreateBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, utils.FormatValidationErrors(err))
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		utils.Unauthorized(c, "user not authenticated")
		return
	}

	resp, err := service.PlaceBid(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAuctionNotFound):
			utils.NotFound(c, err.Error())
		case errors.Is(err, service.ErrAuctionNotActive):
			utils.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrBidTooLow):
			utils.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrAuctionLocked):
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   err.Error(),
			})
		case errors.Is(err, service.ErrSellerCannotBid):
			utils.BadRequest(c, err.Error())
		default:
			utils.InternalError(c, "failed to place bid")
		}
		return
	}

	utils.Created(c, resp)
}

// GetBidsByAuction handles GET /auctions/:auction_id/bids
func GetBidsByAuction(c *gin.Context) {
	auctionID := c.Param("auction_id")
	if auctionID == "" {
		utils.BadRequest(c, "auction_id is required")
		return
	}

	bids, err := service.GetBidsByAuction(c.Request.Context(), auctionID)
	if err != nil {
		utils.InternalError(c, "failed to get bids")
		return
	}

	utils.OK(c, dto.BidListResponse{
		Bids:  bids,
		Total: len(bids),
	})
}

// GetMyBids handles GET /bids/me
func GetMyBids(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		utils.Unauthorized(c, "user not authenticated")
		return
	}

	bids, err := service.GetBidsByUser(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "failed to get bids")
		return
	}

	utils.OK(c, dto.BidListResponse{
		Bids:  bids,
		Total: len(bids),
	})
}

// GetBidHistory handles GET /auctions/:auction_id/bid-history — full bid timeline
func GetBidHistory(c *gin.Context) {
	auctionID := c.Param("auction_id")
	if auctionID == "" {
		utils.BadRequest(c, "auction_id is required")
		return
	}

	bids, err := service.GetBidsByAuction(c.Request.Context(), auctionID)
	if err != nil {
		utils.InternalError(c, "failed to get bid history")
		return
	}

	utils.OK(c, gin.H{
		"auction_id": auctionID,
		"bids":       bids,
		"total":      len(bids),
	})
}

// GetMinBid handles GET /auctions/:auction_id/min-bid — returns minimum valid bid amount
func GetMinBid(c *gin.Context) {
	auctionID := c.Param("auction_id")
	if auctionID == "" {
		utils.BadRequest(c, "auction_id is required")
		return
	}

	// Get current highest bid from Redis or DB
	highest, _ := redisPkg.GetHighestBid(auctionID)
	if highest == 0 {
		// Fallback: get from DB
		highest, _ = repository.GetHighestBid(c.Request.Context(), auctionID)
	}

	// Minimum bid = current highest + 1 (or starting price if no bids)
	minBid := highest + 1.0
	if highest == 0 {
		minBid = 1.0 // will be overridden by auction starting price
	}

	utils.OK(c, gin.H{
		"auction_id":  auctionID,
		"current_bid": highest,
		"min_bid":     minBid,
	})
}
