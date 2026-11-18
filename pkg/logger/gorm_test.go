package logger

import (
	"fmt"
	"testing"

	"gorm.io/gorm"
)

func TestIsRecordNotFound(t *testing.T) {
	if !isRecordNotFound(gorm.ErrRecordNotFound) {
		t.Fatal("expected gorm.ErrRecordNotFound to be classified as not found")
	}

	wrapped := fmt.Errorf("lookup analytics cache: %w", gorm.ErrRecordNotFound)
	if !isRecordNotFound(wrapped) {
		t.Fatal("expected wrapped gorm.ErrRecordNotFound to be classified as not found")
	}

	if isRecordNotFound(fmt.Errorf("connection refused")) {
		t.Fatal("expected a real database error not to be classified as not found")
	}
}
