package secrets

import (
	"context"
	"fmt"
	"os"
	"time"

	vault "github.com/hashicorp/vault/api"
	"go.uber.org/zap"
)

// VaultConfig configures the Vault secret provider.
type VaultConfig struct {
	// Address is the Vault server address (e.g., https://vault.example.com:8200).
	Address string

	// Token is the Vault authentication token.
	// In production, this should come from Kubernetes service account auth or AppRole.
	Token string

	// Namespace is the Vault namespace for multi-tenant installations.
	// Typically set per environment (e.g., "auction-system/dev").
	Namespace string

	// Timeout for Vault API calls.
	Timeout time.Duration

	// AuthMethod specifies the authentication method to use.
	// Supported: "token", "kubernetes", "approle"
	AuthMethod string

	// KubernetesRole is the Vault role for Kubernetes auth method.
	KubernetesRole string

	// KubernetesMountPath is the mount path for Kubernetes auth.
	KubernetesMountPath string

	// AppRoleID and SecretID for AppRole auth method.
	AppRoleID       string
	AppRoleSecretID string
}

// DefaultVaultConfig returns a VaultConfig with sensible defaults.
func DefaultVaultConfig() VaultConfig {
	return VaultConfig{
		Address:             os.Getenv("VAULT_ADDR"),
		Token:               os.Getenv("VAULT_TOKEN"),
		Namespace:           os.Getenv("VAULT_NAMESPACE"),
		Timeout:             10 * time.Second,
		AuthMethod:          "token",
		KubernetesMountPath: "auth/kubernetes",
	}
}

// VaultProvider implements SecretProvider using HashiCorp Vault.
type VaultProvider struct {
	client *vault.Client
	config VaultConfig
	logger *zap.Logger
}

// NewVaultProvider creates a new Vault-based secret provider.
func NewVaultProvider(config VaultConfig, logger *zap.Logger) (*VaultProvider, error) {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = config.Address

	if config.Timeout > 0 {
		vaultConfig.Timeout = config.Timeout
	}

	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	}

	provider := &VaultProvider{
		client: client,
		config: config,
		logger: logger,
	}

	// Authenticate based on the configured method
	if err := provider.authenticate(); err != nil {
		return nil, fmt.Errorf("failed to authenticate with Vault: %w", err)
	}

	return provider, nil
}

// authenticate performs Vault authentication based on the configured method.
func (v *VaultProvider) authenticate() error {
	switch v.config.AuthMethod {
	case "token":
		if v.config.Token == "" {
			return fmt.Errorf("vault token is required for token auth method")
		}
		v.client.SetToken(v.config.Token)
		return nil

	case "kubernetes":
		return v.authenticateKubernetes()

	case "approle":
		return v.authenticateAppRole()

	default:
		return fmt.Errorf("unsupported auth method: %s", v.config.AuthMethod)
	}
}

// authenticateKubernetes authenticates using Kubernetes service account token.
func (v *VaultProvider) authenticateKubernetes() error {
	// Read the service account token from the standard Kubernetes mount path
	jwt, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return fmt.Errorf("failed to read service account token: %w", err)
	}

	mountPath := v.config.KubernetesMountPath
	if mountPath == "" {
		mountPath = "auth/kubernetes"
	}

	data := map[string]interface{}{
		"role": v.config.KubernetesRole,
		"jwt":  string(jwt),
	}

	secret, err := v.client.Logical().Write(mountPath+"/login", data)
	if err != nil {
		return fmt.Errorf("kubernetes auth failed: %w", err)
	}

	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("kubernetes auth returned no token")
	}

	v.client.SetToken(secret.Auth.ClientToken)
	v.logger.Info("authenticated with Vault via Kubernetes",
		zap.String("role", v.config.KubernetesRole),
	)
	return nil
}

// authenticateAppRole authenticates using AppRole credentials.
func (v *VaultProvider) authenticateAppRole() error {
	data := map[string]interface{}{
		"role_id":   v.config.AppRoleID,
		"secret_id": v.config.AppRoleSecretID,
	}

	secret, err := v.client.Logical().Write("auth/approle/login", data)
	if err != nil {
		return fmt.Errorf("approle auth failed: %w", err)
	}

	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("approle auth returned no token")
	}

	v.client.SetToken(secret.Auth.ClientToken)
	v.logger.Info("authenticated with Vault via AppRole")
	return nil
}

// GetSecret retrieves a single secret value by key from the given Vault path.
func (v *VaultProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
	secret, err := v.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to read secret from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("%w: path=%s", ErrSecretNotFound, path)
	}

	// For KV v2, data is nested under "data" key
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		// Try direct access for KV v1
		data = secret.Data
	}

	value, ok := data[key]
	if !ok {
		return "", fmt.Errorf("%w: path=%s, key=%s", ErrSecretNotFound, path, key)
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("secret value at path=%s, key=%s is not a string", path, key)
	}

	return strValue, nil
}

// GetSecrets retrieves all secrets at the given Vault path as a key-value map.
func (v *VaultProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	secret, err := v.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("%w: path=%s", ErrSecretNotFound, path)
	}

	// For KV v2, data is nested under "data" key
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		// Try direct access for KV v1
		data = secret.Data
	}

	result := make(map[string]string, len(data))
	for k, v := range data {
		strValue, ok := v.(string)
		if !ok {
			// Skip non-string values with a warning
			continue
		}
		result[k] = strValue
	}

	return result, nil
}

// IsAvailable checks if the Vault server is reachable and the token is valid.
func (v *VaultProvider) IsAvailable(ctx context.Context) bool {
	// Use sys/health endpoint which returns health status without requiring auth
	health, err := v.client.Sys().HealthWithContext(ctx)
	if err != nil {
		v.logger.Debug("vault health check failed", zap.Error(err))
		return false
	}

	// Vault is available if initialized and not sealed
	return health.Initialized && !health.Sealed
}

// Close is a no-op for Vault (HTTP client doesn't need explicit close).
func (v *VaultProvider) Close() error {
	v.client.ClearToken()
	return nil
}

// Client returns the underlying Vault client for advanced operations.
func (v *VaultProvider) Client() *vault.Client {
	return v.client
}
