package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/auction-system/api-gateway/internal/auth"
	"github.com/auction-system/api-gateway/internal/observability"
)

func Auth(v *auth.Validator) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, token, err := v.ParseBearer(c.GetHeader("Authorization"))
		if err != nil {
			observability.JWTValidationErrors.Inc()
			c.AbortWithStatusJSON(401, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("token", token)
		c.Next()
	}
}
