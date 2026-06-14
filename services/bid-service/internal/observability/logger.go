package observability

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log is the global structured logger — use this everywhere instead of fmt/log
var Log *zap.Logger

// InitLogger initializes the global zap logger.
// In production: JSON format, info level.
// In development: colored console, debug level.
func InitLogger(env string) error {
	var cfg zap.Config

	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := cfg.Build(
		zap.Fields(
			zap.String("service", "bid-service"),
			zap.Int("pid", os.Getpid()),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	Log = logger
	// Also redirect stdlib log to zap
	zap.ReplaceGlobals(logger)
	return nil
}

// Sync flushes any buffered log entries — call on shutdown
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

// ── Context-aware logging helpers ────────────────────────────────────────────
// These extract correlation_id and other fields from context automatically.

type ctxKey string

const logFieldsKey ctxKey = "log_fields"

// WithFields returns a context carrying extra log fields
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	existing := fieldsFromCtx(ctx)
	return context.WithValue(ctx, logFieldsKey, append(existing, fields...))
}

// FromCtx returns a logger enriched with fields stored in the context
func FromCtx(ctx context.Context) *zap.Logger {
	if Log == nil {
		l, _ := zap.NewProduction()
		return l
	}
	fields := fieldsFromCtx(ctx)
	if len(fields) == 0 {
		return Log
	}
	return Log.With(fields...)
}

func fieldsFromCtx(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(logFieldsKey)
	if v == nil {
		return nil
	}
	fields, _ := v.([]zap.Field)
	return fields
}
