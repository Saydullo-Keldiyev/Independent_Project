package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/auction-system/bid-service/internal/config"
	"github.com/auction-system/bid-service/internal/utils"
)

// Claims is the JWT payload — must match what api-gateway issues
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates the JWT Bearer token.
// In production, bid-service sits behind api-gateway which already validates JWT.
// This middleware is a second layer of defense (defense-in-depth).
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Option 1: Trust api-gateway's injected headers (internal network only)
		// If X-User-ID is set, the request came through the gateway
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			c.Set("user_id", userID)
			c.Set("role", c.GetHeader("X-User-Role"))
			c.Set("email", c.GetHeader("X-User-Email"))
			c.Next()
			return
		}

		// Option 2: Direct JWT validation (for dev/testing without gateway)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.Unauthorized(c, "authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			utils.Unauthorized(c, "format must be: Bearer <token>")
			c.Abort()
			return
		}

		tokenStr := parts[1]
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(
			tokenStr,
			claims,
			func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(config.Cfg.JWT.Secret), nil
			},
		)

		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				utils.Unauthorized(c, "token has expired")
				c.Abort()
				return
			}
			utils.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		if !token.Valid {
			utils.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("token", tokenStr)
		c.Next()
	}
}

// RequireRole checks that the authenticated user has one of the allowed roles.
// Must be used after AuthMiddleware.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		roleSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role := c.GetString("role")
		if _, ok := roleSet[role]; !ok {
			c.AbortWithStatusJSON(403, gin.H{
				"success": false,
				"error":   "forbidden: insufficient permissions",
			})
			return
		}
		c.Next()
	}
}
