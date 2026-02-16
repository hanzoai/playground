package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hanzoai/playground/control-plane/internal/packages"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed bot packages",
		Long: `Display all installed bot packages with their status.

Shows package name, version, status (running/stopped), and port if running.

Examples:
  playground list`,
		Run: runListCommand,
	}

	return cmd
}

func runListCommand(cmd *cobra.Command, args []string) {
	agentsHome := getAgentsHomeDir()
	registryPath := filepath.Join(agentsHome, "installed.yaml")

	// Load registry
	registry := &packages.InstallationRegistry{
		Installed: make(map[string]packages.InstalledPackage),
	}

	if data, err := os.ReadFile(registryPath); err == nil {
		if err := yaml.Unmarshal(data, registry); err != nil {
			cmd.PrintErrf("failed to parse registry: %v\n", err)
			return
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		cmd.PrintErrf("failed to read registry: %v\n", err)
		return
	}

	if len(registry.Installed) == 0 {
		fmt.Println("📦 No bot packages installed")
		fmt.Println("💡 Install packages with: playground install <package-path>")
		return
	}

	fmt.Printf("📦 Installed Bot Packages (%d total):\n\n", len(registry.Installed))

	for name, pkg := range registry.Installed {
		status := pkg.Status
		statusIcon := "⏹️"
		if status == "running" {
			statusIcon = "🟢"
		} else if status == "error" {
			statusIcon = "🔴"
		}

		fmt.Printf("%s %s (v%s)\n", statusIcon, name, pkg.Version)
		fmt.Printf("   %s\n", pkg.Description)

		if status == "running" && pkg.Runtime.Port != nil {
			fmt.Printf("   🌐 Running on port %d (PID: %d)\n", *pkg.Runtime.Port, *pkg.Runtime.PID)
		}

		fmt.Printf("   📁 %s\n", pkg.Path)
		fmt.Println()
	}

	fmt.Println("💡 Commands:")
	fmt.Println("   playground run <name>     - Start a bot")
	fmt.Println("   playground stop <name>    - Stop a running bot")
	fmt.Println("   playground logs <name>    - View bot logs")
}
