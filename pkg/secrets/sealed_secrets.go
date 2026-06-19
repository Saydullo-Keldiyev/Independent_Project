package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// SealedSecretsConfig configures the SealedSecrets provider.
type SealedSecretsConfig struct {
	// Namespace is the Kubernetes namespace where SealedSecrets are decrypted.
	// Should match the environment (e.g., "auction-system-dev", "auction-system-prod").
	Namespace string

	// SecretsMountPath is the filesystem path where Kubernetes secrets are mounted.
	// Default: /etc/secrets
	SecretsMountPath string

	// Environment determines which namespace/path to use for secrets.
	Environment Environment
}

// DefaultSealedSecretsConfig returns a SealedSecretsConfig with defaults.
func DefaultSealedSecretsConfig(env Environment) SealedSecretsConfig {
	return SealedSecretsConfig{
		Namespace:        fmt.Sprintf("auction-system-%s", env),
		SecretsMountPath: "/etc/secrets",
		Environment:      env,
	}
}

// SealedSecretsProvider implements SecretProvider by reading Kubernetes Secrets
// that were originally encrypted as SealedSecrets and decrypted by the SealedSecrets controller.
// At runtime, these are standard Kubernetes Secrets mounted as volumes or environment variables.
type SealedSecretsProvider struct {
	config SealedSecretsConfig
	logger *zap.Logger
}

// NewSealedSecretsProvider creates a new SealedSecrets-based secret provider.
// It reads secrets from the mounted Kubernetes Secret volumes.
func NewSealedSecretsProvider(config SealedSecretsConfig, logger *zap.Logger) (*SealedSecretsProvider, error) {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	if config.SecretsMountPath == "" {
		config.SecretsMountPath = "/etc/secrets"
	}

	return &SealedSecretsProvider{
		config: config,
		logger: logger,
	}, nil
}

// GetSecret retrieves a single secret value by key.
// For SealedSecrets, the path maps to the secret name and the key maps to the data key.
// Secrets are read from: {SecretsMountPath}/{environment}/{path}/{key}
func (s *SealedSecretsProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
	// Build the filesystem path
	// path format from SecretManager: "secret/data/{env}/{service}/{subpath}"
	// We extract the meaningful part and map to filesystem
	secretPath := s.resolveFilePath(path, key)

	data, err := os.ReadFile(secretPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: path=%s, key=%s", ErrSecretNotFound, path, key)
		}
		return "", fmt.Errorf("failed to read secret file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// GetSecrets retrieves all secrets at the given path as a key-value map.
// For SealedSecrets, this reads all files in the secret mount directory.
func (s *SealedSecretsProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	dirPath := s.resolveDirPath(path)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: path=%s", ErrSecretNotFound, path)
		}
		return nil, fmt.Errorf("failed to read secrets directory: %w", err)
	}

	result := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Skip hidden files and symlink artifacts
		if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "..") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dirPath, entry.Name()))
		if err != nil {
			s.logger.Warn("failed to read secret file",
				zap.String("file", entry.Name()),
				zap.Error(err),
			)
			continue
		}

		result[entry.Name()] = strings.TrimSpace(string(data))
	}

	// Also try reading a JSON file if it exists (some secrets are mounted as JSON)
	jsonPath := dirPath + ".json"
	if jsonData, err := os.ReadFile(jsonPath); err == nil {
		var jsonSecrets map[string]string
		if err := json.Unmarshal(jsonData, &jsonSecrets); err == nil {
			for k, v := range jsonSecrets {
				result[k] = v
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%w: path=%s (directory empty)", ErrSecretNotFound, path)
	}

	return result, nil
}

// IsAvailable checks if the secret mount path exists and is accessible.
func (s *SealedSecretsProvider) IsAvailable(ctx context.Context) bool {
	info, err := os.Stat(s.config.SecretsMountPath)
	if err != nil {
		s.logger.Debug("sealed secrets mount path not available", zap.Error(err))
		return false
	}
	return info.IsDir()
}

// Close is a no-op for SealedSecrets (file-based, no connections to close).
func (s *SealedSecretsProvider) Close() error {
	return nil
}

// resolveFilePath maps the logical path + key to a filesystem path.
// Input path format: "secret/data/{env}/{service}/{subpath}"
// Output: "{SecretsMountPath}/{service}/{subpath}/{key}"
func (s *SealedSecretsProvider) resolveFilePath(path string, key string) string {
	// Strip the "secret/data/{env}/" prefix to get relative path
	relative := s.stripPathPrefix(path)
	return filepath.Join(s.config.SecretsMountPath, relative, key)
}

// resolveDirPath maps the logical path to a filesystem directory.
func (s *SealedSecretsProvider) resolveDirPath(path string) string {
	relative := s.stripPathPrefix(path)
	return filepath.Join(s.config.SecretsMountPath, relative)
}

// stripPathPrefix removes the "secret/data/{env}/" prefix from a path.
func (s *SealedSecretsProvider) stripPathPrefix(path string) string {
	prefix := fmt.Sprintf("secret/data/%s/", s.config.Environment)
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	// If no expected prefix, return as-is
	return path
}
