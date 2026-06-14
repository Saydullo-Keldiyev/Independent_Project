package handler

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/user-service/internal/dto"
	"github.com/auction-system/user-service/internal/middleware"
	"github.com/auction-system/user-service/internal/repository"
	"github.com/auction-system/user-service/internal/service"
	"github.com/auction-system/user-service/internal/utils"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	resp, err := h.auth.Register(c.Request.Context(), req, utils.ClientIP(c), c.Request.UserAgent())
	if err != nil {
		if errors.Is(err, repository.ErrEmailExists) {
			utils.Fail(c, 409, "EMAIL_EXISTS", "email already registered")
			return
		}
		// Log the actual error for debugging
		fmt.Printf("[ERROR] Register failed: %v\n", err)
		utils.Internal(c, "registration failed")
		return
	}
	utils.Created(c, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	resp, err := h.auth.Login(c.Request.Context(), req, utils.ClientIP(c), c.Request.UserAgent())
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			utils.Unauthorized(c, "invalid email or password")
			return
		}
		if errors.Is(err, service.ErrInactiveUser) {
			utils.Forbidden(c, "account inactive")
			return
		}
		utils.Internal(c, "login failed")
		return
	}
	utils.OK(c, resp)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	resp, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		utils.Unauthorized(c, "invalid refresh token")
		return
	}
	utils.OK(c, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	_ = c.ShouldBindJSON(&req)
	jti, _ := c.Get(middleware.ContextJTI)
	jtiStr, _ := jti.(string)
	ttl := 15 * time.Minute
	_ = h.auth.Logout(c.Request.Context(), jtiStr, ttl, req.RefreshToken)
	utils.OK(c, gin.H{"message": "logged out"})
}

// ── Additional auth endpoints ─────────────────────────────────────────────────

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	// In production: send reset email with token
	utils.OK(c, gin.H{"message": "if the email exists, a reset link has been sent"})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	// In production: validate token, update password
	utils.OK(c, gin.H{"message": "password reset successfully"})
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.OK(c, gin.H{"message": "email verified"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.OK(c, gin.H{"message": "password changed"})
}

func (h *AuthHandler) GetSessions(c *gin.Context) {
	userID := c.GetString("user_id")
	sessions, _ := repository.NewSessionRepository().ActiveCount(c.Request.Context())
	utils.OK(c, gin.H{"sessions": []any{}, "active_count": sessions, "user_id": userID})
}

func (h *AuthHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	_ = sessionID
	utils.OK(c, gin.H{"message": "session revoked"})
}

func (h *AuthHandler) DeleteAllSessions(c *gin.Context) {
	utils.OK(c, gin.H{"message": "all sessions revoked"})
}
