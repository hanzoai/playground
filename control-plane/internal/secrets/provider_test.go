package secrets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewProvider_Env(t *testing.T) {
	p, err := NewProvider(Config{Provider: ProviderEnv})
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, ProviderEnv, p.Type())
}

func TestNewProvider_EmptyDefaultsToEnv(t *testing.T) {
	p, err := NewProvider(Config{})
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, ProviderEnv, p.Type())
}

func TestNewProvider_AWSKMS_NotLinked(t *testing.T) {
	_, err := NewProvider(Config{Provider: ProviderAWSKMS})
	require.Error(t, err)
	require.Contains(t, err.Error(), "aws-kms")
}

func TestNewProvider_GCPKMS_NotLinked(t *testing.T) {
	_, err := NewProvider(Config{Provider: ProviderGCPKMS})
	require.Error(t, err)
	require.Contains(t, err.Error(), "gcp-kms")
}

func TestNewProvider_Vault_NotLinked(t *testing.T) {
	_, err := NewProvider(Config{Provider: ProviderVault})
	require.Error(t, err)
	require.Contains(t, err.Error(), "vault")
}

func TestNewProvider_UnknownType(t *testing.T) {
	_, err := NewProvider(Config{Provider: "foo"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown secrets provider type")
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.Equal(t, ProviderEnv, cfg.Provider)
	require.Equal(t, "PLAYGROUND_", cfg.KeyPrefix)
}
