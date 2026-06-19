// Package pkginit wires shared pkg/ packages into the notification-service.
// It initializes the structured logger, circuit breakers, Kafka consumer
// integration, and validation.
package pkginit

import (
	"github.com/auction-system/pkg/circuitbreaker"
	"github.com/auction-system/pkg/logger"
	"github.com/auction-system/pkg/validation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	ServiceName = "notification-service"
	Version     = "1.0.0"
)

// Services holds initialized shared package instances for the notification-service.
type Services struct {
	Logger          logger.Logger
	Validator       validation.Validator
	CircuitBreakers map[string]circuitbreaker.CircuitBreaker
}

// Config holds configuration needed by shared packages.
type Config struct {
	Environment string
}

// Init initializes all shared packages for the notification-service.
func Init(cfg Config) (*Services, error) {
	// 1. Initialize structured logger.
	log, err := logger.New(logger.LogConfig{
		ServiceName: ServiceName,
		Environment: cfg.Environment,
		Version:     Version,
	})
	if err != nil {
		return nil, err
	}

	// 2. Initialize validator.
	v := validation.New()

	// 3. Initialize circuit breakers for downstream services.
	downstreamServices := []string{"user-service", "auction-service"}
	cbs := make(map[string]circuitbreaker.CircuitBreaker, len(downstreamServices))

	for _, svc := range downstreamServices {
		cbCfg := circuitbreaker.DefaultConfig(svc)
		cb, cbErr := circuitbreaker.New(cbCfg,
			circuitbreaker.WithLogger(zap.NewNop()),
			circuitbreaker.WithPrometheusRegisterer(prometheus.DefaultRegisterer),
		)
		if cbErr != nil {
			log.Warn("failed to create circuit breaker",
				zap.String("service", svc),
				zap.Error(cbErr),
			)
			continue
		}
		cbs[svc] = cb
	}

	log.Info("shared packages initialized",
		zap.String("service", ServiceName),
		zap.String("env", cfg.Environment),
		zap.Int("circuit_breakers", len(cbs)),
	)

	return &Services{
		Logger:          log,
		Validator:       v,
		CircuitBreakers: cbs,
	}, nil
}

// Close cleans up shared package resources.
func (s *Services) Close() {
	if s.Logger != nil {
		_ = s.Logger.Sync()
	}
}
