package database

import (
	"testing"

	"gorm.io/gorm/logger"
)

func TestGormLogLevelUsesEnvironment(t *testing.T) {
	if got := gormLogLevel("production"); got != logger.Silent {
		t.Fatalf("expected silent database logging in production, got %v", got)
	}

	if got := gormLogLevel("development"); got != logger.Info {
		t.Fatalf("expected informational database logging outside production, got %v", got)
	}
}
