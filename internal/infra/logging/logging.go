// Package logging configures process-wide structured logging via log/slog.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON slog.Logger writing to stdout at the given level.
// Unknown or empty levels default to Info.
func New(level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: ParseLevel(level)}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// ParseLevel maps config strings to slog levels (case-insensitive).
// Accepts the common set: debug, info, warn/warning, error.
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "trace":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "fatal", "panic":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// OrDefault returns lg when non-nil, otherwise slog.Default().
func OrDefault(lg *slog.Logger) *slog.Logger {
	if lg == nil {
		return slog.Default()
	}
	return lg
}
