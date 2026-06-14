package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/auction-system/bid-service/internal/service"
	"github.com/auction-system/bid-service/internal/utils"
)

// ServeWS handles GET /ws/:auction_id
// Upgrades the connection to WebSocket for real-time bid updates
func ServeWS(c *gin.Context) {
	auctionID := c.Param("auction_id")
	if auctionID == "" {
		utils.BadRequest(c, "auction_id is required")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		utils.Unauthorized(c, "user not authenticated")
		return
	}

	if err := service.ServeWS(c.Writer, c.Request, auctionID, userID); err != nil {
		utils.InternalError(c, "failed to upgrade websocket connection")
		return
	}
}
