package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/user-service/internal/database"
	redisPkg "github.com/auction-system/user-service/internal/redis"
	"github.com/auction-system/user-service/internal/repository"
)

type HealthHandler struct {
	sessions *repository.SessionRepository
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{sessions: repository.NewSessionRepository()}
}

func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "user-service"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := database.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "db": err.Error()})
		return
	}
	if err := redisPkg.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "redis": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
