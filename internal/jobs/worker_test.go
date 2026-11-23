package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteRecoversStringPanic(t *testing.T) {
	w := NewWorker(1)
	defer func() { _ = w.Shutdown(context.Background()) }()

	err := w.execute(func(context.Context) error {
		panic("boom")
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "panic: boom")
}

func TestExecutePreservesErrorPanic(t *testing.T) {
	w := NewWorker(1)
	defer func() { _ = w.Shutdown(context.Background()) }()

	sentinel := errors.New("kaboom")
	err := w.execute(func(context.Context) error {
		panic(sentinel)
	})

	require.ErrorIs(t, err, sentinel)
}

func TestExecuteReturnsJobError(t *testing.T) {
	w := NewWorker(1)
	defer func() { _ = w.Shutdown(context.Background()) }()

	sentinel := errors.New("job failed")
	err := w.execute(func(context.Context) error {
		return sentinel
	})

	require.ErrorIs(t, err, sentinel)
}

func TestEnqueueAsyncSurvivesPanicAndTracksFailure(t *testing.T) {
	w := NewWorker(1)
	defer func() { _ = w.Shutdown(context.Background()) }()

	started := make(chan struct{})
	w.EnqueueAsync(func(context.Context) error {
		close(started)
		panic("async boom")
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("async job did not start")
	}

	require.Eventually(t, func() bool {
		return w.GetStats().FailedJobs == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestScheduledJobSurvivesPanic(t *testing.T) {
	w := NewWorker(1)
	defer func() { _ = w.Shutdown(context.Background()) }()

	started := make(chan struct{})
	w.ScheduleEveryImmediate("test", time.Hour, func(context.Context) error {
		close(started)
		panic("scheduled boom")
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled job did not run")
	}

	require.Eventually(t, func() bool {
		return w.GetStats().FailedJobs == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestShutdownWaitsForRunningJobsWithinTimeout(t *testing.T) {
	w := NewWorker(1)

	finished := make(chan struct{})
	w.EnqueueAsync(func(context.Context) error {
		time.Sleep(50 * time.Millisecond)
		close(finished)
		return nil
	})
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, w.Shutdown(ctx))

	select {
	case <-finished:
	default:
		t.Fatal("running job did not finish before Shutdown returned")
	}
}

func TestShutdownCancelsJobsAfterTimeout(t *testing.T) {
	w := NewWorker(1)

	cancelled := make(chan struct{})
	started := make(chan struct{})
	w.EnqueueAsync(func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	})
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := w.Shutdown(ctx)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("job context was not cancelled after the shutdown timeout")
	}
}

func TestShutdownStopsScheduledRuns(t *testing.T) {
	w := NewWorker(1)

	var runs atomic.Int32
	w.ScheduleEvery("test", 5*time.Millisecond, func(context.Context) error {
		runs.Add(1)
		return nil
	})
	require.Eventually(t, func() bool { return runs.Load() >= 2 }, 2*time.Second, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, w.Shutdown(ctx))

	stopped := runs.Load()
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, stopped, runs.Load())
}

func TestScheduleEveryImmediateJitterRunsAfterDelayAndRepeats(t *testing.T) {
	w := NewWorker(1)
	defer func() { _ = w.Shutdown(context.Background()) }()

	var runs atomic.Int32
	w.ScheduleEveryImmediateJitter("test", 20*time.Millisecond, 100*time.Millisecond, func(context.Context) error {
		runs.Add(1)
		return nil
	})

	require.Eventually(t, func() bool { return runs.Load() >= 2 }, 2*time.Second, 10*time.Millisecond)
}

func TestScheduleEveryImmediateJitterAbandonsPendingRunOnShutdown(t *testing.T) {
	w := NewWorker(1)

	var runs atomic.Int32
	// The random first-run delay lives in [0, 1h), so it is virtually certain
	// to still be pending when shutdown begins.
	w.ScheduleEveryImmediateJitter("test", time.Hour, time.Hour, func(context.Context) error {
		runs.Add(1)
		return nil
	})

	time.Sleep(200 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, w.Shutdown(ctx))

	assert.Zero(t, runs.Load(), "jittered first run must not execute after shutdown")
}
