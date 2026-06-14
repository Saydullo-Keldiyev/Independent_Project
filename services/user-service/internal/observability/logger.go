package observability

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

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
	logger, err := cfg.Build(zap.Fields(
		zap.String("service", "user-service"),
		zap.Int("pid", os.Getpid()),
	))
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	Log = logger
	zap.ReplaceGlobals(logger)
	return nil
}

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}
