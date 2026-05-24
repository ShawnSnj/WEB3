//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// seedTask inserts a fresh task via the task repository so the session
// foreign key has a target. Returns the task ID.
func seedTask(t *testing.T, tr *repository.PostgresTaskRepository) (id [16]byte) {
	t.Helper()
	task := &model.Task{
		Title: "session-host", Priority: model.PriorityMedium,
		Category: model.CategoryMisc, Status: model.StatusPending,
	}
	if err := tr.Create(context.Background(), task); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task.ID
}

func TestPostgres_SessionLifecycleAndUniqueRunning(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE task_execution_sessions, tasks RESTART IDENTITY CASCADE`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE task_execution_sessions, tasks RESTART IDENTITY CASCADE`)
	})

	taskRepo := repository.NewPostgresTaskRepository(pool)
	repo := repository.NewPostgresTaskSessionRepository(pool)
	ctx := context.Background()

	taskID := seedTask(t, taskRepo)

	// Start one
	s := &model.TaskSession{
		TaskID:    taskID,
		Status:    model.SessionStatusActive,
		StartedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Trying to create a second running session for the same task must
	// hit the partial unique index.
	s2 := &model.TaskSession{
		TaskID:    taskID,
		Status:    model.SessionStatusActive,
		StartedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, s2); !errors.Is(err, model.ErrSessionAlreadyRunning) {
		t.Errorf("want ErrSessionAlreadyRunning, got %v", err)
	}

	// Stop the first, then a new one should be allowed.
	stoppedStatus := model.SessionStatusStopped
	endedAt := time.Now().UTC()
	if _, err := repo.Update(ctx, s.ID, repository.SessionUpdate{
		Status:  &stoppedStatus,
		EndedAt: &endedAt,
	}); err != nil {
		t.Fatalf("Update -> stopped: %v", err)
	}
	if err := repo.Create(ctx, s2); err != nil {
		t.Errorf("second Create after stop: %v", err)
	}
}

func TestPostgres_SumEffectiveMinutes(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE task_execution_sessions, tasks RESTART IDENTITY CASCADE`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE task_execution_sessions, tasks RESTART IDENTITY CASCADE`)
	})

	taskRepo := repository.NewPostgresTaskRepository(pool)
	repo := repository.NewPostgresTaskSessionRepository(pool)
	ctx := context.Background()
	taskID := seedTask(t, taskRepo)

	start := time.Now().UTC().Add(-2 * time.Hour)
	mid := start.Add(30 * time.Minute)
	end := mid.Add(20 * time.Minute) // session A: 50 min wall, 0 pause -> 50 min

	a := &model.TaskSession{
		TaskID: taskID, Status: model.SessionStatusStopped,
		StartedAt: start, EndedAt: &end,
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("a: %v", err)
	}

	// Manually update its status after create (Create defaults to active).
	stopped := model.SessionStatusStopped
	if _, err := repo.Update(ctx, a.ID, repository.SessionUpdate{
		Status: &stopped, EndedAt: &end,
	}); err != nil {
		t.Fatalf("a update: %v", err)
	}

	// Session B: still active, 15 min of wall, 5 min paused already.
	bStart := time.Now().UTC().Add(-15 * time.Minute)
	b := &model.TaskSession{
		TaskID: taskID, Status: model.SessionStatusActive,
		StartedAt: bStart, TotalPausedSeconds: 300,
	}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatalf("b: %v", err)
	}

	now := time.Now().UTC()
	minutes, err := repo.SumEffectiveMinutesByTask(ctx, taskID, now)
	if err != nil {
		t.Fatalf("SumEffectiveMinutesByTask: %v", err)
	}
	// Expected: 50 (a) + (15 - 5) (b) = 60. Floor div may shave 0-1 minute
	// depending on clock skew between Go and Postgres on bStart.
	if minutes < 58 || minutes > 61 {
		t.Errorf("sum minutes = %d, want ~60", minutes)
	}
}

func TestPostgres_FindRunningByTask(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE task_execution_sessions, tasks RESTART IDENTITY CASCADE`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE task_execution_sessions, tasks RESTART IDENTITY CASCADE`)
	})

	taskRepo := repository.NewPostgresTaskRepository(pool)
	repo := repository.NewPostgresTaskSessionRepository(pool)
	ctx := context.Background()
	taskID := seedTask(t, taskRepo)

	if _, err := repo.FindRunningByTask(ctx, taskID); !errors.Is(err, model.ErrSessionNotFound) {
		t.Errorf("empty: want ErrSessionNotFound, got %v", err)
	}

	s := &model.TaskSession{
		TaskID: taskID, Status: model.SessionStatusActive, StartedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.FindRunningByTask(ctx, taskID)
	if err != nil {
		t.Fatalf("FindRunningByTask: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("got %v, want %v", got.ID, s.ID)
	}
}
