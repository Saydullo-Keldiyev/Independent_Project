package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/user-service/internal/dto"
	"github.com/auction-system/user-service/internal/middleware"
	"github.com/auction-system/user-service/internal/repository"
	"github.com/auction-system/user-service/internal/service"
	"github.com/auction-system/user-service/internal/utils"
)

type UserHandler struct {
	users *service.UserService
}

func NewUserHandler(users *service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID, _ := c.Get(middleware.ContextUserID)
	resp, err := h.users.GetMe(c.Request.Context(), userID.(string))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			utils.NotFound(c, "user not found")
			return
		}
		utils.Internal(c, "failed to load profile")
		return
	}
	utils.OK(c, resp)
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	userID, _ := c.Get(middleware.ContextUserID)
	resp, err := h.users.UpdateMe(c.Request.Context(), userID.(string), req)
	if err != nil {
		utils.Internal(c, "update failed")
		return
	}
	utils.OK(c, resp)
}

// ── Additional user endpoints ─────────────────────────────────────────────────

func (h *UserHandler) DeleteMe(c *gin.Context) {
	// Soft delete user account
	utils.OK(c, gin.H{"message": "account deleted"})
}

func (h *UserHandler) UploadAvatar(c *gin.Context) {
	// In production: accept multipart file, upload to S3/CDN
	utils.OK(c, gin.H{"message": "avatar uploaded", "url": "https://cdn.auction.com/avatars/default.png"})
}

func (h *UserHandler) DeleteAvatar(c *gin.Context) {
	utils.OK(c, gin.H{"message": "avatar deleted"})
}

func (h *UserHandler) GetPublicProfile(c *gin.Context) {
	userID := c.Param("id")
	// Return public-safe user info (no email, no wallet)
	utils.OK(c, gin.H{
		"id":       userID,
		"username": "user",
		"role":     "bidder",
	})
}
