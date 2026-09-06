// Package logging provides a shared structured logger for the tender-scraper.
//
// It emits logs via Go's standard library log/slog (no external deps). Output
// format is controlled by env vars so the same binary works for local dev
// (human-readable text) and production log ingestion (JSON on stdout):
//
//	LOG_FORMAT = "json" | "text"   (default: "text")
//	LOG_LEVEL  = "debug" | "info" | "warn" | "error"  (default: "info")
//
// JSON output is line-delimited and ships cleanly into Loki / ELK / CloudWatch
// / Datadog without a parsing rule. Each line carries a timestamp, level,
// message, and any structured key/value fields attached at the call site.
package logging

import (
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	initOnce sync.Once
	base     *slog.Logger
)

// Init configures the global slog default logger from environment variables.
// Safe to call multiple times; only the first call takes effect.
func Init() *slog.Logger {
	initOnce.Do(func() {
		level := parseLevel(os.Getenv("LOG_LEVEL"))
		opts := &slog.HandlerOptions{Level: level}

		var handler slog.Handler
		if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
			handler = slog.NewJSONHandler(os.Stdout, opts)
		} else {
			handler = slog.NewTextHandler(os.Stdout, opts)
		}

		base = slog.New(handler)
		slog.SetDefault(base)
	})
	return base
}

// L returns the shared logger, initializing it with defaults if needed.
func L() *slog.Logger {
	if base == nil {
		return Init()
	}
	return base
}

// With returns a child logger with the given key/value context attached.
// Useful for tagging all logs from a job: logging.With("job_id", id, "state", st).
func With(args ...any) *slog.Logger {
	return L().With(args...)
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
