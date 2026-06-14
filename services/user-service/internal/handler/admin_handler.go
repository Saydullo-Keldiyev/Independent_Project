package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/user-service/internal/model"
	"github.com/auction-system/user-service/internal/repository"
	"github.com/auction-system/user-service/internal/utils"
)

type AdminHandler struct {
	repo *repository.AdminRepository
}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{repo: repository.NewAdminRepository()}
}

// ListUsers returns paginated user list with filters
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	role := c.Query("role")
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	users, total, err := h.repo.ListUsers(c.Request.Context(), page, limit, role, search)
	if err != nil {
		utils.Internal(c, "failed to list users")
		return
	}

	// Map to response (exclude password_hash)
	type UserItem struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		Email      string `json:"email"`
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		Role       string `json:"role"`
		IsVerified bool   `json:"is_verified"`
		IsActive   bool   `json:"is_active"`
		CreatedAt  string `json:"created_at"`
	}

	items := make([]UserItem, 0, len(users))
	for _, u := range users {
		items = append(items, UserItem{
			ID:         u.ID,
			Username:   u.Username,
			Email:      u.Email,
			FirstName:  u.FirstName,
			LastName:   u.LastName,
			Role:       string(u.Role),
			IsVerified: u.IsVerified,
			IsActive:   u.IsActive,
			CreatedAt:  u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	utils.OK(c, gin.H{
		"users": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// UpdateRole changes a user's role
func (h *AdminHandler) UpdateRole(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Role string `json:"role" binding:"required,oneof=admin seller bidder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if err := h.repo.UpdateRole(c.Request.Context(), userID, model.Role(req.Role)); err != nil {
		utils.Internal(c, "failed to update role")
		return
	}

	utils.OK(c, gin.H{"message": "role updated", "user_id": userID, "new_role": req.Role})
}

// BanUser deactivates a user
func (h *AdminHandler) BanUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.repo.SetActive(c.Request.Context(), userID, false); err != nil {
		utils.Internal(c, "failed to ban user")
		return
	}
	utils.OK(c, gin.H{"message": "user banned", "user_id": userID})
}

// UnbanUser reactivates a user
func (h *AdminHandler) UnbanUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.repo.SetActive(c.Request.Context(), userID, true); err != nil {
		utils.Internal(c, "failed to unban user")
		return
	}
	utils.OK(c, gin.H{"message": "user unbanned", "user_id": userID})
}

// DeleteUser soft-deletes a user
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if err := h.repo.SoftDelete(c.Request.Context(), userID); err != nil {
		utils.Internal(c, "failed to delete user")
		return
	}
	utils.OK(c, gin.H{"message": "user deleted", "user_id": userID})
}
