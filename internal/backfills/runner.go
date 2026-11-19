package backfills

import (
	"context"
	"errors"
	"fmt"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"gorm.io/gorm"
)

// Runner executes backfill tasks and records each run.
type Runner struct {
	repo repository.BackfillRepository
}

// NewRunner creates a backfill runner.
func NewRunner(repo repository.BackfillRepository) *Runner {
	return &Runner{repo: repo}
}

// TaskRun is the outcome of one task execution.
type TaskRun struct {
	Task    Task
	Run     *models.BackfillRun
	Skipped bool
	Err     error
}

// TaskStatus is the list view of a task and its latest run.
type TaskStatus struct {
	Task    Task
	Status  string
	LastRun *models.BackfillRun
}

// Run executes one task. A completed task is skipped unless force is true.
func (r *Runner) Run(ctx context.Context, task Task, force bool) TaskRun {
	result := TaskRun{Task: task}
	if r == nil || r.repo == nil {
		result.Err = errors.New("backfill runner has no repository")
		return result
	}
	if !force {
		completed, err := r.repo.FindCompleted(ctx, task.Name, task.Version)
		if err == nil {
			result.Run = completed
			result.Skipped = true
			return result
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err = err
			return result
		}
	}

	run, err := r.repo.Start(ctx, task.Name, task.Version)
	if err != nil {
		result.Err = err
		return result
	}
	result.Run = run

	outcome, runErr := task.Run(ctx)
	if runErr != nil {
		_ = r.repo.Fail(ctx, run.ID, runErr.Error(), outcome.Details)
		result.Err = runErr
		return result
	}
	if err := r.repo.Complete(ctx, run.ID, outcome.RowsAffected, outcome.Details); err != nil {
		result.Err = err
		return result
	}
	run.Status = models.BackfillStatusCompleted
	run.RowsAffected = outcome.RowsAffected
	run.Details = outcome.Details
	return result
}

// RunPending executes every task that has not completed for its version.
func (r *Runner) RunPending(ctx context.Context, registry *Registry) []TaskRun {
	tasks := registry.Tasks()
	results := make([]TaskRun, 0, len(tasks))
	for _, task := range tasks {
		results = append(results, r.Run(ctx, task, false))
	}
	return results
}

// Statuses returns the state of every registered task.
func (r *Runner) Statuses(ctx context.Context, registry *Registry) ([]TaskStatus, error) {
	if r == nil || r.repo == nil {
		return nil, errors.New("backfill runner has no repository")
	}
	runs, err := r.repo.ListRuns(ctx, 500)
	if err != nil {
		return nil, err
	}
	latest := make(map[string]*models.BackfillRun, len(runs))
	completed := make(map[string]bool, len(runs))
	for i := range runs {
		run := &runs[i]
		key := run.Name + "|" + run.Version
		if _, ok := latest[key]; !ok {
			latest[key] = run
		}
		if run.Status == models.BackfillStatusCompleted {
			completed[key] = true
		}
	}

	tasks := registry.Tasks()
	statuses := make([]TaskStatus, 0, len(tasks))
	for _, task := range tasks {
		key := task.Name + "|" + task.Version
		status := "pending"
		if completed[key] {
			status = models.BackfillStatusCompleted
		} else if run := latest[key]; run != nil && run.Status == models.BackfillStatusFailed {
			status = models.BackfillStatusFailed
		}
		statuses = append(statuses, TaskStatus{
			Task:    task,
			Status:  status,
			LastRun: latest[key],
		})
	}
	return statuses, nil
}

// String makes TaskRun easy to log.
func (r TaskRun) String() string {
	if r.Err != nil {
		return fmt.Sprintf("%s@%s failed: %v", r.Task.Name, r.Task.Version, r.Err)
	}
	if r.Skipped {
		return fmt.Sprintf("%s@%s already completed", r.Task.Name, r.Task.Version)
	}
	rows := int64(0)
	if r.Run != nil {
		rows = r.Run.RowsAffected
	}
	return fmt.Sprintf("%s@%s completed (%d rows)", r.Task.Name, r.Task.Version, rows)
}
