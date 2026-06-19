// Package pkginit wires shared pkg/ packages into the api-gateway service.
// It initializes the structured logger, circuit breakers for downstream services,
// and the validation middleware.
package pkginit

import (
	"github.com/auction-system/pkg/circuitbreaker"
	"github.com/auction-system/pkg/logger"
	"github.com/auction-system/pkg/validation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	ServiceName = "api-gateway"
	Version     = "1.0.0"
)

// Services holds initialized shared package instances for the api-gateway.
type Services struct {
	Logger          logger.Logger
	Validator       validation.Validator
	CircuitBreakers map[string]circuitbreaker.CircuitBreaker
}

// Init initializes all shared packages for the api-gateway service.
func Init(env string) (*Services, error) {
	// 1. Initialize structured logger with correlation ID support.
	log, err := logger.New(logger.LogConfig{
		ServiceName: ServiceName,
		Environment: env,
		Version:     Version,
	})
	if err != nil {
		return nil, err
	}

	// 2. Initialize validator.
	v := validation.New()

	// 3. Initialize circuit breakers for downstream services.
	downstreamServices := []string{"user-service", "auction-service", "bid-service", "notification-service", "payment-service"}
	cbs := make(map[string]circuitbreaker.CircuitBreaker, len(downstreamServices))

	for _, svc := range downstreamServices {
		cfg := circuitbreaker.DefaultConfig(svc)
		cb, cbErr := circuitbreaker.New(cfg,
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
		zap.String("env", env),
		zap.Int("circuit_breakers", len(cbs)),
	)

	return &Services{
		Logger:          log,
		Validator:       v,
		CircuitBreakers: cbs,
	}, nil
}
