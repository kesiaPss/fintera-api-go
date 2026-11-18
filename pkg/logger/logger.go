package logger

import (
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

// current holds the configured logger. Access is atomic so Setup and logging
// can run concurrently (for example, tests that reconfigure logging while
// background jobs are still running).
var current atomic.Pointer[slog.Logger]

// Setup initializes the global logger based on the environment. Deployed
// environments always emit structured JSON so the collector receives a true
// severity field; local text logs are opt-in with LOG_FORMAT=text.
func Setup(env string) {
	l := slog.New(newHandler(env))
	current.Store(l)
	slog.SetDefault(l)
}

func newHandler(_ string) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: logLevel(),
	}

	if os.Getenv("LOG_FORMAT") == "text" {
		return slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.NewJSONHandler(os.Stdout, opts)
}

// logLevel reads LOG_LEVEL (debug, info, warn, error). Unset or invalid values
// keep the info default; routine SQL and per-run job details log at debug.
func logLevel() slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.TrimSpace(os.Getenv("LOG_LEVEL")))); err != nil {
		return slog.LevelInfo
	}
	return level
}

func get() *slog.Logger {
	if l := current.Load(); l != nil {
		return l
	}
	return slog.Default()
}

// Info logs an info message
func Info(msg string, args ...any) {
	get().Info(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	get().Error(msg, args...)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	get().Debug(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	get().Warn(msg, args...)
}
