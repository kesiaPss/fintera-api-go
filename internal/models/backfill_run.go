package models

import "time"

// Backfill run statuses. A task is considered executed when a completed row
// exists for its name and version.
const (
	BackfillStatusRunning   = "running"
	BackfillStatusCompleted = "completed"
	BackfillStatusFailed    = "failed"
)

// BackfillRun records one execution of a registered backfill task.
type BackfillRun struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"size:120;not null" json:"name"`
	Version      string     `gorm:"size:80;not null;default:''" json:"version"`
	Status       string     `gorm:"size:20;not null;default:running" json:"status"`
	StartedAt    time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	FinishedAt   *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	RowsAffected int64      `gorm:"column:rows_affected;not null;default:0" json:"rows_affected"`
	Details      string     `gorm:"type:text" json:"details,omitempty"`
	Error        string     `gorm:"type:text" json:"error,omitempty"`
}

// TableName specifies the table name for BackfillRun.
func (BackfillRun) TableName() string {
	return "backfill_runs"
}
