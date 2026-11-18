package logger

import (
	"log/slog"
	"testing"
)

func TestNewHandlerEmitsJSONByDefault(t *testing.T) {
	t.Setenv("LOG_FORMAT", "")

	handler := newHandler("production")

	if _, ok := handler.(*slog.JSONHandler); !ok {
		t.Fatalf("expected a JSON handler so severity is machine-readable, got %T", handler)
	}
}

func TestNewHandlerOptInText(t *testing.T) {
	t.Setenv("LOG_FORMAT", "text")

	handler := newHandler("development")

	if _, ok := handler.(*slog.TextHandler); !ok {
		t.Fatalf("expected a text handler when LOG_FORMAT=text, got %T", handler)
	}
}

func TestLogLevelFromEnvironment(t *testing.T) {
	tests := map[string]slog.Level{
		"":        slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"WARN":    slog.LevelWarn,
		" error ": slog.LevelError,
		"verbose": slog.LevelInfo,
	}

	for value, want := range tests {
		t.Setenv("LOG_LEVEL", value)
		if got := logLevel(); got != want {
			t.Errorf("LOG_LEVEL=%q: level = %v, want %v", value, got, want)
		}
	}
}
