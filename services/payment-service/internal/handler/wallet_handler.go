package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/auction-system/payment-service/internal/service"
)

type WalletHandler struct {
	svc *service.WalletService
}

func NewWalletHandler(svc *service.WalletService) *WalletHandler {
	return &WalletHandler{svc: svc}
}

func (h *WalletHandler) GetWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	wallet, err := h.svc.GetWallet(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":                wallet.ID,
			"available_balance": wallet.AvailableBalance,
			"held_balance":      wallet.HeldBalance,
			"total_balance":     wallet.TotalBalance(),
			"currency":          wallet.Currency,
		},
	})
}

type depositRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

func (h *WalletHandler) Deposit(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req depositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate idempotency key from header or auto-generate
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	txn, err := h.svc.Deposit(c.Request.Context(), userID, req.Amount, idempotencyKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "deposit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": txn})
}

type holdRequest struct {
	AuctionID string  `json:"auction_id" binding:"required"`
	Amount    float64 `json:"amount"     binding:"required,gt=0"`
}

func (h *WalletHandler) HoldBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req holdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = "hold:" + req.AuctionID + ":" + userID
	}

	err := h.svc.HoldBalance(c.Request.Context(), userID, req.AuctionID, req.Amount, idempotencyKey)
	if err != nil {
		switch err {
		case service.ErrInsufficientBalance:
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient balance"})
		case service.ErrWalletNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "hold failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "balance held"})
}

func (h *WalletHandler) GetHistory(c *gin.Context) {
	userID := c.GetString("user_id")
	wallet, err := h.svc.GetWallet(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
		return
	}

	txns, err := h.svc.GetTransactions(c.Request.Context(), wallet.ID, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": txns})
}

type settleRequest struct {
	AuctionID string `json:"auction_id" binding:"required"`
	WinnerID  string `json:"winner_id"  binding:"required"`
}

func (h *WalletHandler) SettleAuction(c *gin.Context) {
	var req settleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.SettleAuction(c.Request.Context(), req.AuctionID, req.WinnerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settlement failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "auction settled"})
}
