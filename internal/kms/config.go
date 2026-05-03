package kms

import "os"

// Config holds configuration for the KMS MPC integration.
type Config struct {
	// Enabled controls whether KMS/MPC wallet operations are active.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`

	// Endpoint is the base URL of the Lux KMS MPC daemon.
	Endpoint string `yaml:"endpoint" mapstructure:"endpoint"`

	// Token is the bearer token for authenticating with the MPC daemon.
	Token string `yaml:"token" mapstructure:"token"`

	// VaultID is the default MPC vault for key generation.
	VaultID string `yaml:"vault_id" mapstructure:"vault_id"`

	// DefaultThreshold is the default signing threshold for new wallets.
	DefaultThreshold int `yaml:"default_threshold" mapstructure:"default_threshold"`

	// DefaultParties is the default number of MPC participants for new wallets.
	DefaultParties int `yaml:"default_parties" mapstructure:"default_parties"`
}

// DefaultConfig returns a Config with sensible production defaults.
// KMS is disabled by default — callers must opt in via config or env vars.
func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		Endpoint:         "https://kms.hanzo.ai",
		VaultID:          "default",
		DefaultThreshold: 2,
		DefaultParties:   3,
	}
}

// ApplyEnvOverrides reads PLAYGROUND_KMS_* environment variables and applies
// them over the current config values. Non-empty env vars take precedence.
func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("PLAYGROUND_KMS_ENDPOINT"); v != "" {
		c.Endpoint = v
	}
	if v := os.Getenv("PLAYGROUND_KMS_TOKEN"); v != "" {
		c.Token = v
	}
	if v := os.Getenv("PLAYGROUND_KMS_VAULT_ID"); v != "" {
		c.VaultID = v
	}
}
