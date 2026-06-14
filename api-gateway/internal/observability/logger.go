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
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
	}
	l, err := cfg.Build(zap.Fields(zap.String("service", "api-gateway"), zap.Int("pid", os.Getpid())))
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	Log = l
	return nil
}

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}
