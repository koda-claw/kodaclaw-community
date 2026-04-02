package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init initializes the global slog logger with the given level.
// Supported levels: "debug", "info", "warn", "error" (case-insensitive, default "info").
func Init(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn", "warning":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})
	slog.SetDefault(slog.New(h))
}
