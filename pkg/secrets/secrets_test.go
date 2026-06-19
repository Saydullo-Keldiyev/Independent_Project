package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// mockProvider is a test implementation of SecretProvider.
type mockProvider struct {
	available     bool
	secrets       map[string]map[string]string // path -> key -> value
	availableCalls int
	getSecretCalls int
}

func newMockProvider(available bool) *mockProvider {
	return &mockProvider{
		available: available,
		secrets:   make(map[string]map[string]string),
	}
}

func (m *mockProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
	m.getSecretCalls++
	pathSecrets, ok := m.secrets[path]
	if !ok {
		return "", ErrSecretNotFound
	}
	value, ok := pathSecrets[key]
	if !ok {
		return "", ErrSecretNotFound
	}
	return value, nil
}

func (m *mockProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	pathSecrets, ok := m.secrets[path]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return pathSecrets, nil
}

func (m *mockProvider) IsAvailable(ctx context.Context) bool {
	m.availableCalls++
	return m.available
}

func (m *mockProvider) Close() error {
	return nil
}

func (m *mockProvider) addSecret(path, key, value string) {
	if m.secrets[path] == nil {
		m.secrets[path] = make(map[string]string)
	}
	m.secrets[path][key] = value
}

// --- SecretManager Tests ---

func TestNewSecretManager_ValidEnvironments(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(true)

	tests := []struct {
		env     Environment
		wantErr bool
	}{
		{EnvDev, false},
		{EnvStaging, false},
		{EnvProd, false},
		{Environment("invalid"), true},
		{Environment(""), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			config := Config{
				Environment: tt.env,
				ServiceName: "test-service",
			}
			sm, err := NewSecretManager(provider, config, logger)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for env=%q, got nil", tt.env)
				}
				if !errors.Is(err, ErrInvalidEnvironment) {
					t.Errorf("expected ErrInvalidEnvironment, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for env=%q: %v", tt.env, err)
				}
				if sm == nil {
					t.Error("expected non-nil SecretManager")
				}
			}
		})
	}
}

func TestSecretManager_Initialize_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(true)

	sm, err := NewSecretManager(provider, Config{
		Environment: EnvDev,
		ServiceName: "test-service",
		Retry:       DefaultRetryConfig(),
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	ctx := context.Background()
	err = sm.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !sm.IsReady() {
		t.Error("expected IsReady to be true after successful initialization")
	}

	if provider.availableCalls != 1 {
		t.Errorf("expected 1 availability check, got %d", provider.availableCalls)
	}
}

func TestSecretManager_Initialize_RetriesOnFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(false)

	sm, err := NewSecretManager(provider, Config{
		Environment: EnvDev,
		ServiceName: "test-service",
		Retry: RetryConfig{
			MaxRetries:   3,
			MaxTotalTime: 5 * time.Second,
			BaseDelay:    10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
		},
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	ctx := context.Background()
	err = sm.Initialize(ctx)
	if err == nil {
		t.Fatal("expected Initialize to fail when provider is unavailable")
	}

	if !errors.Is(err, ErrSecretsUnavailable) {
		t.Errorf("expected ErrSecretsUnavailable, got %v", err)
	}

	if !sm.IsReady() == true {
		// sm should NOT be ready
	}
	if sm.IsReady() {
		t.Error("expected IsReady to be false after failed initialization")
	}

	// Should have retried 3 times
	if provider.availableCalls != 3 {
		t.Errorf("expected 3 availability checks, got %d", provider.availableCalls)
	}
}

func TestSecretManager_Initialize_RespectsContextCancellation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(false)

	sm, err := NewSecretManager(provider, Config{
		Environment: EnvDev,
		ServiceName: "test-service",
		Retry: RetryConfig{
			MaxRetries:   5,
			MaxTotalTime: 60 * time.Second,
			BaseDelay:    100 * time.Millisecond,
			MaxDelay:     1 * time.Second,
		},
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err = sm.Initialize(ctx)
	if err == nil {
		t.Fatal("expected Initialize to fail on context cancellation")
	}

	if !errors.Is(err, ErrSecretsUnavailable) {
		t.Errorf("expected ErrSecretsUnavailable, got %v", err)
	}
}

func TestSecretManager_Initialize_RespectsMaxTotalTime(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(false)

	maxTotal := 200 * time.Millisecond
	sm, err := NewSecretManager(provider, Config{
		Environment: EnvStaging,
		ServiceName: "test-service",
		Retry: RetryConfig{
			MaxRetries:   10, // High retry count
			MaxTotalTime: maxTotal,
			BaseDelay:    50 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
		},
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	start := time.Now()
	ctx := context.Background()
	err = sm.Initialize(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Initialize to fail")
	}

	// Should not exceed MaxTotalTime by more than a small margin
	if elapsed > maxTotal+100*time.Millisecond {
		t.Errorf("Initialize took %v, expected at most %v", elapsed, maxTotal+100*time.Millisecond)
	}
}

func TestSecretManager_GetSecret_BeforeInitialize(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(true)

	sm, err := NewSecretManager(provider, Config{
		Environment: EnvDev,
		ServiceName: "test-service",
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	// Don't call Initialize
	_, err = sm.GetSecret(context.Background(), "db", "password")
	if !errors.Is(err, ErrProviderNotReady) {
		t.Errorf("expected ErrProviderNotReady, got %v", err)
	}
}

func TestSecretManager_GetSecret_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(true)
	provider.addSecret("secret/data/dev/bid-service/db", "password", "super-secret-pw")

	sm, err := NewSecretManager(provider, Config{
		Environment: EnvDev,
		ServiceName: "bid-service",
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	if err := sm.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	value, err := sm.GetSecret(context.Background(), "db", "password")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}

	if value != "super-secret-pw" {
		t.Errorf("expected 'super-secret-pw', got %q", value)
	}
}

func TestSecretManager_GetSecrets_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(true)
	provider.secrets["secret/data/prod/payment-service/db"] = map[string]string{
		"host":     "db.prod.internal",
		"password": "prod-pw-123",
		"port":     "5432",
	}

	sm, err := NewSecretManager(provider, Config{
		Environment: EnvProd,
		ServiceName: "payment-service",
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	if err := sm.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	secrets, err := sm.GetSecrets(context.Background(), "db")
	if err != nil {
		t.Fatalf("GetSecrets failed: %v", err)
	}

	if len(secrets) != 3 {
		t.Errorf("expected 3 secrets, got %d", len(secrets))
	}
	if secrets["password"] != "prod-pw-123" {
		t.Errorf("expected password='prod-pw-123', got %q", secrets["password"])
	}
}

func TestSecretManager_EnvironmentPathSeparation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Verify that different environments produce different paths
	tests := []struct {
		env      Environment
		service  string
		subPath  string
		expected string
	}{
		{EnvDev, "bid-service", "db", "secret/data/dev/bid-service/db"},
		{EnvStaging, "bid-service", "db", "secret/data/staging/bid-service/db"},
		{EnvProd, "bid-service", "db", "secret/data/prod/bid-service/db"},
		{EnvProd, "payment-service", "redis", "secret/data/prod/payment-service/redis"},
		{EnvDev, "api-gateway", "", "secret/data/dev/api-gateway"},
	}

	for _, tt := range tests {
		t.Run(string(tt.env)+"/"+tt.service+"/"+tt.subPath, func(t *testing.T) {
			provider := newMockProvider(true)
			provider.addSecret(tt.expected, "test-key", "test-value")

			sm, err := NewSecretManager(provider, Config{
				Environment: tt.env,
				ServiceName: tt.service,
			}, logger)
			if err != nil {
				t.Fatalf("failed to create SecretManager: %v", err)
			}

			if err := sm.Initialize(context.Background()); err != nil {
				t.Fatalf("Initialize failed: %v", err)
			}

			value, err := sm.GetSecret(context.Background(), tt.subPath, "test-key")
			if err != nil {
				t.Fatalf("GetSecret failed: %v", err)
			}
			if value != "test-value" {
				t.Errorf("expected 'test-value', got %q", value)
			}
		})
	}
}

func TestSecretManager_LoadServiceSecrets(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider := newMockProvider(true)
	provider.secrets["secret/data/staging/user-service"] = map[string]string{
		"db_url":      "postgres://...",
		"jwt_secret":  "jwt-key-123",
		"redis_pass":  "redis-pw",
		"smtp_pass":   "smtp-pw",
	}

	sm, err := NewSecretManager(provider, Config{
		Environment: EnvStaging,
		ServiceName: "user-service",
	}, logger)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	if err := sm.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	secrets, err := sm.LoadServiceSecrets(context.Background())
	if err != nil {
		t.Fatalf("LoadServiceSecrets failed: %v", err)
	}

	if len(secrets) != 4 {
		t.Errorf("expected 4 secrets, got %d", len(secrets))
	}
	if secrets["jwt_secret"] != "jwt-key-123" {
		t.Errorf("expected jwt_secret='jwt-key-123', got %q", secrets["jwt_secret"])
	}
}

func TestSecretManager_DefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", cfg.MaxRetries)
	}
	if cfg.MaxTotalTime != 60*time.Second {
		t.Errorf("expected MaxTotalTime=60s, got %v", cfg.MaxTotalTime)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay=1s, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay=30s, got %v", cfg.MaxDelay)
	}
}

// --- SealedSecretsProvider Tests ---

func TestSealedSecretsProvider_GetSecret(t *testing.T) {
	// Create a temp directory to simulate mounted secrets
	tmpDir := t.TempDir()

	// Create directory structure: {mount}/{service}/{subpath}/{key}
	secretDir := filepath.Join(tmpDir, "bid-service", "db")
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		t.Fatalf("failed to create secret dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "password"), []byte("my-db-password"), 0644); err != nil {
		t.Fatalf("failed to write secret: %v", err)
	}

	logger := zaptest.NewLogger(t)
	provider, err := NewSealedSecretsProvider(SealedSecretsConfig{
		Namespace:        "auction-system-dev",
		SecretsMountPath: tmpDir,
		Environment:      EnvDev,
	}, logger)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	value, err := provider.GetSecret(context.Background(), "secret/data/dev/bid-service/db", "password")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}

	if value != "my-db-password" {
		t.Errorf("expected 'my-db-password', got %q", value)
	}
}

func TestSealedSecretsProvider_GetSecrets(t *testing.T) {
	tmpDir := t.TempDir()

	secretDir := filepath.Join(tmpDir, "payment-service", "db")
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		t.Fatalf("failed to create secret dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "host"), []byte("db.prod.internal"), 0644); err != nil {
		t.Fatalf("failed to write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "password"), []byte("prod-pw"), 0644); err != nil {
		t.Fatalf("failed to write secret: %v", err)
	}

	logger := zaptest.NewLogger(t)
	provider, err := NewSealedSecretsProvider(SealedSecretsConfig{
		Namespace:        "auction-system-prod",
		SecretsMountPath: tmpDir,
		Environment:      EnvProd,
	}, logger)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	secrets, err := provider.GetSecrets(context.Background(), "secret/data/prod/payment-service/db")
	if err != nil {
		t.Fatalf("GetSecrets failed: %v", err)
	}

	if len(secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(secrets))
	}
	if secrets["password"] != "prod-pw" {
		t.Errorf("expected 'prod-pw', got %q", secrets["password"])
	}
}

func TestSealedSecretsProvider_IsAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zaptest.NewLogger(t)

	provider, _ := NewSealedSecretsProvider(SealedSecretsConfig{
		SecretsMountPath: tmpDir,
		Environment:      EnvDev,
	}, logger)

	if !provider.IsAvailable(context.Background()) {
		t.Error("expected provider to be available with existing directory")
	}

	// Non-existent path
	provider2, _ := NewSealedSecretsProvider(SealedSecretsConfig{
		SecretsMountPath: "/nonexistent/path",
		Environment:      EnvDev,
	}, logger)

	if provider2.IsAvailable(context.Background()) {
		t.Error("expected provider to be unavailable with non-existent path")
	}
}

func TestSealedSecretsProvider_GetSecret_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zaptest.NewLogger(t)

	provider, _ := NewSealedSecretsProvider(SealedSecretsConfig{
		SecretsMountPath: tmpDir,
		Environment:      EnvDev,
	}, logger)

	_, err := provider.GetSecret(context.Background(), "secret/data/dev/service/nonexistent", "key")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

// --- Nil logger handling ---

func TestNewSecretManager_NilLogger(t *testing.T) {
	provider := newMockProvider(true)
	sm, err := NewSecretManager(provider, Config{
		Environment: EnvDev,
		ServiceName: "test",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm == nil {
		t.Fatal("expected non-nil SecretManager")
	}
}

// --- buildPath tests ---

func TestSecretManager_BuildPath(t *testing.T) {
	logger, _ := zap.NewNop(), error(nil)
	provider := newMockProvider(true)

	sm := &SecretManager{
		provider: provider,
		config: Config{
			Environment: EnvProd,
			ServiceName: "bid-service",
		},
		logger: logger,
	}

	tests := []struct {
		subPath  string
		expected string
	}{
		{"db", "secret/data/prod/bid-service/db"},
		{"redis", "secret/data/prod/bid-service/redis"},
		{"kafka/credentials", "secret/data/prod/bid-service/kafka/credentials"},
		{"", "secret/data/prod/bid-service"},
	}

	for _, tt := range tests {
		t.Run(tt.subPath, func(t *testing.T) {
			result := sm.buildPath(tt.subPath)
			if result != tt.expected {
				t.Errorf("buildPath(%q) = %q, want %q", tt.subPath, result, tt.expected)
			}
		})
	}
}
