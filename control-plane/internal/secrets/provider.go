package secrets

import (
	"context"
	"fmt"
)

// ProviderType identifies the secret management backend.
type ProviderType string

const (
	// ProviderEnv uses environment variables as the secret store (default).
	ProviderEnv ProviderType = "env"
	// ProviderAWSKMS uses AWS KMS for envelope encryption of secrets.
	ProviderAWSKMS ProviderType = "aws-kms"
	// ProviderGCPKMS uses GCP Cloud KMS for envelope encryption of secrets.
	ProviderGCPKMS ProviderType = "gcp-kms"
	// ProviderVault uses HashiCorp Vault for secret management.
	ProviderVault ProviderType = "vault"
)

// Provider defines the interface for secret management backends.
// Implementations must be safe for concurrent use.
type Provider interface {
	// GetSecret retrieves a secret value by key.
	// Returns an empty string and no error when the key does not exist.
	GetSecret(ctx context.Context, key string) (string, error)

	// SetSecret stores a secret value under the given key.
	SetSecret(ctx context.Context, key, value string) error

	// DeleteSecret removes a secret by key.
	// Returns no error if the key does not exist.
	DeleteSecret(ctx context.Context, key string) error

	// Type returns the provider backend type.
	Type() ProviderType
}

// Config holds the configuration for the secret management subsystem.
type Config struct {
	// Provider selects the backend: "env" (default), "aws-kms", "gcp-kms", "vault".
	Provider ProviderType `yaml:"provider" mapstructure:"provider"`

	// KeyPrefix is prepended to all secret keys when reading from env vars.
	// Defaults to "PLAYGROUND_" with a "AGENTS_" fallback for backward compatibility.
	KeyPrefix string `yaml:"key_prefix" mapstructure:"key_prefix"`

	// AWSKMS holds configuration specific to the aws-kms provider.
	AWSKMS AWSKMSConfig `yaml:"aws_kms" mapstructure:"aws_kms"`

	// GCPKMS holds configuration specific to the gcp-kms provider.
	GCPKMS GCPKMSConfig `yaml:"gcp_kms" mapstructure:"gcp_kms"`

	// Vault holds configuration specific to the vault provider.
	Vault VaultConfig `yaml:"vault" mapstructure:"vault"`
}

// AWSKMSConfig holds AWS KMS-specific settings.
type AWSKMSConfig struct {
	Region string `yaml:"region" mapstructure:"region"`
	KeyID  string `yaml:"key_id" mapstructure:"key_id"`
}

// GCPKMSConfig holds GCP Cloud KMS-specific settings.
type GCPKMSConfig struct {
	Project  string `yaml:"project" mapstructure:"project"`
	Location string `yaml:"location" mapstructure:"location"`
	Keyring  string `yaml:"keyring" mapstructure:"keyring"`
	Key      string `yaml:"key" mapstructure:"key"`
}

// VaultConfig holds HashiCorp Vault-specific settings.
type VaultConfig struct {
	Address   string `yaml:"address" mapstructure:"address"`
	Token     string `yaml:"token" mapstructure:"token"`
	MountPath string `yaml:"mount_path" mapstructure:"mount_path"`
}

// DefaultConfig returns sensible defaults for secret management.
func DefaultConfig() Config {
	return Config{
		Provider:  ProviderEnv,
		KeyPrefix: "PLAYGROUND_",
	}
}

// NewProvider creates a Provider for the given configuration.
// Currently the env provider is built-in; aws-kms, gcp-kms, and vault are
// supported as extension points and will return an error until their SDK
// adapters are registered.
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case ProviderEnv, "":
		return NewEnvProvider(cfg.KeyPrefix), nil
	case ProviderAWSKMS:
		return nil, fmt.Errorf("aws-kms provider: not yet linked — register an AWSKMSProvider implementation")
	case ProviderGCPKMS:
		return nil, fmt.Errorf("gcp-kms provider: not yet linked — register a GCPKMSProvider implementation")
	case ProviderVault:
		return nil, fmt.Errorf("vault provider: not yet linked — register a VaultProvider implementation")
	default:
		return nil, fmt.Errorf("unknown secrets provider type: %q", cfg.Provider)
	}
}
