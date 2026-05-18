package obs

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger returns a *slog.Logger writing JSON to stderr.
// Level is read from LOG_LEVEL (default: info).
func NewLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
