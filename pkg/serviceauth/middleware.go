package serviceauth

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthFailureResponse is the standard error response for auth failures.
type AuthFailureResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// MiddlewareConfig holds configuration for the service auth middleware.
type MiddlewareConfig struct {
	// Validator is the token validator instance.
	Validator *TokenValidator

	// Logger is the structured logger for audit logging.
	Logger *zap.Logger

	// HeaderName is the HTTP header containing the service token.
	// Defaults to "X-Service-Token".
	HeaderName string

	// SkipPaths are paths that do not require service authentication (e.g., health checks).
	SkipPaths []string
}

// Middleware returns a Gin middleware that authenticates inter-service requests.
// It validates the service token, logs auth failures with source IP, service identity,
// and timestamp, and returns HTTP 403 on failure.
// When the auth subsystem is unavailable, it rejects ALL requests (fail-closed).
func Middleware(cfg *MiddlewareConfig) gin.HandlerFunc {
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "X-Service-Token"
	}

	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	skipPaths := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPaths[path] = true
	}

	return func(c *gin.Context) {
		// Skip paths that don't require auth (e.g., health/ready)
		if skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Extract token from header
		tokenString := c.GetHeader(headerName)
		if tokenString == "" {
			// Also check Authorization: Bearer <token> format
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenString == "" {
			logAuthFailure(logger, c, "", "missing service token")
			c.AbortWithStatusJSON(http.StatusForbidden, AuthFailureResponse{
				Success: false,
				Error:   "missing service authentication token",
			})
			return
		}

		// Validate the token
		claims, err := cfg.Validator.ValidateToken(tokenString)
		if err != nil {
			serviceID := ""
			if claims != nil {
				serviceID = claims.ServiceID
			}

			logAuthFailure(logger, c, serviceID, err.Error())

			// Determine error message
			errMsg := "service authentication failed"
			if err == ErrAuthUnavailable {
				errMsg = "authentication subsystem unavailable"
			}

			c.AbortWithStatusJSON(http.StatusForbidden, AuthFailureResponse{
				Success: false,
				Error:   errMsg,
			})
			return
		}

		// Set service identity in context for downstream use
		c.Set("service_id", claims.ServiceID)
		c.Set("service_token_id", claims.ID)
		c.Next()
	}
}

// logAuthFailure logs an authentication failure with required audit fields:
// source IP, service identity, and timestamp.
func logAuthFailure(logger *zap.Logger, c *gin.Context, serviceID, reason string) {
	logger.Warn("service auth failure",
		zap.String("source_ip", c.ClientIP()),
		zap.String("service_id", serviceID),
		zap.Time("timestamp", time.Now()),
		zap.String("reason", reason),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
	)
}
