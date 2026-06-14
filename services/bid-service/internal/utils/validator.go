package utils

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// FormatValidationErrors converts validator errors into readable messages
func FormatValidationErrors(err error) string {
	var errs validator.ValidationErrors
	if ok := isValidationErrors(err, &errs); !ok {
		return err.Error()
	}

	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, formatFieldError(e))
	}
	return strings.Join(msgs, "; ")
}

func isValidationErrors(err error, target *validator.ValidationErrors) bool {
	if ve, ok := err.(validator.ValidationErrors); ok {
		*target = ve
		return true
	}
	return false
}

func formatFieldError(e validator.FieldError) string {
	field := strings.ToLower(e.Field())
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, e.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, e.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, e.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// RegisterCustomValidators registers custom validators with gin
func RegisterCustomValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// Example: register a custom "positive" validator
		v.RegisterValidation("positive", func(fl validator.FieldLevel) bool {
			return fl.Field().Float() > 0
		})
	}
}
