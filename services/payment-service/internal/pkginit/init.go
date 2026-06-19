// Package pkginit wires shared pkg/ packages into the payment-service.
// It initializes the structured logger, circuit breakers for downstream calls,
// Kafka producer, and validation.
package pkginit

import (
	"github.com/auction-system/pkg/circuitbreaker"
	"github.com/auction-system/pkg/kafka"
	"github.com/auction-system/pkg/logger"
	"github.com/auction-system/pkg/validation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	ServiceName = "payment-service"
	Version     = "1.0.0"
)

// Services holds initialized shared package instances for the payment-service.
type Services struct {
	Logger          logger.Logger
	Validator       validation.Validator
	KafkaProducer   *kafka.Producer
	CircuitBreakers map[string]circuitbreaker.CircuitBreaker
}

// Config holds configuration needed by shared packages.
type Config struct {
	Environment  string
	KafkaBrokers []string
	KafkaTopic   string
}

// Init initializes all shared packages for the payment-service.
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

	// 3. Initialize Kafka producer using shared pkg.
	var producer *kafka.Producer
	if len(cfg.KafkaBrokers) > 0 && cfg.KafkaBrokers[0] != "" {
		producer, err = kafka.NewProducer(kafka.ProducerConfig{
			Brokers: cfg.KafkaBrokers,
			Topic:   cfg.KafkaTopic,
		}, zap.NewNop())
		if err != nil {
			log.Warn("shared kafka producer init failed — using existing producer",
				zap.Error(err),
			)
		} else {
			log.Info("shared kafka producer initialized")
		}
	}

	// 4. Initialize circuit breakers for downstream services.
	downstreamServices := []string{"user-service", "auction-service", "bid-service", "notification-service"}
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
		KafkaProducer:   producer,
		CircuitBreakers: cbs,
	}, nil
}

// Close cleans up shared package resources.
func (s *Services) Close() {
	if s.KafkaProducer != nil {
		_ = s.KafkaProducer.Close()
	}
	if s.Logger != nil {
		_ = s.Logger.Sync()
	}
}
