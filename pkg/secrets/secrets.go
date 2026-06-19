// Package secrets provides a unified interface for retrieving application secrets
// from HashiCorp Vault or Kubernetes SealedSecrets at runtime. It supports
// environment-based path separation (dev, staging, prod), retry with exponential
// backoff on startup, and failing pod startup if secrets cannot be retrieved.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
)

// Errors returned by the secrets package.
var (
	ErrSecretsUnavailable = errors.New("secrets: unable to retrieve secrets after retries")
	ErrInvalidEnvironment = errors.New("secrets: invalid environment specified")
	ErrSecretNotFound     = errors.New("secrets: secret not found at specified path")
	ErrProviderNotReady   = errors.New("secrets: provider is not ready")
)

// Environment represents a deployment environment for secret path separation.
type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

// ValidEnvironments is the set of supported environments.
var ValidEnvironments = map[Environment]bool{
	EnvDev:     true,
	EnvStaging: true,
	EnvProd:    true,
}

// RetryConfig defines the retry behavior for secret retrieval on startup.
type RetryConfig struct {
	MaxRetries   int           // Maximum number of retry attempts; default 5
	MaxTotalTime time.Duration // Maximum total time for all retries; default 60s
	BaseDelay    time.Duration // Initial backoff delay; default 1s
	MaxDelay     time.Duration // Maximum delay between retries; default 30s
}

// DefaultRetryConfig returns a RetryConfig meeting the requirement:
// 5 retries with exponential backoff, max 60s total.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   5,
		MaxTotalTime: 60 * time.Second,
		BaseDelay:    1 * time.Second,
		MaxDelay:     30 * time.Second,
	}
}

// Config configures the SecretManager.
type Config struct {
	// Environment determines the Vault path/namespace for secrets.
	Environment Environment

	// ServiceName identifies the service requesting secrets.
	ServiceName string

	// Retry configures the startup retry behavior.
	Retry RetryConfig
}

// SecretProvider is the interface that backends (Vault, SealedSecrets) must implement.
// It provides methods for retrieving individual secrets and bulk secret loading.
type SecretProvider interface {
	// GetSecret retrieves a single secret value by its key from the given path.
	GetSecret(ctx context.Context, path string, key string) (string, error)

	// GetSecrets retrieves all secrets at the given path as a key-value map.
	GetSecrets(ctx context.Context, path string) (map[string]string, error)

	// IsAvailable checks if the secret provider is currently reachable.
	IsAvailable(ctx context.Context) bool

	// Close releases any resources held by the provider.
	Close() error
}

// SecretManager orchestrates secret retrieval with retry logic and environment-based paths.
type SecretManager struct {
	provider    SecretProvider
	config      Config
	logger      *zap.Logger
	initialized bool
}

// NewSecretManager creates a new SecretManager with the given provider and configuration.
func NewSecretManager(provider SecretProvider, config Config, logger *zap.Logger) (*SecretManager, error) {
	if !ValidEnvironments[config.Environment] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEnvironment, config.Environment)
	}

	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	if config.Retry.MaxRetries == 0 {
		config.Retry = DefaultRetryConfig()
	}

	return &SecretManager{
		provider: provider,
		config:   config,
		logger:   logger,
	}, nil
}

// Initialize attempts to connect to the secret provider with retry logic.
// This should be called during pod startup. It will retry up to MaxRetries times
// with exponential backoff, failing with ErrSecretsUnavailable if the provider
// cannot be reached within MaxTotalTime.
func (sm *SecretManager) Initialize(ctx context.Context) error {
	cfg := sm.config.Retry
	startTime := time.Now()
	delay := cfg.BaseDelay

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		// Check total time budget
		elapsed := time.Since(startTime)
		if elapsed >= cfg.MaxTotalTime {
			sm.logger.Error("secret retrieval exceeded maximum total time",
				zap.Duration("elapsed", elapsed),
				zap.Duration("max_total_time", cfg.MaxTotalTime),
				zap.Int("attempts", attempt-1),
				zap.String("environment", string(sm.config.Environment)),
				zap.String("service", sm.config.ServiceName),
			)
			return fmt.Errorf("%w: exceeded max total time %v after %d attempts",
				ErrSecretsUnavailable, cfg.MaxTotalTime, attempt-1)
		}

		sm.logger.Info("attempting to connect to secret provider",
			zap.Int("attempt", attempt),
			zap.Int("max_retries", cfg.MaxRetries),
			zap.String("environment", string(sm.config.Environment)),
			zap.String("service", sm.config.ServiceName),
		)

		if sm.provider.IsAvailable(ctx) {
			sm.initialized = true
			sm.logger.Info("secret provider connected successfully",
				zap.Int("attempt", attempt),
				zap.Duration("elapsed", time.Since(startTime)),
				zap.String("environment", string(sm.config.Environment)),
			)
			return nil
		}

		sm.logger.Warn("secret provider unavailable, retrying",
			zap.Int("attempt", attempt),
			zap.Int("max_retries", cfg.MaxRetries),
			zap.Duration("next_delay", delay),
			zap.String("environment", string(sm.config.Environment)),
		)

		// Wait with backoff, respecting context cancellation and total time budget
		remainingTime := cfg.MaxTotalTime - time.Since(startTime)
		waitDuration := delay
		if waitDuration > remainingTime {
			waitDuration = remainingTime
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: context cancelled after %d attempts: %v",
				ErrSecretsUnavailable, attempt, ctx.Err())
		case <-time.After(waitDuration):
		}

		// Exponential backoff: delay * 2^(attempt-1), capped at MaxDelay
		delay = time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	sm.logger.Error("failed to connect to secret provider after all retries",
		zap.Int("max_retries", cfg.MaxRetries),
		zap.Duration("elapsed", time.Since(startTime)),
		zap.String("environment", string(sm.config.Environment)),
		zap.String("service", sm.config.ServiceName),
	)
	return fmt.Errorf("%w: failed after %d retries over %v",
		ErrSecretsUnavailable, cfg.MaxRetries, time.Since(startTime))
}

// GetSecret retrieves a single secret by key, using the environment-based path.
// The full path is constructed as: secret/{environment}/{serviceName}/{secretPath}
func (sm *SecretManager) GetSecret(ctx context.Context, secretPath string, key string) (string, error) {
	if !sm.initialized {
		return "", ErrProviderNotReady
	}

	fullPath := sm.buildPath(secretPath)
	value, err := sm.provider.GetSecret(ctx, fullPath, key)
	if err != nil {
		sm.logger.Error("failed to retrieve secret",
			zap.String("path", fullPath),
			zap.String("key", key),
			zap.Error(err),
		)
		return "", err
	}

	return value, nil
}

// GetSecrets retrieves all secrets at the given path as a key-value map.
func (sm *SecretManager) GetSecrets(ctx context.Context, secretPath string) (map[string]string, error) {
	if !sm.initialized {
		return nil, ErrProviderNotReady
	}

	fullPath := sm.buildPath(secretPath)
	secrets, err := sm.provider.GetSecrets(ctx, fullPath)
	if err != nil {
		sm.logger.Error("failed to retrieve secrets",
			zap.String("path", fullPath),
			zap.Error(err),
		)
		return nil, err
	}

	return secrets, nil
}

// LoadServiceSecrets loads all secrets for the current service in a single call.
// Path: secret/{environment}/{serviceName}
func (sm *SecretManager) LoadServiceSecrets(ctx context.Context) (map[string]string, error) {
	if !sm.initialized {
		return nil, ErrProviderNotReady
	}

	fullPath := sm.buildPath("")
	secrets, err := sm.provider.GetSecrets(ctx, fullPath)
	if err != nil {
		sm.logger.Error("failed to load service secrets",
			zap.String("path", fullPath),
			zap.String("service", sm.config.ServiceName),
			zap.Error(err),
		)
		return nil, err
	}

	sm.logger.Info("loaded service secrets",
		zap.String("service", sm.config.ServiceName),
		zap.Int("secret_count", len(secrets)),
	)

	return secrets, nil
}

// buildPath constructs the full Vault path using the environment and service name.
// Format: secret/{environment}/{serviceName}/{subPath}
func (sm *SecretManager) buildPath(subPath string) string {
	basePath := fmt.Sprintf("secret/data/%s/%s", sm.config.Environment, sm.config.ServiceName)
	if subPath == "" {
		return basePath
	}
	return fmt.Sprintf("%s/%s", basePath, subPath)
}

// IsReady returns whether the SecretManager has been successfully initialized.
func (sm *SecretManager) IsReady() bool {
	return sm.initialized
}

// Close releases resources held by the underlying provider.
func (sm *SecretManager) Close() error {
	if sm.provider != nil {
		return sm.provider.Close()
	}
	return nil
}
