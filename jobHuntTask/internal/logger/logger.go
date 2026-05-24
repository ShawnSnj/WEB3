// Package logger provides a thin wrapper around log/slog so that the rest
// of the codebase depends on a single, stable construction function rather
// than on slog directly.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/shawn/jobhunttask/internal/config"
)

// New builds a *slog.Logger from the given config. The returned logger is
// safe for concurrent use and is intentionally NOT installed as the slog
// default — callers should pass it explicitly down the dependency tree.
func New(cfg config.Log) *slog.Logger {
	return newWithWriter(cfg, os.Stdout)
}

func newWithWriter(cfg config.Log, w io.Writer) *slog.Logger {
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "text":
		handler = slog.NewTextHandler(w, opts)
	default:
		handler = slog.NewJSONHandler(w, opts)
	}
	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
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
