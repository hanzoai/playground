package cli

import (
	"fmt"

	"github.com/hanzoai/playground/control-plane/internal/packages"
	"github.com/spf13/cobra"
)

var (
	uninstallForce bool
)

// NewUninstallCommand creates the uninstall command
func NewUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <package-name>",
		Short: "Uninstall a bot package",
		Long: `Uninstall removes an installed bot package from your system.

This command will:
- Stop the bot if it's currently running
- Remove the package directory and all its files
- Remove the package from the installation registry
- Clean up any associated logs

Examples:
  playground uninstall my-bot
  playground uninstall sentiment-analyzer --force`,
		Args: cobra.ExactArgs(1),
		RunE: runUninstallCommand,
	}

	cmd.Flags().BoolVarP(&uninstallForce, "force", "f", false, "Force uninstall even if bot is running")

	return cmd
}

func runUninstallCommand(cmd *cobra.Command, args []string) error {
	packageName := args[0]

	// Create uninstaller
	uninstaller := &packages.PackageUninstaller{
		AgentsHome: getAgentsHomeDir(),
		Force:          uninstallForce,
	}

	// Uninstall package
	if err := uninstaller.UninstallPackage(packageName); err != nil {
		return fmt.Errorf("uninstallation failed: %w", err)
	}

	return nil
}
