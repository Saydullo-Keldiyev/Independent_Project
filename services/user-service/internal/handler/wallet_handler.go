package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/user-service/internal/dto"
	"github.com/auction-system/user-service/internal/middleware"
	"github.com/auction-system/user-service/internal/repository"
	"github.com/auction-system/user-service/internal/service"
	"github.com/auction-system/user-service/internal/utils"
)

type WalletHandler struct {
	wallet *service.WalletService
}

func NewWalletHandler(wallet *service.WalletService) *WalletHandler {
	return &WalletHandler{wallet: wallet}
}

func (h *WalletHandler) GetWallet(c *gin.Context) {
	userID, _ := c.Get(middleware.ContextUserID)
	resp, err := h.wallet.GetWallet(c.Request.Context(), userID.(string))
	if err != nil {
		utils.NotFound(c, "wallet not found")
		return
	}
	utils.OK(c, resp)
}

func (h *WalletHandler) History(c *gin.Context) {
	userID, _ := c.Get(middleware.ContextUserID)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	resp, err := h.wallet.History(c.Request.Context(), userID.(string), limit, offset)
	if err != nil {
		utils.Internal(c, "failed to load history")
		return
	}
	utils.OK(c, resp)
}

func (h *WalletHandler) Deposit(c *gin.Context) {
	var req dto.DepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	userID, _ := c.Get(middleware.ContextUserID)
	resp, err := h.wallet.Deposit(c.Request.Context(), userID.(string), req.Amount)
	if err != nil {
		if errors.Is(err, repository.ErrWalletNotFound) {
			utils.NotFound(c, "wallet not found")
			return
		}
		utils.Internal(c, "deposit failed")
		return
	}
	utils.OK(c, resp)
}

func (h *WalletHandler) Withdraw(c *gin.Context) {
	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.OK(c, gin.H{"message": "withdrawal initiated", "amount": req.Amount})
}

// ── Internal API (inter-service) ──────────────────────────────────────────────

func (h *WalletHandler) Hold(c *gin.Context) {
	var req struct {
		UserID string  `json:"user_id" binding:"required"`
		Amount float64 `json:"amount" binding:"required,gt=0"`
		Ref    string  `json:"ref" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.wallet.Hold(c.Request.Context(), req.UserID, req.Amount, req.Ref); err != nil {
		if errors.Is(err, repository.ErrInsufficientFunds) {
			utils.BadRequest(c, "insufficient balance")
			return
		}
		utils.Internal(c, "hold failed")
		return
	}
	utils.OK(c, gin.H{"message": "hold created", "amount": req.Amount})
}

func (h *WalletHandler) Release(c *gin.Context) {
	var req struct {
		UserID string  `json:"user_id" binding:"required"`
		Amount float64 `json:"amount" binding:"required,gt=0"`
		Ref    string  `json:"ref" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.wallet.ReleaseHold(c.Request.Context(), req.UserID, req.Amount, req.Ref); err != nil {
		utils.Internal(c, "release failed")
		return
	}
	utils.OK(c, gin.H{"message": "hold released", "amount": req.Amount})
}

func (h *WalletHandler) Settle(c *gin.Context) {
	var req struct {
		UserID    string  `json:"user_id" binding:"required"`
		Amount    float64 `json:"amount" binding:"required,gt=0"`
		AuctionID string  `json:"auction_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.wallet.Settle(c.Request.Context(), req.UserID, req.Amount, req.AuctionID); err != nil {
		utils.Internal(c, "settle failed")
		return
	}
	utils.OK(c, gin.H{"message": "settled", "amount": req.Amount})
}

func (h *WalletHandler) CreditSeller(c *gin.Context) {
	var req struct {
		UserID    string  `json:"user_id" binding:"required"`
		Amount    float64 `json:"amount" binding:"required,gt=0"`
		AuctionID string  `json:"auction_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.wallet.Credit(c.Request.Context(), req.UserID, req.Amount, req.AuctionID); err != nil {
		utils.Internal(c, "credit failed")
		return
	}
	utils.OK(c, gin.H{"message": "credited", "amount": req.Amount})
}
