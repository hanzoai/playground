// Package logger provides a global zerolog logger for the Agents CLI.
package logger

import (
	"log/slog"
	"os"

	"github.com/rs/zerolog"
)

var (
	// Logger is the global zerolog logger instance.
	Logger zerolog.Logger
)

// InitLogger initializes the global logger with the specified log level.
func InitLogger(verbose bool) {
	level := zerolog.InfoLevel
	if verbose {
		level = zerolog.DebugLevel
	}
	Logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(level)
}

// SlogAdapter returns an *slog.Logger that writes through zerolog.
// Used by ZAP nodes and other components that expect the slog interface.
func SlogAdapter() *slog.Logger {
	level := slog.LevelInfo
	if Logger.GetLevel() <= zerolog.DebugLevel {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
