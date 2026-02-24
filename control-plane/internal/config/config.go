package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"

	"github.com/hanzoai/playground/control-plane/internal/secrets"
	"github.com/hanzoai/playground/control-plane/internal/storage"
)

// Config holds the entire configuration for the Playground server.
type Config struct {
	Agents   PlaygroundConfig `yaml:"agents" mapstructure:"agents"`
	Features FeatureConfig    `yaml:"features" mapstructure:"features"`
	Storage  StorageConfig    `yaml:"storage" mapstructure:"storage"`
	UI       UIConfig         `yaml:"ui" mapstructure:"ui"`
	API      APIConfig        `yaml:"api" mapstructure:"api"`
	Cloud    CloudConfig      `yaml:"cloud" mapstructure:"cloud"`
	IAM      IAMConfig        `yaml:"iam" mapstructure:"iam"`
	Secrets  secrets.Config   `yaml:"secrets" mapstructure:"secrets"`
}

// UIConfig holds configuration for the web UI.
type UIConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	Mode       string `yaml:"mode" mapstructure:"mode"`               // "embedded", "dev", "separate"
	SourcePath string `yaml:"source_path" mapstructure:"source_path"` // Path to UI source for building
	DistPath   string `yaml:"dist_path" mapstructure:"dist_path"`     // Path to built UI assets for serving
	DevPort    int    `yaml:"dev_port" mapstructure:"dev_port"`       // Port for UI dev server
}

// PlaygroundConfig holds the core Playground server configuration.
type PlaygroundConfig struct {
	Port             int                    `yaml:"port"`
	NodeHealth       NodeHealthConfig       `yaml:"node_health" mapstructure:"node_health"`
	ExecutionCleanup ExecutionCleanupConfig `yaml:"execution_cleanup" mapstructure:"execution_cleanup"`
	ExecutionQueue   ExecutionQueueConfig   `yaml:"execution_queue" mapstructure:"execution_queue"`
}

// NodeHealthConfig holds configuration for Node health monitoring.
// Zero values are treated as "use default" — set explicitly to override.
type NodeHealthConfig struct {
	CheckInterval           time.Duration `yaml:"check_interval" mapstructure:"check_interval"`                       // How often to HTTP health check nodes (0 = default 10s)
	CheckTimeout            time.Duration `yaml:"check_timeout" mapstructure:"check_timeout"`                         // Timeout per HTTP health check (0 = default 5s)
	ConsecutiveFailures     int           `yaml:"consecutive_failures" mapstructure:"consecutive_failures"`            // Failures before marking inactive (0 = default 3; set 1 for instant)
	RecoveryDebounce        time.Duration `yaml:"recovery_debounce" mapstructure:"recovery_debounce"`                 // Wait before allowing inactive->active (0 = default 5s)
	HeartbeatStaleThreshold time.Duration `yaml:"heartbeat_stale_threshold" mapstructure:"heartbeat_stale_threshold"` // Heartbeat age before marking stale (0 = default 60s)
}

// ExecutionCleanupConfig holds configuration for execution cleanup and garbage collection
type ExecutionCleanupConfig struct {
	Enabled                bool          `yaml:"enabled" mapstructure:"enabled" default:"true"`
	RetentionPeriod        time.Duration `yaml:"retention_period" mapstructure:"retention_period" default:"24h"`
	CleanupInterval        time.Duration `yaml:"cleanup_interval" mapstructure:"cleanup_interval" default:"1h"`
	BatchSize              int           `yaml:"batch_size" mapstructure:"batch_size" default:"100"`
	PreserveRecentDuration time.Duration `yaml:"preserve_recent_duration" mapstructure:"preserve_recent_duration" default:"1h"`
	StaleExecutionTimeout  time.Duration `yaml:"stale_execution_timeout" mapstructure:"stale_execution_timeout" default:"30m"`
}

// ExecutionQueueConfig configures execution and webhook settings.
type ExecutionQueueConfig struct {
	AgentCallTimeout       time.Duration `yaml:"agent_call_timeout" mapstructure:"agent_call_timeout"`
	WebhookTimeout         time.Duration `yaml:"webhook_timeout" mapstructure:"webhook_timeout"`
	WebhookMaxAttempts     int           `yaml:"webhook_max_attempts" mapstructure:"webhook_max_attempts"`
	WebhookRetryBackoff    time.Duration `yaml:"webhook_retry_backoff" mapstructure:"webhook_retry_backoff"`
	WebhookMaxRetryBackoff time.Duration `yaml:"webhook_max_retry_backoff" mapstructure:"webhook_max_retry_backoff"`
}

// FeatureConfig holds configuration for enabling/disabling features.
type FeatureConfig struct {
	DID DIDConfig `yaml:"did" mapstructure:"did"`
}

// DIDConfig holds configuration for DID identity system.
type DIDConfig struct {
	Enabled          bool           `yaml:"enabled" mapstructure:"enabled" default:"true"`
	Method           string         `yaml:"method" mapstructure:"method" default:"did:key"`
	KeyAlgorithm     string         `yaml:"key_algorithm" mapstructure:"key_algorithm" default:"Ed25519"`
	DerivationMethod string         `yaml:"derivation_method" mapstructure:"derivation_method" default:"BIP32"`
	KeyRotationDays  int            `yaml:"key_rotation_days" mapstructure:"key_rotation_days" default:"90"`
	VCRequirements   VCRequirements `yaml:"vc_requirements" mapstructure:"vc_requirements"`
	Keystore         KeystoreConfig `yaml:"keystore" mapstructure:"keystore"`
}

// VCRequirements holds VC generation requirements.
type VCRequirements struct {
	RequireVCForRegistration bool   `yaml:"require_vc_registration" mapstructure:"require_vc_registration" default:"true"`
	RequireVCForExecution    bool   `yaml:"require_vc_execution" mapstructure:"require_vc_execution" default:"true"`
	RequireVCForCrossAgent   bool   `yaml:"require_vc_cross_agent" mapstructure:"require_vc_cross_agent" default:"true"`
	StoreInputOutput         bool   `yaml:"store_input_output" mapstructure:"store_input_output" default:"false"`
	HashSensitiveData        bool   `yaml:"hash_sensitive_data" mapstructure:"hash_sensitive_data" default:"true"`
	PersistExecutionVC       bool   `yaml:"persist_execution_vc" mapstructure:"persist_execution_vc" default:"true"`
	StorageMode              string `yaml:"storage_mode" mapstructure:"storage_mode" default:"inline"`
}

// KeystoreConfig holds keystore configuration.
type KeystoreConfig struct {
	Type           string `yaml:"type" mapstructure:"type" default:"local"`
	Path           string `yaml:"path" mapstructure:"path" default:"./data/keys"`
	Encryption     string `yaml:"encryption" mapstructure:"encryption" default:"AES-256-GCM"`
	BackupEnabled  bool   `yaml:"backup_enabled" mapstructure:"backup_enabled" default:"true"`
	BackupInterval string `yaml:"backup_interval" mapstructure:"backup_interval" default:"24h"`
}

// APIConfig holds configuration for API settings
type APIConfig struct {
	CORS CORSConfig `yaml:"cors" mapstructure:"cors"`
	Auth AuthConfig `yaml:"auth" mapstructure:"auth"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins" mapstructure:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods" mapstructure:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers" mapstructure:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers" mapstructure:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials" mapstructure:"allow_credentials"`
}

// AuthConfig holds API authentication configuration.
type AuthConfig struct {
	// APIKey is checked against headers or query params. Empty disables auth.
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// SkipPaths allows bypassing auth for specific endpoints (e.g., health).
	SkipPaths []string `yaml:"skip_paths" mapstructure:"skip_paths"`
}

// StorageConfig is an alias of the storage layer's configuration so callers can
// work with a single definition while keeping the canonical struct colocated
// with the implementation in the storage package.
type StorageConfig = storage.StorageConfig

// DefaultConfigPath is the default path for the playground configuration file.
const DefaultConfigPath = "playground.yaml"

// LoadConfig reads the configuration from the given path or default paths.
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	// Check if the specific path exists, with fallback to legacy names
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try common locations: config/playground.yaml, then legacy agents.yaml
		fallbacks := []string{
			filepath.Join("config", "playground.yaml"),
			"agents.yaml",
			filepath.Join("config", "agents.yaml"),
		}
		found := false
		for _, alt := range fallbacks {
			if _, err2 := os.Stat(alt); err2 == nil {
				configPath = alt
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("configuration file not found at %s or default locations: %w", configPath, err)
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", configPath, err)
	}

	// First unmarshal into a generic map so we can decode via mapstructure
	// which supports time.Duration string parsing (e.g. "24h", "90s").
	var rawMap map[string]any
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file %s: %w", configPath, err)
	}

	var cfg Config
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
		Result:           &cfg,
		TagName:          "yaml",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create config decoder: %w", err)
	}
	if err := decoder.Decode(rawMap); err != nil {
		return nil, fmt.Errorf("failed to decode configuration from %s: %w", configPath, err)
	}

	// Apply defaults for new config sections
	if cfg.Cloud.Kubernetes.Namespace == "" {
		defaults := DefaultCloudConfig()
		if !cfg.Cloud.Enabled {
			cfg.Cloud = defaults
		}
	}
	if cfg.IAM.Endpoint == "" {
		cfg.IAM = DefaultIAMConfig()
	}

	// Apply defaults for the secrets subsystem.
	if cfg.Secrets.Provider == "" {
		cfg.Secrets = secrets.DefaultConfig()
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)
	applyCloudEnvOverrides(&cfg)

	// Resolve any secret:// references in sensitive config fields via the
	// configured secrets provider (env vars by default).
	if err := resolveSecrets(&cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}

	return &cfg, nil
}

// envWithFallback reads PLAYGROUND_<key> first, then falls back to AGENTS_<key>.
func envWithFallback(key string) string {
	if v := os.Getenv("PLAYGROUND_" + key); v != "" {
		return v
	}
	return os.Getenv("AGENTS_" + key)
}

// ApplyAllEnvOverrides applies all environment variable overrides to the config.
// Call this after unmarshalling config (from viper or YAML) to pick up
// PLAYGROUND_*, AGENTS_*, and HANZO_PLAYGROUND_* env vars.
func ApplyAllEnvOverrides(cfg *Config) {
	applyEnvOverrides(cfg)
	applyCloudEnvOverrides(cfg)
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables take precedence over YAML config values.
// Reads PLAYGROUND_* first, falls back to AGENTS_* for backward compatibility.
func applyEnvOverrides(cfg *Config) {
	// API Authentication
	if apiKey := envWithFallback("API_KEY"); apiKey != "" {
		cfg.API.Auth.APIKey = apiKey
	}
	// Also support the nested path format for consistency
	if apiKey := envWithFallback("API_AUTH_API_KEY"); apiKey != "" {
		cfg.API.Auth.APIKey = apiKey
	}

	// Node health monitoring overrides
	if val := envWithFallback("HEALTH_CHECK_INTERVAL"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Agents.NodeHealth.CheckInterval = d
		}
	}
	if val := envWithFallback("HEALTH_CHECK_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Agents.NodeHealth.CheckTimeout = d
		}
	}
	if val := envWithFallback("HEALTH_CONSECUTIVE_FAILURES"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			cfg.Agents.NodeHealth.ConsecutiveFailures = i
		}
	}
	if val := envWithFallback("HEALTH_RECOVERY_DEBOUNCE"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Agents.NodeHealth.RecoveryDebounce = d
		}
	}
	if val := envWithFallback("HEARTBEAT_STALE_THRESHOLD"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.Agents.NodeHealth.HeartbeatStaleThreshold = d
		}
	}

	// Secrets provider type override
	if val := envWithFallback("SECRETS_PROVIDER"); val != "" {
		cfg.Secrets.Provider = secrets.ProviderType(val)
	}
}

// resolveSecrets walks the sensitive config fields and resolves any that carry
// the "secret://" prefix through the configured secrets.Provider.  Plain-text
// values (including those already set via env-var overrides) pass through
// unchanged, preserving full backward compatibility.
func resolveSecrets(cfg *Config) error {
	provider, err := secrets.NewProvider(cfg.Secrets)
	if err != nil {
		return err
	}

	resolver := secrets.NewResolver(provider)
	ctx := context.Background()

	// Resolve each sensitive field.  When the field value does not start with
	// "secret://" the resolver returns it verbatim, so this is always safe.
	fields := []*string{
		&cfg.API.Auth.APIKey,
		&cfg.Cloud.Kubernetes.CloudAPIKey,
		&cfg.Cloud.Visor.ClientID,
		&cfg.Cloud.Visor.ClientSecret,
		&cfg.IAM.ClientID,
		&cfg.IAM.ClientSecret,
	}

	for _, f := range fields {
		resolved, resolveErr := resolver.Resolve(ctx, *f)
		if resolveErr != nil {
			return resolveErr
		}
		*f = resolved
	}

	return nil
}
