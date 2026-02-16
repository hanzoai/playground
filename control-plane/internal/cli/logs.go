package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec" // Added missing import
	"path/filepath"

	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/packages"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	logsFollow bool
	logsTail   int
)

// NewLogsCommand creates the logs command
func NewLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <bot-name>",
		Short: "View logs for a bot",
		Long: `Display logs for an installed bot package.

Shows the most recent log entries from the bot's log file.

Examples:
  playground logs email-helper
  playground logs data-analyzer --follow`,
		Args: cobra.ExactArgs(1),
		RunE: runLogsCommand,
	}

	cmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVarP(&logsTail, "tail", "n", 50, "Number of lines to show from the end")

	return cmd
}

func runLogsCommand(cmd *cobra.Command, args []string) error {
	nodeName := args[0]

	logViewer := &LogViewer{
		AgentsHome: getAgentsHomeDir(),
		Follow:         logsFollow,
		Tail:           logsTail,
	}

	if err := logViewer.ViewLogs(nodeName); err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to view logs")
		return fmt.Errorf("failed to view logs: %w", err)
	}

	return nil
}

// LogViewer handles viewing hanzo node logs
type LogViewer struct {
	AgentsHome string
	Follow         bool
	Tail           int
}

// ViewLogs displays logs for an hanzo node
func (lv *LogViewer) ViewLogs(nodeName string) error {
	// Load registry to get log file path
	registryPath := filepath.Join(lv.AgentsHome, "installed.yaml")
	registry := &packages.InstallationRegistry{
		Installed: make(map[string]packages.InstalledPackage),
	}

	if data, err := os.ReadFile(registryPath); err == nil {
		if err := yaml.Unmarshal(data, registry); err != nil {
			return fmt.Errorf("failed to parse registry: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to read registry: %w", err)
	}

	node, exists := registry.Installed[nodeName]
	if !exists {
		return fmt.Errorf("hanzo node %s not installed", nodeName)
	}

	logFile := node.Runtime.LogFile
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		logger.Logger.Info().Msgf("📝 No logs found for %s", nodeName)
		logger.Logger.Info().Msg("💡 Logs will appear here when the hanzo node is running")
		return nil
	}

	logger.Logger.Info().Msgf("📝 Logs for %s:", nodeName)
	logger.Logger.Info().Msgf("📁 %s\n", logFile)

	if lv.Follow {
		return lv.followLogs(logFile)
	} else {
		return lv.tailLogs(logFile, lv.Tail)
	}
}

// tailLogs shows the last N lines of the log file
func (lv *LogViewer) tailLogs(logFile string, lines int) error {
	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", lines), logFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// followLogs follows the log file in real-time
func (lv *LogViewer) followLogs(logFile string) error {
	cmd := exec.Command("tail", "-f", logFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
