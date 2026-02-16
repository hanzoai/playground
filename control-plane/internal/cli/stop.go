package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/packages"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewStopCommand creates the stop command
func NewStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <bot-name>",
		Short: "Stop a running bot",
		Long: `Stop a running bot package.

The bot process will be terminated gracefully and its status
will be updated in the registry.

Examples:
  playground stop email-helper
  playground stop data-analyzer`,
		Args: cobra.ExactArgs(1),
		RunE: runStopCommand,
	}

	return cmd
}

func runStopCommand(cmd *cobra.Command, args []string) error {
	nodeName := args[0]

	stopper := &NodeStopper{
		AgentsHome: getAgentsHomeDir(),
	}

	if err := stopper.StopNode(nodeName); err != nil {
		return fmt.Errorf("failed to stop hanzo node: %w", err)
	}

	return nil
}

// NodeStopper handles stopping hanzo nodes
type NodeStopper struct {
	AgentsHome string
}

// StopNode stops a running hanzo node
func (as *NodeStopper) StopNode(nodeName string) error {
	// Load registry
	registry, err := as.loadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	node, exists := registry.Installed[nodeName]
	if !exists {
		return fmt.Errorf("hanzo node %s not installed", nodeName)
	}

	if node.Status != "running" {
		fmt.Printf("⚠️  node %s is not running\n", nodeName)
		return nil
	}

	if node.Runtime.PID == nil {
		return fmt.Errorf("no PID found for hanzo node %s", nodeName)
	}

	fmt.Printf("🛑 Stopping hanzo node: %s (PID: %d)\n", nodeName, *node.Runtime.PID)

	// Find and kill the process
	process, err := os.FindProcess(*node.Runtime.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Try HTTP shutdown first if port is available
	httpShutdownSuccess := false
	if node.Runtime.Port != nil {
		fmt.Printf("🛑 Attempting graceful HTTP shutdown for bot %s on port %d\n", nodeName, *node.Runtime.Port)

		// Construct bot base URL
		baseURL := fmt.Sprintf("http://localhost:%d", *node.Runtime.Port)
		shutdownURL := fmt.Sprintf("%s/shutdown", baseURL)

		// Create shutdown request
		requestBody := map[string]interface{}{
			"graceful":        true,
			"timeout_seconds": 30,
		}

		bodyBytes, err := json.Marshal(requestBody)
		if err == nil {
			req, err := http.NewRequest("POST", shutdownURL, bytes.NewReader(bodyBytes))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "Playground-CLI/1.0")

				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					if resp.StatusCode == 200 {
						fmt.Printf("✅ HTTP shutdown request accepted for bot %s\n", nodeName)
						httpShutdownSuccess = true

						// Wait a moment for graceful shutdown
						time.Sleep(3 * time.Second)
					} else {
						fmt.Printf("⚠️ HTTP shutdown returned status %d for bot %s\n", resp.StatusCode, nodeName)
					}
				} else {
					fmt.Printf("⚠️ HTTP shutdown request failed for bot %s: %v\n", nodeName, err)
				}
			}
		}
	}

	// If HTTP shutdown failed or not available, fall back to process signals
	if !httpShutdownSuccess {
		fmt.Printf("🔄 Falling back to process signal shutdown for bot %s\n", nodeName)

		// Send SIGTERM for graceful shutdown
		if err := process.Signal(os.Interrupt); err != nil {
			// If graceful shutdown fails, force kill
			if err := process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
		} else {
			// Wait for graceful shutdown, then check if still running
			time.Sleep(3 * time.Second)

			// Check if process is still running
			if err := process.Signal(syscall.Signal(0)); err == nil {
				// Process still running, force kill
				fmt.Printf("⚠️ Process still running, force killing bot %s\n", nodeName)
				if err := process.Kill(); err != nil {
					return fmt.Errorf("failed to force kill process: %w", err)
				}
			}
		}
	}

	// Update registry
	node.Status = "stopped"
	node.Runtime.Port = nil
	node.Runtime.PID = nil
	node.Runtime.StartedAt = nil
	registry.Installed[nodeName] = node

	if err := as.saveRegistry(registry); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	fmt.Printf("✅ node %s stopped successfully\n", nodeName)

	return nil
}

// loadRegistry loads the installation registry
func (as *NodeStopper) loadRegistry() (*packages.InstallationRegistry, error) {
	registryPath := filepath.Join(as.AgentsHome, "installed.yaml")

	registry := &packages.InstallationRegistry{
		Installed: make(map[string]packages.InstalledPackage),
	}

	if data, err := os.ReadFile(registryPath); err == nil {
		if err := yaml.Unmarshal(data, registry); err != nil {
			return nil, fmt.Errorf("failed to parse registry: %w", err)
		}
	}

	return registry, nil
}

// saveRegistry saves the installation registry
func (as *NodeStopper) saveRegistry(registry *packages.InstallationRegistry) error {
	registryPath := filepath.Join(as.AgentsHome, "installed.yaml")

	data, err := yaml.Marshal(registry)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	return os.WriteFile(registryPath, data, 0644)
}
