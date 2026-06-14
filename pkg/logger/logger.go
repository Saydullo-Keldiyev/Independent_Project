// Package logger provides a shared structured logger for all services.
// All services import this instead of initializing their own zap logger.
package logger

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxKey string

const fieldsKey ctxKey = "log_fields"

// Log is the global logger — initialized once via Init()
var Log *zap.Logger

// Init initializes the global logger. Call once in main().
func Init(env string) error {
	var cfg zap.Config

	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := cfg.Build(zap.Fields(
		zap.Int("pid", os.Getpid()),
	))
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	Log = logger
	zap.ReplaceGlobals(logger)
	return nil
}

// With returns a logger with extra fields — use for request-scoped logging
func With(fields ...zap.Field) *zap.Logger {
	if Log == nil {
		l, _ := zap.NewProduction()
		return l.With(fields...)
	}
	return Log.With(fields...)
}

// FromCtx returns a logger enriched with fields stored in context
func FromCtx(ctx context.Context) *zap.Logger {
	if Log == nil {
		l, _ := zap.NewProduction()
		return l
	}
	if fields := fieldsFromCtx(ctx); len(fields) > 0 {
		return Log.With(fields...)
	}
	return Log
}

// WithCtx stores log fields in context for downstream use
func WithCtx(ctx context.Context, fields ...zap.Field) context.Context {
	existing := fieldsFromCtx(ctx)
	return context.WithValue(ctx, fieldsKey, append(existing, fields...))
}

func fieldsFromCtx(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(fieldsKey).([]zap.Field)
	return v
}

// Sync flushes buffered log entries — defer this in main()
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}
