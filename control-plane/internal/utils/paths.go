package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataDirectories holds all the standardized paths for Agents data storage
type DataDirectories struct {
	AgentsHome   string
	DataDir          string
	DatabaseDir      string
	KeysDir          string
	DIDRegistriesDir string
	VCsDir           string
	VCsExecutionsDir string
	VCsWorkflowsDir  string
	AgentsDir        string
	LogsDir          string
	ConfigDir        string
	TempDir          string
	PayloadsDir      string
}

// GetAgentsDataDirectories returns the standardized data directories for Agents
// It respects environment variables and provides sensible defaults
func GetAgentsDataDirectories() (*DataDirectories, error) {
	// Determine Agents home directory
	agentsHome := os.Getenv("AGENTS_HOME")
	if agentsHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		agentsHome = filepath.Join(homeDir, ".hanzo/agents")
	}

	// Create the data directories structure
	dirs := &DataDirectories{
		AgentsHome:   agentsHome,
		DataDir:          filepath.Join(agentsHome, "data"),
		DatabaseDir:      filepath.Join(agentsHome, "data"),
		KeysDir:          filepath.Join(agentsHome, "data", "keys"),
		DIDRegistriesDir: filepath.Join(agentsHome, "data", "did_registries"),
		VCsDir:           filepath.Join(agentsHome, "data", "vcs"),
		VCsExecutionsDir: filepath.Join(agentsHome, "data", "vcs", "executions"),
		VCsWorkflowsDir:  filepath.Join(agentsHome, "data", "vcs", "workflows"),
		AgentsDir:        filepath.Join(agentsHome, "agents"),
		LogsDir:          filepath.Join(agentsHome, "logs"),
		ConfigDir:        filepath.Join(agentsHome, "config"),
		TempDir:          filepath.Join(agentsHome, "temp"),
		PayloadsDir:      filepath.Join(agentsHome, "data", "payloads"),
	}

	return dirs, nil
}

// EnsureDataDirectories creates all necessary Agents data directories
func EnsureDataDirectories() (*DataDirectories, error) {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return nil, err
	}

	// Create all directories with appropriate permissions
	directoriesToCreate := []string{
		dirs.AgentsHome,
		dirs.DataDir,
		dirs.DatabaseDir,
		dirs.KeysDir,
		dirs.DIDRegistriesDir,
		dirs.VCsDir,
		dirs.VCsExecutionsDir,
		dirs.VCsWorkflowsDir,
		dirs.AgentsDir,
		dirs.LogsDir,
		dirs.ConfigDir,
		dirs.TempDir,
		dirs.PayloadsDir,
	}

	for _, dir := range directoriesToCreate {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// Set more restrictive permissions for sensitive directories
	sensitiveDirectories := []string{
		dirs.KeysDir,
		dirs.DIDRegistriesDir,
	}

	for _, dir := range sensitiveDirectories {
		if err := os.Chmod(dir, 0700); err != nil {
			return nil, err
		}
	}

	return dirs, nil
}

// GetDatabasePath returns the path to the main Agents database
func GetDatabasePath() (string, error) {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.DatabaseDir, "agents.db"), nil
}

// GetKVStorePath returns the path to the Agents key-value store
func GetKVStorePath() (string, error) {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.DatabaseDir, "agents.bolt"), nil
}

// GetAgentRegistryPath returns the path to the agent registry file
func GetAgentRegistryPath() (string, error) {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.AgentsHome, "installed.json"), nil
}

// GetConfigPath returns the path to a configuration file
func GetConfigPath(filename string) (string, error) {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.ConfigDir, filename), nil
}

// GetLogPath returns the path to a log file
func GetLogPath(filename string) (string, error) {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.LogsDir, filename), nil
}

// GetTempPath returns the path to a temporary file
func GetTempPath(filename string) (string, error) {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.TempDir, filename), nil
}

// GetPlatformSpecificPaths returns platform-specific paths if needed
func GetPlatformSpecificPaths() map[string]string {
	paths := make(map[string]string)

	switch runtime.GOOS {
	case "windows":
		// Windows-specific paths if needed
		paths["app_data"] = os.Getenv("APPDATA")
		paths["local_app_data"] = os.Getenv("LOCALAPPDATA")
	case "darwin":
		// macOS-specific paths if needed
		homeDir, _ := os.UserHomeDir()
		paths["application_support"] = filepath.Join(homeDir, "Library", "Application Support")
		paths["caches"] = filepath.Join(homeDir, "Library", "Caches")
	case "linux":
		// Linux-specific paths if needed
		paths["xdg_config_home"] = os.Getenv("XDG_CONFIG_HOME")
		paths["xdg_data_home"] = os.Getenv("XDG_DATA_HOME")
		paths["xdg_cache_home"] = os.Getenv("XDG_CACHE_HOME")
	}

	return paths
}

// ValidatePaths checks if all required paths are accessible
func ValidatePaths() error {
	dirs, err := GetAgentsDataDirectories()
	if err != nil {
		return err
	}

	// Check if we can write to the Agents home directory
	testFile := filepath.Join(dirs.AgentsHome, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return err
	}
	os.Remove(testFile)

	return nil
}
