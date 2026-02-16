package packages

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// NodeRunner handles running hanzo nodes
type NodeRunner struct {
	AgentsHome string
	Port           int
	Detach         bool
}

// RunNode starts an installed hanzo node
func (ar *NodeRunner) RunNode(nodeName string) error {
	fmt.Printf("🚀 Launching hanzo node: %s\n", nodeName)

	// 1. Check if hanzo node is installed
	registry, err := ar.loadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	node, exists := registry.Installed[nodeName]
	if !exists {
		return fmt.Errorf("hanzo node %s not installed", nodeName)
	}

	// 2. Check if already running
	if node.Status == "running" {
		return fmt.Errorf("hanzo node %s is already running on port %d", nodeName, *node.Runtime.Port)
	}

	// 3. Allocate port
	fmt.Printf("🔍 Searching for available port...\n")
	port := ar.Port
	if port == 0 {
		port, err = ar.getFreePort()
		if err != nil {
			return fmt.Errorf("failed to allocate port: %w", err)
		}
	}

	fmt.Printf("✅ Assigned port: %d\n", port)

	// 4. Start hanzo node process
	fmt.Printf("📡 Starting hanzo node process...\n")
	cmd, err := ar.startNodeProcess(node, port)
	if err != nil {
		return fmt.Errorf("failed to start hanzo node: %w", err)
	}

	// 5. Wait for hanzo node to be ready
	if err := ar.waitForNode(port, 10*time.Second); err != nil {
		if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			fmt.Printf("⚠️  Failed to kill hanzo node process: %v\n", killErr)
		}
		return fmt.Errorf("hanzo node failed to start: %w", err)
	}

	fmt.Printf("🧠 node registered with Playground Server\n")

	// 6. Update registry with runtime info
	if err := ar.updateRuntimeInfo(nodeName, port, cmd.Process.Pid); err != nil {
		return fmt.Errorf("failed to update runtime info: %w", err)
	}

	// 7. Display hanzo node capabilities
	if err := ar.displayCapabilities(node, port); err != nil {
		fmt.Printf("⚠️  Could not fetch capabilities: %v\n", err)
	}

	fmt.Printf("\n💡 node running in background (PID: %d)\n", cmd.Process.Pid)
	fmt.Printf("💡 View logs: playground logs %s\n", nodeName)
	fmt.Printf("💡 Stop hanzo node: playground stop %s\n", nodeName)

	return nil
}

// getFreePort finds an available port in the range 8001-8999
func (ar *NodeRunner) getFreePort() (int, error) {
	for port := 8001; port <= 8999; port++ {
		if ar.isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port available in range 8001-8999")
}

// isPortAvailable checks if a port is available
func (ar *NodeRunner) isPortAvailable(port int) bool {
	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// startNodeProcess starts the hanzo node process
func (ar *NodeRunner) startNodeProcess(node InstalledPackage, port int) (*exec.Cmd, error) {
	// Prepare environment variables
	env := os.Environ()
	env = append(env, fmt.Sprintf("PORT=%d", port))
	env = append(env, "PLAYGROUND_SERVER_URL=http://localhost:8080")

	// Load environment variables from package .env file
	if envVars, err := ar.loadPackageEnvFile(node.Path); err == nil {
		for key, value := range envVars {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		fmt.Printf("🔧 Loaded %d environment variables from .env file\n", len(envVars))
	}

	// Prepare command - use virtual environment if available
	var pythonPath string
	venvPath := filepath.Join(node.Path, "venv")

	// Check if virtual environment exists
	if _, err := os.Stat(filepath.Join(venvPath, "bin", "python")); err == nil {
		pythonPath = filepath.Join(venvPath, "bin", "python")
		fmt.Printf("🐍 Using virtual environment: %s\n", venvPath)
	} else if _, err := os.Stat(filepath.Join(venvPath, "Scripts", "python.exe")); err == nil {
		pythonPath = filepath.Join(venvPath, "Scripts", "python.exe") // Windows
		fmt.Printf("🐍 Using virtual environment: %s\n", venvPath)
	} else {
		// Fallback to system python
		pythonPath = "python"
		fmt.Printf("⚠️  Virtual environment not found, using system Python\n")
	}

	cmd := exec.Command(pythonPath, "main.py")
	cmd.Dir = node.Path
	cmd.Env = env

	// Setup logging
	logFile, err := os.OpenFile(node.Runtime.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	return cmd, nil
}

// waitForNode waits for the hanzo node to become ready
func (ar *NodeRunner) waitForNode(port int, timeout time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("hanzo node did not become ready within %v", timeout)
}

// displayCapabilities fetches and displays hanzo node capabilities
func (ar *NodeRunner) displayCapabilities(node InstalledPackage, port int) error {
	client := &http.Client{Timeout: 5 * time.Second}

	// Get bots
	botsResp, err := client.Get(fmt.Sprintf("http://localhost:%d/bots", port))
	if err != nil {
		return err
	}
	defer botsResp.Body.Close()

	var botsData map[string]interface{}
	if err := json.NewDecoder(botsResp.Body).Decode(&botsData); err != nil {
		return err
	}

	// Get skills
	skillsResp, err := client.Get(fmt.Sprintf("http://localhost:%d/skills", port))
	if err != nil {
		return err
	}
	defer skillsResp.Body.Close()

	var skillsData map[string]interface{}
	if err := json.NewDecoder(skillsResp.Body).Decode(&skillsData); err != nil {
		return err
	}

	fmt.Printf("\n🌐 Access locally at: http://localhost:%d\n", port)
	fmt.Printf("📖 Available functions:\n")

	// Display bots
	if bots, ok := botsData["bots"].([]interface{}); ok && len(bots) > 0 {
		fmt.Printf("  🧠 Bots: ")
		var botNames []string
		for _, bot := range bots {
			if r, ok := bot.(map[string]interface{}); ok {
				if id, ok := r["id"].(string); ok {
					botNames = append(botNames, id)
				}
			}
		}
		fmt.Printf("%s\n", strings.Join(botNames, ", "))
	}

	// Display skills
	if skills, ok := skillsData["skills"].([]interface{}); ok && len(skills) > 0 {
		fmt.Printf("  🛠️  Skills:    ")
		var skillNames []string
		for _, skill := range skills {
			if s, ok := skill.(map[string]interface{}); ok {
				if id, ok := s["id"].(string); ok {
					skillNames = append(skillNames, id)
				}
			}
		}
		fmt.Printf("%s\n", strings.Join(skillNames, ", "))
	}

	return nil
}

// updateRuntimeInfo updates the registry with runtime information
func (ar *NodeRunner) updateRuntimeInfo(nodeName string, port, pid int) error {
	registryPath := filepath.Join(ar.AgentsHome, "installed.yaml")

	// Load registry
	registry := &InstallationRegistry{}
	if data, err := os.ReadFile(registryPath); err == nil {
		if err := yaml.Unmarshal(data, registry); err != nil {
			return fmt.Errorf("failed to parse registry: %w", err)
		}
	}

	// Update runtime info
	if node, exists := registry.Installed[nodeName]; exists {
		startedAt := time.Now().Format(time.RFC3339)
		node.Status = "running"
		node.Runtime.Port = &port
		node.Runtime.PID = &pid
		node.Runtime.StartedAt = &startedAt
		registry.Installed[nodeName] = node
	}

	// Save registry
	data, err := yaml.Marshal(registry)
	if err != nil {
		return err
	}

	return os.WriteFile(registryPath, data, 0644)
}

// loadRegistry loads the installation registry
func (ar *NodeRunner) loadRegistry() (*InstallationRegistry, error) {
	registryPath := filepath.Join(ar.AgentsHome, "installed.yaml")

	registry := &InstallationRegistry{
		Installed: make(map[string]InstalledPackage),
	}

	if data, err := os.ReadFile(registryPath); err == nil {
		if err := yaml.Unmarshal(data, registry); err != nil {
			return nil, fmt.Errorf("failed to parse registry: %w", err)
		}
	}

	return registry, nil
}

// loadPackageEnvFile loads environment variables from package .env file
func (ar *NodeRunner) loadPackageEnvFile(packagePath string) (map[string]string, error) {
	envPath := filepath.Join(packagePath, ".env")

	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil, err
	}

	envVars := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove quotes if present
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}

			envVars[key] = value
		}
	}

	return envVars, nil
}
