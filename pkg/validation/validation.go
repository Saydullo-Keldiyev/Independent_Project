// Package validation provides input validation and sanitization for the auction system.
// It validates struct fields against DTO struct tags, sanitizes HTML special characters,
// validates UUID v4 format, enforces size/length limits, and validates bid amounts.
package validation

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// MaxPayloadSize is the maximum allowed request payload size (1MB).
const MaxPayloadSize = 1 * 1024 * 1024

// DefaultMaxTextLength is the default maximum length for free-text fields.
const DefaultMaxTextLength = 1000

// MaxBidAmount is the maximum allowed bid amount.
const MaxBidAmount = 999_999_999.99

// uuidV4Regex matches UUID v4 format strings.
var uuidV4Regex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

// FieldError represents a validation error for a specific field.
type FieldError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// Error implements the error interface for FieldError.
func (fe FieldError) Error() string {
	return fmt.Sprintf("field '%s': %s", fe.Field, fe.Message)
}

// Validator defines the interface for input validation and sanitization.
type Validator interface {
	// ValidateStruct validates a struct using its field tags and returns any errors.
	ValidateStruct(s interface{}) []FieldError
	// SanitizeString escapes HTML special characters in the input string.
	SanitizeString(input string) string
	// ValidateUUID checks whether a string is a valid UUID v4.
	ValidateUUID(value string) bool
	// ValidatePayloadSize checks if payload size exceeds MaxPayloadSize.
	// Returns true if the size is within the limit.
	ValidatePayloadSize(size int64) bool
	// ValidateTextLength checks if text exceeds the given max length.
	// If maxLength is 0, DefaultMaxTextLength is used.
	// Returns true if the length is within the limit.
	ValidateTextLength(text string, maxLength int) bool
	// ValidateBidAmount checks if a bid amount is valid:
	// positive, max 2 decimal places, and ≤ MaxBidAmount.
	ValidateBidAmount(amount float64) []FieldError
	// ValidateQueryParam validates a single query/path parameter value
	// against a specified validation rule.
	ValidateQueryParam(name, value, rule string) *FieldError
}

// validator wraps go-playground/validator with custom rules.
type validatorImpl struct {
	validate *validator.Validate
	mu       sync.RWMutex
}

// New creates a new Validator instance with custom validation rules registered.
func New() Validator {
	v := validator.New()

	// Register custom UUID v4 validation tag
	v.RegisterValidation("uuid4", func(fl validator.FieldLevel) bool {
		return uuidV4Regex.MatchString(fl.Field().String())
	})

	// Register custom bid_amount validation tag
	v.RegisterValidation("bid_amount", func(fl validator.FieldLevel) bool {
		amount := fl.Field().Float()
		if amount <= 0 {
			return false
		}
		if amount > MaxBidAmount {
			return false
		}
		if !hasMaxTwoDecimalPlaces(amount) {
			return false
		}
		return true
	})

	// Register custom max_text validation tag (uses param for custom limit)
	v.RegisterValidation("max_text", func(fl validator.FieldLevel) bool {
		text := fl.Field().String()
		maxLen := DefaultMaxTextLength
		if fl.Param() != "" {
			fmt.Sscanf(fl.Param(), "%d", &maxLen)
		}
		return len(text) <= maxLen
	})

	return &validatorImpl{
		validate: v,
	}
}

// ValidateStruct validates a struct using its field tags.
func (vi *validatorImpl) ValidateStruct(s interface{}) []FieldError {
	vi.mu.RLock()
	defer vi.mu.RUnlock()

	err := vi.validate.Struct(s)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []FieldError{
			{Field: "unknown", Message: err.Error()},
		}
	}

	fieldErrors := make([]FieldError, 0, len(validationErrors))
	for _, ve := range validationErrors {
		fieldErrors = append(fieldErrors, FieldError{
			Field:   formatFieldName(ve.Field()),
			Message: buildErrorMessage(ve),
			Value:   ve.Value(),
		})
	}
	return fieldErrors
}

// SanitizeString escapes HTML special characters: & < > " '
func (vi *validatorImpl) SanitizeString(input string) string {
	// html.EscapeString handles &, <, >, "
	// We additionally handle single quotes (') -> &#39;
	escaped := html.EscapeString(input)
	escaped = strings.ReplaceAll(escaped, "'", "&#39;")
	return escaped
}

// ValidateUUID checks if the value matches UUID v4 format.
func (vi *validatorImpl) ValidateUUID(value string) bool {
	return uuidV4Regex.MatchString(value)
}

// ValidatePayloadSize returns true if size is within the 1MB limit.
func (vi *validatorImpl) ValidatePayloadSize(size int64) bool {
	return size <= MaxPayloadSize
}

// ValidateTextLength returns true if text length is within the specified limit.
// If maxLength is 0, DefaultMaxTextLength (1000) is used.
func (vi *validatorImpl) ValidateTextLength(text string, maxLength int) bool {
	if maxLength <= 0 {
		maxLength = DefaultMaxTextLength
	}
	return len(text) <= maxLength
}

// ValidateBidAmount checks that a bid amount is positive, has max 2 decimal places,
// and does not exceed MaxBidAmount (999,999,999.99).
func (vi *validatorImpl) ValidateBidAmount(amount float64) []FieldError {
	var errs []FieldError

	if amount <= 0 {
		errs = append(errs, FieldError{
			Field:   "amount",
			Message: "must be positive",
			Value:   amount,
		})
		return errs
	}

	if amount > MaxBidAmount {
		errs = append(errs, FieldError{
			Field:   "amount",
			Message: fmt.Sprintf("must not exceed %.2f", MaxBidAmount),
			Value:   amount,
		})
	}

	if !hasMaxTwoDecimalPlaces(amount) {
		errs = append(errs, FieldError{
			Field:   "amount",
			Message: "must have at most 2 decimal places",
			Value:   amount,
		})
	}

	return errs
}

// ValidateQueryParam validates a single query or path parameter.
// Supported rules: "uuid4", "required", "numeric", "max_text".
func (vi *validatorImpl) ValidateQueryParam(name, value, rule string) *FieldError {
	switch rule {
	case "uuid4":
		if !vi.ValidateUUID(value) {
			return &FieldError{
				Field:   name,
				Message: "must be a valid UUID v4 format",
				Value:   value,
			}
		}
	case "required":
		if strings.TrimSpace(value) == "" {
			return &FieldError{
				Field:   name,
				Message: "is required",
				Value:   value,
			}
		}
	case "numeric":
		dotCount := 0
		dashCount := 0
		for i, ch := range value {
			if ch == '.' {
				dotCount++
				if dotCount > 1 {
					return &FieldError{
						Field:   name,
						Message: "must be a valid number",
						Value:   value,
					}
				}
			} else if ch == '-' {
				dashCount++
				if dashCount > 1 || i != 0 {
					return &FieldError{
						Field:   name,
						Message: "must be a valid number",
						Value:   value,
					}
				}
			} else if ch < '0' || ch > '9' {
				return &FieldError{
					Field:   name,
					Message: "must be a valid number",
					Value:   value,
				}
			}
		}
	case "max_text":
		if !vi.ValidateTextLength(value, 0) {
			return &FieldError{
				Field:   name,
				Message: fmt.Sprintf("must not exceed %d characters", DefaultMaxTextLength),
				Value:   value,
			}
		}
	default:
		// Unknown rule — no validation applied
	}
	return nil
}

// hasMaxTwoDecimalPlaces checks that a float64 has at most 2 decimal places.
func hasMaxTwoDecimalPlaces(amount float64) bool {
	// Multiply by 100, round, and check if the result equals the original * 100
	rounded := math.Round(amount*100) / 100
	return math.Abs(amount-rounded) < 1e-9
}

// formatFieldName converts a struct field name to a lowercase JSON-friendly name.
func formatFieldName(field string) string {
	if len(field) == 0 {
		return field
	}
	// Convert first letter to lowercase for JSON convention
	result := strings.ToLower(field[:1]) + field[1:]
	return result
}

// buildErrorMessage creates a user-friendly error message from a validator.FieldError.
func buildErrorMessage(ve validator.FieldError) string {
	switch ve.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", ve.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", ve.Param())
	case "email":
		return "must be a valid email address"
	case "uuid4":
		return "must be a valid UUID v4 format"
	case "bid_amount":
		return "must be a positive number with max 2 decimal places, not exceeding 999,999,999.99"
	case "max_text":
		limit := ve.Param()
		if limit == "" {
			limit = fmt.Sprintf("%d", DefaultMaxTextLength)
		}
		return fmt.Sprintf("must not exceed %s characters", limit)
	case "gt":
		return fmt.Sprintf("must be greater than %s", ve.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", ve.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", ve.Param())
	case "url":
		return "must be a valid URL"
	case "len":
		return fmt.Sprintf("must be exactly %s characters", ve.Param())
	default:
		return fmt.Sprintf("failed on '%s' validation", ve.Tag())
	}
}
