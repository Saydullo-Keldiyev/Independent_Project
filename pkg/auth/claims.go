package auth

import "github.com/golang-jwt/jwt/v5"

// Claims is the JWT payload shared across all services
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}
