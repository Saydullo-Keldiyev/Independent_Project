package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/auction-system/auction-service/internal/config"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			c.Set("user_id", userID)
			c.Set("role", c.GetHeader("X-User-Role"))
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid format"})
			return
		}

		claims := &jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (any, error) {
			return []byte(config.Cfg.JWT.Secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}

		if uid, ok := (*claims)["user_id"].(string); ok {
			c.Set("user_id", uid)
		}
		if role, ok := (*claims)["role"].(string); ok {
			c.Set("role", role)
		}
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	set := make(map[string]struct{})
	for _, r := range roles {
		set[r] = struct{}{}
	}
	return func(c *gin.Context) {
		if _, ok := set[c.GetString("role")]; !ok {
			c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
