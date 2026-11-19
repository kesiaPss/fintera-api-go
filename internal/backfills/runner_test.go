package backfills

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeBackfillRepo struct {
	runs   []models.BackfillRun
	nextID uint
}

func newFakeBackfillRepo() *fakeBackfillRepo {
	return &fakeBackfillRepo{nextID: 1}
}

func (r *fakeBackfillRepo) FindCompleted(_ context.Context, name, version string) (*models.BackfillRun, error) {
	for i := range r.runs {
		if r.runs[i].Name == name && r.runs[i].Version == version && r.runs[i].Status == models.BackfillStatusCompleted {
			copy := r.runs[i]
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeBackfillRepo) Start(_ context.Context, name, version string) (*models.BackfillRun, error) {
	run := models.BackfillRun{
		ID:        r.nextID,
		Name:      name,
		Version:   version,
		Status:    models.BackfillStatusRunning,
		StartedAt: time.Now(),
	}
	r.nextID++
	r.runs = append(r.runs, run)
	return &run, nil
}

func (r *fakeBackfillRepo) Complete(_ context.Context, id uint, rowsAffected int64, details string) error {
	for i := range r.runs {
		if r.runs[i].ID == id {
			now := time.Now()
			r.runs[i].Status = models.BackfillStatusCompleted
			r.runs[i].FinishedAt = &now
			r.runs[i].RowsAffected = rowsAffected
			r.runs[i].Details = details
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeBackfillRepo) Fail(_ context.Context, id uint, message, details string) error {
	for i := range r.runs {
		if r.runs[i].ID == id {
			now := time.Now()
			r.runs[i].Status = models.BackfillStatusFailed
			r.runs[i].FinishedAt = &now
			r.runs[i].Error = message
			r.runs[i].Details = details
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeBackfillRepo) ListRuns(_ context.Context, limit int) ([]models.BackfillRun, error) {
	result := make([]models.BackfillRun, 0, len(r.runs))
	for i := len(r.runs) - 1; i >= 0; i-- {
		result = append(result, r.runs[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func testTask(counter *int, result Result, runErr error) Task {
	return Task{
		Name:    "demo",
		Version: "1",
		Run: func(context.Context) (Result, error) {
			*counter++
			return result, runErr
		},
	}
}

func TestRunnerRunsTaskOnce(t *testing.T) {
	repo := newFakeBackfillRepo()
	runner := NewRunner(repo)
	registry := NewRegistry()
	counter := 0
	require.NoError(t, registry.Register(testTask(&counter, Result{RowsAffected: 7, Details: "linked=7"}, nil)))

	first := runner.RunPending(context.Background(), registry)
	require.Len(t, first, 1)
	require.NoError(t, first[0].Err)
	assert.False(t, first[0].Skipped)
	assert.Equal(t, 1, counter)
	assert.Equal(t, int64(7), first[0].Run.RowsAffected)
	assert.Equal(t, models.BackfillStatusCompleted, repo.runs[0].Status)

	second := runner.RunPending(context.Background(), registry)
	require.Len(t, second, 1)
	assert.True(t, second[0].Skipped)
	assert.Equal(t, 1, counter)
}

func TestRunnerForceReruns(t *testing.T) {
	repo := newFakeBackfillRepo()
	runner := NewRunner(repo)
	counter := 0
	task := testTask(&counter, Result{RowsAffected: 1}, nil)

	require.NoError(t, runner.Run(context.Background(), task, false).Err)
	require.NoError(t, runner.Run(context.Background(), task, true).Err)
	assert.Equal(t, 2, counter)
	assert.Len(t, repo.runs, 2)
}

func TestRunnerRecordsFailureAndRetries(t *testing.T) {
	repo := newFakeBackfillRepo()
	runner := NewRunner(repo)
	counter := 0
	failing := Task{
		Name:    "demo",
		Version: "1",
		Run: func(context.Context) (Result, error) {
			counter++
			return Result{}, errors.New("boom")
		},
	}

	result := runner.Run(context.Background(), failing, false)
	assert.Error(t, result.Err)
	assert.Equal(t, models.BackfillStatusFailed, repo.runs[0].Status)
	assert.Contains(t, repo.runs[0].Error, "boom")

	registry := NewRegistry()
	require.NoError(t, registry.Register(testTask(&counter, Result{RowsAffected: 2}, nil)))
	retry := runner.RunPending(context.Background(), registry)
	require.Len(t, retry, 1)
	require.NoError(t, retry[0].Err)
	assert.Equal(t, 2, counter)
	assert.Equal(t, models.BackfillStatusCompleted, repo.runs[1].Status)
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	registry := NewRegistry()
	counter := 0
	require.NoError(t, registry.Register(testTask(&counter, Result{}, nil)))
	err := registry.Register(testTask(&counter, Result{}, nil))
	assert.ErrorIs(t, err, ErrDuplicateTask)
}

func TestRunnerStatuses(t *testing.T) {
	repo := newFakeBackfillRepo()
	runner := NewRunner(repo)
	registry := NewRegistry()
	counter := 0
	require.NoError(t, registry.Register(testTask(&counter, Result{}, nil)))

	statuses, err := runner.Statuses(context.Background(), registry)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, "pending", statuses[0].Status)

	require.NoError(t, runner.Run(context.Background(), registry.Tasks()[0], false).Err)
	statuses, err = runner.Statuses(context.Background(), registry)
	require.NoError(t, err)
	assert.Equal(t, models.BackfillStatusCompleted, statuses[0].Status)
}
