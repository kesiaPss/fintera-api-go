package backfills

import (
	"context"
	"errors"
	"fmt"
)

// ErrDuplicateTask is returned when two tasks share a name.
var ErrDuplicateTask = errors.New("backfill task already registered")

// Result is the outcome of a backfill task.
type Result struct {
	RowsAffected int64
	Details      string
}

// Task is one registered one-off data migration.
type Task struct {
	Name        string
	Version     string
	Description string
	Run         func(ctx context.Context) (Result, error)
}

// Registry keeps the tasks available to the API and the CLI.
type Registry struct {
	tasks  []Task
	byName map[string]Task
}

// NewRegistry creates an empty backfill registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]Task)}
}

// Register adds a task. Names must be unique; an empty version defaults to 1.
func (r *Registry) Register(task Task) error {
	if task.Name == "" {
		return fmt.Errorf("backfill task name is required")
	}
	if task.Run == nil {
		return fmt.Errorf("backfill task %s has no run function", task.Name)
	}
	if task.Version == "" {
		task.Version = "1"
	}
	if _, exists := r.byName[task.Name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateTask, task.Name)
	}
	r.tasks = append(r.tasks, task)
	r.byName[task.Name] = task
	return nil
}

// Tasks returns the registered tasks in registration order.
func (r *Registry) Tasks() []Task {
	if r == nil {
		return nil
	}
	return r.tasks
}

// Find returns a task by name.
func (r *Registry) Find(name string) (Task, bool) {
	if r == nil {
		return Task{}, false
	}
	task, ok := r.byName[name]
	return task, ok
}
