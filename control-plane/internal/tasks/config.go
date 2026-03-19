package tasks

import "os"

// TemporalConfig holds Temporal connection settings.
// In production, tasks are executed durably via tasks.hanzo.ai (Temporal Cloud).
type TemporalConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Address   string `json:"address" yaml:"address" mapstructure:"address"`
	Namespace string `json:"namespace" yaml:"namespace" mapstructure:"namespace"`
}

// DefaultTemporalConfig returns sensible defaults.
// When enabled, connects to tasks.hanzo.ai for durable cloud task execution.
// Override with PLAYGROUND_TEMPORAL_ADDRESS for local dev (e.g. localhost:7233).
func DefaultTemporalConfig() TemporalConfig {
	cfg := TemporalConfig{
		Enabled:   false,
		Address:   "tasks.hanzo.ai:7233",
		Namespace: "hanzo",
	}

	if v := os.Getenv("PLAYGROUND_TEMPORAL_ENABLED"); v == "true" || v == "1" {
		cfg.Enabled = true
	}
	if v := os.Getenv("PLAYGROUND_TEMPORAL_ADDRESS"); v != "" {
		cfg.Address = v
	}
	if v := os.Getenv("PLAYGROUND_TEMPORAL_NAMESPACE"); v != "" {
		cfg.Namespace = v
	}

	return cfg
}
