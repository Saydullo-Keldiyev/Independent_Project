package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/auction-system/user-service/internal/model"
)

type Claims struct {
	UserID   string      `json:"user_id"`
	Username string      `json:"username"`
	Email    string      `json:"email"`
	Role     model.Role  `json:"role"`
	jwt.RegisteredClaims
}
