package repository

import (
	"context"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// BackfillRepository stores backfill execution history so one-off data tasks
// can be audited and only run once per version.
type BackfillRepository interface {
	FindCompleted(ctx context.Context, name, version string) (*models.BackfillRun, error)
	Start(ctx context.Context, name, version string) (*models.BackfillRun, error)
	Complete(ctx context.Context, id uint, rowsAffected int64, details string) error
	Fail(ctx context.Context, id uint, message, details string) error
	ListRuns(ctx context.Context, limit int) ([]models.BackfillRun, error)
}

type backfillRepository struct {
	db *gorm.DB
}

// NewBackfillRepository creates the backfill-run repository.
func NewBackfillRepository(db *gorm.DB) BackfillRepository {
	return &backfillRepository{db: db}
}

func (r *backfillRepository) FindCompleted(ctx context.Context, name, version string) (*models.BackfillRun, error) {
	var run models.BackfillRun
	err := r.db.WithContext(ctx).
		Where("name = ? AND version = ? AND status = ?", name, version, models.BackfillStatusCompleted).
		Order("finished_at DESC").
		First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *backfillRepository) Start(ctx context.Context, name, version string) (*models.BackfillRun, error) {
	run := &models.BackfillRun{
		Name:      name,
		Version:   version,
		Status:    models.BackfillStatusRunning,
		StartedAt: time.Now(),
	}
	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		return nil, err
	}
	return run, nil
}

func (r *backfillRepository) Complete(ctx context.Context, id uint, rowsAffected int64, details string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.BackfillRun{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.BackfillStatusCompleted,
			"finished_at":   now,
			"rows_affected": rowsAffected,
			"details":       details,
			"error":         "",
		}).Error
}

func (r *backfillRepository) Fail(ctx context.Context, id uint, message, details string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.BackfillRun{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      models.BackfillStatusFailed,
			"finished_at": now,
			"details":     details,
			"error":       message,
		}).Error
}

func (r *backfillRepository) ListRuns(ctx context.Context, limit int) ([]models.BackfillRun, error) {
	if limit <= 0 {
		limit = 50
	}
	var runs []models.BackfillRun
	err := r.db.WithContext(ctx).
		Order("started_at DESC").
		Limit(limit).
		Find(&runs).Error
	return runs, err
}
