package validation

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PayloadSizeLimitMiddleware returns a Gin middleware that rejects
// request payloads exceeding MaxPayloadSize (1MB) with HTTP 413.
func PayloadSizeLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > MaxPayloadSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "PAYLOAD_TOO_LARGE",
					"message": "Request payload exceeds the maximum allowed size of 1MB",
				},
			})
			return
		}

		// Also limit the reader to prevent chunked transfers exceeding the limit
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxPayloadSize)
		c.Next()
	}
}

// ValidationErrorResponse represents the standard error response for validation failures.
type ValidationErrorResponse struct {
	Success bool            `json:"success"`
	Error   ValidationError `json:"error"`
}

// ValidationError wraps validation error details.
type ValidationError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// RespondWithValidationError sends a structured HTTP 400 response with field-level errors.
func RespondWithValidationError(c *gin.Context, errors []FieldError) {
	c.AbortWithStatusJSON(http.StatusBadRequest, ValidationErrorResponse{
		Success: false,
		Error: ValidationError{
			Code:    "VALIDATION_ERROR",
			Message: "Request validation failed",
			Details: errors,
		},
	})
}

// RespondWithPayloadTooLarge sends an HTTP 413 response.
func RespondWithPayloadTooLarge(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, ValidationErrorResponse{
		Success: false,
		Error: ValidationError{
			Code:    "PAYLOAD_TOO_LARGE",
			Message: "Request payload exceeds the maximum allowed size of 1MB",
		},
	})
}

// RespondWithInvalidParam sends an HTTP 400 response for a malformed parameter.
func RespondWithInvalidParam(c *gin.Context, fieldErr *FieldError) {
	c.AbortWithStatusJSON(http.StatusBadRequest, ValidationErrorResponse{
		Success: false,
		Error: ValidationError{
			Code:    "INVALID_PARAMETER",
			Message: "Invalid request parameter",
			Details: []FieldError{*fieldErr},
		},
	})
}
