// Package errors provides typed domain errors shared across all services.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNotFound      = errors.New("not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
	ErrBadRequest    = errors.New("bad request")
	ErrInternal      = errors.New("internal server error")
	ErrTimeout       = errors.New("request timeout")
	ErrServiceUnavailable = errors.New("service unavailable")
)

// ── AppError — structured error with HTTP status ──────────────────────────────

type AppError struct {
	Code    int    // HTTP status code
	Message string // user-facing message
	Err     error  // underlying error (for logging)
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

// New creates a new AppError
func New(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// Convenience constructors
func NotFound(msg string) *AppError {
	return New(http.StatusNotFound, msg, ErrNotFound)
}

func Unauthorized(msg string) *AppError {
	return New(http.StatusUnauthorized, msg, ErrUnauthorized)
}

func Forbidden(msg string) *AppError {
	return New(http.StatusForbidden, msg, ErrForbidden)
}

func BadRequest(msg string) *AppError {
	return New(http.StatusBadRequest, msg, ErrBadRequest)
}

func Internal(msg string, err error) *AppError {
	return New(http.StatusInternalServerError, msg, err)
}

func Conflict(msg string) *AppError {
	return New(http.StatusConflict, msg, ErrConflict)
}

// HTTPStatus returns the HTTP status code for a given error.
// Falls back to 500 for unknown errors.
func HTTPStatus(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// UserMessage returns a safe user-facing message (no internal details)
func UserMessage(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "an unexpected error occurred"
}
