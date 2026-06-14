package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/auction-system/user-service/internal/model"
	"github.com/auction-system/user-service/internal/service"
	"github.com/auction-system/user-service/internal/utils"
)

const (
	ContextUserID   = "user_id"
	ContextUserRole = "user_role"
	ContextJTI      = "jti"
)

func AuthRequired(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			utils.Unauthorized(c, "missing bearer token")
			c.Abort()
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := authSvc.ParseToken(token)
		if err != nil {
			utils.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUserRole, string(claims.Role))
		c.Set(ContextJTI, claims.ID)
		c.Next()
	}
}

func RequireRole(roles ...model.Role) gin.HandlerFunc {
	allowed := make(map[model.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		roleStr, ok := c.Get(ContextUserRole)
		if !ok {
			utils.Forbidden(c, "insufficient permissions")
			c.Abort()
			return
		}
		if _, ok := allowed[model.Role(roleStr.(string))]; !ok {
			utils.Forbidden(c, "insufficient permissions")
			c.Abort()
			return
		}
		c.Next()
	}
}
