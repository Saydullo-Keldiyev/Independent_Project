package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/auction-system/api-gateway/internal/auth"
)

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	set := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		set[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role := c.GetString("role")
		if _, ok := set[role]; !ok {
			c.AbortWithStatusJSON(403, gin.H{
				"success": false,
				"error":   "forbidden: insufficient permissions",
			})
			return
		}
		c.Next()
	}
}

func RequireSellerOrAdmin() gin.HandlerFunc {
	return RequireRole(auth.RoleSeller, auth.RoleAdmin)
}

func RequireAdmin() gin.HandlerFunc {
	return RequireRole(auth.RoleAdmin)
}
