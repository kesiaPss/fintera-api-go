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
