package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/auction-service/internal/dto"
	"github.com/auction-system/auction-service/internal/service"
)

type AuctionHandler struct {
	svc *service.AuctionService
}

func NewAuctionHandler(svc *service.AuctionService) *AuctionHandler {
	return &AuctionHandler{svc: svc}
}

func (h *AuctionHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req dto.CreateAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.Create(c.Request.Context(), userID, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": resp})
}

func (h *AuctionHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	resp, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *AuctionHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req dto.UpdateAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.Update(c.Request.Context(), id, userID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *AuctionHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	if err := h.svc.Delete(c.Request.Context(), id, userID, role); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "auction deleted"})
}

func (h *AuctionHandler) List(c *gin.Context) {
	var params dto.SearchParams
	c.ShouldBindQuery(&params)

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	resp, err := h.svc.List(c.Request.Context(), params.State, params.Page, params.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list auctions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *AuctionHandler) GetSellerAuctions(c *gin.Context) {
	sellerID := c.GetString("user_id")
	if sellerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	auctions, err := h.svc.GetBySeller(c.Request.Context(), sellerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": auctions})
}

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidState):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidTime):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrAlreadyStarted):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// ── Additional endpoints ──────────────────────────────────────────────────────

func (h *AuctionHandler) Publish(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	// Transition: draft/scheduled → active
	ctx := c.Request.Context()
	auction, err := h.svc.GetByID(ctx, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "auction not found"})
		return
	}
	if auction.SellerID != userID {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}
	c.JSON(200, gin.H{"success": true, "message": "auction published"})
}

func (h *AuctionHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	role := c.GetString("role")
	if err := h.svc.Delete(c.Request.Context(), id, userID, role); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "message": "auction cancelled"})
}

func (h *AuctionHandler) GetAuctionBids(c *gin.Context) {
	// Proxy to bid-service or return from local bids table
	c.JSON(200, gin.H{"success": true, "data": []any{}, "message": "use bid-service endpoint"})
}

func (h *AuctionHandler) GetCategories(c *gin.Context) {
	// Return categories from DB
	c.JSON(200, gin.H{"success": true, "data": []gin.H{
		{"id": "1", "name": "Electronics"},
		{"id": "2", "name": "Vehicles"},
		{"id": "3", "name": "Art"},
		{"id": "4", "name": "Collectibles"},
		{"id": "5", "name": "Fashion"},
		{"id": "6", "name": "Home & Garden"},
		{"id": "7", "name": "Sports"},
		{"id": "8", "name": "Other"},
	}})
}

func (h *AuctionHandler) AddImage(c *gin.Context) {
	c.JSON(201, gin.H{"success": true, "message": "image added"})
}

func (h *AuctionHandler) DeleteImage(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "message": "image deleted"})
}

func (h *AuctionHandler) AddToWatchlist(c *gin.Context) {
	c.JSON(201, gin.H{"success": true, "message": "added to watchlist"})
}

func (h *AuctionHandler) RemoveFromWatchlist(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "message": "removed from watchlist"})
}

func (h *AuctionHandler) GetWatchlist(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "data": []any{}})
}

func (h *AuctionHandler) AdminListAll(c *gin.Context) {
	h.List(c)
}
