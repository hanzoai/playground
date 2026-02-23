package config

import (
	"os"
	"strconv"
	"time"
)

// CloudConfig holds configuration for cloud agent provisioning.
type CloudConfig struct {
	Enabled    bool             `yaml:"enabled" mapstructure:"enabled"`
	Kubernetes KubernetesConfig `yaml:"kubernetes" mapstructure:"kubernetes"`
	Visor      VisorConfig      `yaml:"visor" mapstructure:"visor"`
}

// KubernetesConfig holds K8s-specific provisioning settings.
type KubernetesConfig struct {
	Enabled          bool              `yaml:"enabled" mapstructure:"enabled"`
	Namespace        string            `yaml:"namespace" mapstructure:"namespace"`
	BotImage         string            `yaml:"agent_image" mapstructure:"agent_image"`
	ImagePullSecret  string            `yaml:"image_pull_secret" mapstructure:"image_pull_secret"`
	ServiceAccount   string            `yaml:"service_account" mapstructure:"service_account"`
	NodeSelector     map[string]string `yaml:"node_selector" mapstructure:"node_selector"`
	MaxAgentsPerOrg  int               `yaml:"max_agents_per_org" mapstructure:"max_agents_per_org"`
	DefaultCPU       string            `yaml:"default_cpu" mapstructure:"default_cpu"`
	DefaultMemory    string            `yaml:"default_memory" mapstructure:"default_memory"`
	LimitCPU         string            `yaml:"limit_cpu" mapstructure:"limit_cpu"`
	LimitMemory      string            `yaml:"limit_memory" mapstructure:"limit_memory"`
	PodTTL           time.Duration     `yaml:"pod_ttl" mapstructure:"pod_ttl"`
	GracefulShutdown time.Duration     `yaml:"graceful_shutdown" mapstructure:"graceful_shutdown"`
	// Operative sidecar (full Linux desktop: noVNC + Xvfb)
	OperativeEnabled bool   `yaml:"operative_enabled" mapstructure:"operative_enabled"`
	OperativeImage   string `yaml:"operative_image" mapstructure:"operative_image"`
	// Hanzo Cloud AI backend for bot LLM calls
	CloudAPIEndpoint string `yaml:"cloud_api_endpoint" mapstructure:"cloud_api_endpoint"`
	CloudAPIKey      string `yaml:"cloud_api_key" mapstructure:"cloud_api_key"`
	// Central gateway: cloud pods connect as nodes to the shared bot-gateway
	// so all nodes (local Mac + cloud pods) appear in one unified gateway.
	GatewayURL   string `yaml:"gateway_url" mapstructure:"gateway_url"`     // ws://bot-gateway.hanzo.svc:18789
	GatewayToken string `yaml:"gateway_token" mapstructure:"gateway_token"` // Shared auth token
}

// VisorConfig holds configuration for Visor multi-cloud VM provisioning.
// Visor manages VMs across AWS EC2, DigitalOcean, GCP, Azure, Proxmox, etc.
// and provides remote access (RDP/VNC/SSH) via Guacamole tunnels.
type VisorConfig struct {
	Enabled      bool   `yaml:"enabled" mapstructure:"enabled"`
	Endpoint     string `yaml:"endpoint" mapstructure:"endpoint"`         // http://visor.hanzo.svc:19000
	ClientID     string `yaml:"client_id" mapstructure:"client_id"`       // Visor IAM client ID
	ClientSecret string `yaml:"client_secret" mapstructure:"client_secret"` // Visor IAM client secret
}

// IAMConfig holds configuration for Hanzo IAM integration.
type IAMConfig struct {
	Enabled        bool   `yaml:"enabled" mapstructure:"enabled"`
	Endpoint       string `yaml:"endpoint" mapstructure:"endpoint"`               // Internal: http://iam.hanzo.svc:8000
	PublicEndpoint string `yaml:"public_endpoint" mapstructure:"public_endpoint"` // Public: https://hanzo.id
	ClientID       string `yaml:"client_id" mapstructure:"client_id"`
	ClientSecret   string `yaml:"client_secret" mapstructure:"client_secret"`
	Organization   string `yaml:"organization" mapstructure:"organization"`
	Application    string `yaml:"application" mapstructure:"application"`
}

// DefaultCloudConfig returns sensible defaults for cloud provisioning.
func DefaultCloudConfig() CloudConfig {
	return CloudConfig{
		Enabled: false,
		Kubernetes: KubernetesConfig{
			Enabled:          false,
			Namespace:        "hanzo",
			BotImage:         "ghcr.io/hanzoai/bot:latest",
			ImagePullSecret:  "ghcr-secret",
			ServiceAccount:   "playground-agent",
			MaxAgentsPerOrg:  20,
			DefaultCPU:       "250m",
			DefaultMemory:    "512Mi",
			LimitCPU:         "2000m",
			LimitMemory:      "4Gi",
			PodTTL:           24 * time.Hour,
			GracefulShutdown: 30 * time.Second,
			OperativeEnabled: true,
			OperativeImage:   "ghcr.io/hanzoai/operative:latest",
			CloudAPIEndpoint: "https://api.hanzo.ai/v1",
			GatewayURL:       "ws://bot-gateway.hanzo.svc:18789",
		},
		Visor: VisorConfig{
			Enabled:  false,
			Endpoint: "http://visor.hanzo.svc:19000",
		},
	}
}

// DefaultIAMConfig returns sensible defaults for IAM.
func DefaultIAMConfig() IAMConfig {
	return IAMConfig{
		Enabled:        false,
		Endpoint:       "http://iam.hanzo.svc:8000",
		PublicEndpoint: "https://hanzo.id",
		Organization:   "hanzo",
		Application:    "app-hanzobot",
	}
}

// hanzoEnvWithFallback reads HANZO_PLAYGROUND_<key> first, then falls back to HANZO_AGENTS_<key>.
func hanzoEnvWithFallback(key string) string {
	if v := os.Getenv("HANZO_PLAYGROUND_" + key); v != "" {
		return v
	}
	return os.Getenv("HANZO_AGENTS_" + key)
}

// applyCloudEnvOverrides loads cloud config from HANZO_PLAYGROUND_* environment variables
// (with HANZO_AGENTS_* fallback for backward compatibility).
func applyCloudEnvOverrides(cfg *Config) {
	// Cloud
	if v := hanzoEnvWithFallback("CLOUD_ENABLED"); v != "" {
		cfg.Cloud.Enabled = v == "true" || v == "1"
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_ENABLED"); v != "" {
		cfg.Cloud.Kubernetes.Enabled = v == "true" || v == "1"
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_NAMESPACE"); v != "" {
		cfg.Cloud.Kubernetes.Namespace = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_AGENT_IMAGE"); v != "" {
		cfg.Cloud.Kubernetes.BotImage = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_IMAGE_PULL_SECRET"); v != "" {
		cfg.Cloud.Kubernetes.ImagePullSecret = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_SERVICE_ACCOUNT"); v != "" {
		cfg.Cloud.Kubernetes.ServiceAccount = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_MAX_AGENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Cloud.Kubernetes.MaxAgentsPerOrg = n
		}
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_DEFAULT_CPU"); v != "" {
		cfg.Cloud.Kubernetes.DefaultCPU = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_DEFAULT_MEMORY"); v != "" {
		cfg.Cloud.Kubernetes.DefaultMemory = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_LIMIT_CPU"); v != "" {
		cfg.Cloud.Kubernetes.LimitCPU = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_LIMIT_MEMORY"); v != "" {
		cfg.Cloud.Kubernetes.LimitMemory = v
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_POD_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Cloud.Kubernetes.PodTTL = d
		}
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_OPERATIVE_ENABLED"); v != "" {
		cfg.Cloud.Kubernetes.OperativeEnabled = v == "true" || v == "1"
	}
	if v := hanzoEnvWithFallback("CLOUD_K8S_OPERATIVE_IMAGE"); v != "" {
		cfg.Cloud.Kubernetes.OperativeImage = v
	}
	if v := hanzoEnvWithFallback("CLOUD_API_ENDPOINT"); v != "" {
		cfg.Cloud.Kubernetes.CloudAPIEndpoint = v
	}
	if v := hanzoEnvWithFallback("CLOUD_API_KEY"); v != "" {
		cfg.Cloud.Kubernetes.CloudAPIKey = v
	}
	if v := hanzoEnvWithFallback("CLOUD_GATEWAY_URL"); v != "" {
		cfg.Cloud.Kubernetes.GatewayURL = v
	}
	if v := hanzoEnvWithFallback("CLOUD_GATEWAY_TOKEN"); v != "" {
		cfg.Cloud.Kubernetes.GatewayToken = v
	}

	// Visor (multi-cloud VM provisioning)
	if v := hanzoEnvWithFallback("VISOR_ENABLED"); v != "" {
		cfg.Cloud.Visor.Enabled = v == "true" || v == "1"
	}
	if v := hanzoEnvWithFallback("VISOR_ENDPOINT"); v != "" {
		cfg.Cloud.Visor.Endpoint = v
	}
	if v := hanzoEnvWithFallback("VISOR_CLIENT_ID"); v != "" {
		cfg.Cloud.Visor.ClientID = v
	}
	if v := hanzoEnvWithFallback("VISOR_CLIENT_SECRET"); v != "" {
		cfg.Cloud.Visor.ClientSecret = v
	}

	// IAM
	if v := hanzoEnvWithFallback("IAM_ENABLED"); v != "" {
		cfg.IAM.Enabled = v == "true" || v == "1"
	}
	if v := hanzoEnvWithFallback("IAM_ENDPOINT"); v != "" {
		cfg.IAM.Endpoint = v
	}
	if v := hanzoEnvWithFallback("IAM_PUBLIC_ENDPOINT"); v != "" {
		cfg.IAM.PublicEndpoint = v
	}
	if v := hanzoEnvWithFallback("IAM_CLIENT_ID"); v != "" {
		cfg.IAM.ClientID = v
	}
	if v := os.Getenv("IAM_CLIENT_SECRET"); v != "" {
		cfg.IAM.ClientSecret = v
	}
	if v := hanzoEnvWithFallback("IAM_CLIENT_SECRET"); v != "" {
		cfg.IAM.ClientSecret = v
	}
	if v := hanzoEnvWithFallback("IAM_ORGANIZATION"); v != "" {
		cfg.IAM.Organization = v
	}
	if v := hanzoEnvWithFallback("IAM_APPLICATION"); v != "" {
		cfg.IAM.Application = v
	}
}
