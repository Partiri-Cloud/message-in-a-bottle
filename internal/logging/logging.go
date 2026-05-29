// Package logging provides a shared, structured (slog) logger for all services.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON slog.Logger writing to stdout. The level is read from the
// LOG_LEVEL environment variable (debug|info|warn|error), defaulting to info.
func New() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: levelFromEnv(),
	})
	return slog.New(handler)
}

// Init builds the logger via New and installs it as the slog default so that
// package-level slog calls (slog.Info, slog.Error, ...) are structured too.
func Init() *slog.Logger {
	logger := New()
	slog.SetDefault(logger)
	return logger
}

func levelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
