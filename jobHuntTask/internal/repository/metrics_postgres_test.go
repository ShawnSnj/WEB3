//go:build integration

// Postgres-backed integration tests for the metrics repository.
//
// Run with:
//
//	TEST_DATABASE_URL=postgres://jobhunt:jobhunt@localhost:5432/jobhunt?sslmode=disable \
//	    go test -tags=integration ./internal/repository -count=1
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// seedMetricsTask inserts a single task row in whatever state the test needs.
// We fill in safe defaults so callers only need to supply the fields they
// care about for the assertion at hand.
func seedMetricsTask(t *testing.T, repo *repository.PostgresTaskRepository, task *model.Task) {
	t.Helper()
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	if task.Status == "" {
		task.Status = model.StatusPending
	}
	if task.Priority == "" {
		task.Priority = model.PriorityMedium
	}
	if task.Category == "" {
		task.Category = model.CategoryMisc
	}
	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestMetrics_StatusBreakdown_And_CompletionCounts(t *testing.T) {
	pool := newTestPool(t)
	taskRepo := repository.NewPostgresTaskRepository(pool)
	metricsRepo := repository.NewPostgresMetricsRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	for _, s := range []model.Status{
		model.StatusPending, model.StatusInProgress,
		model.StatusCompleted, model.StatusCompleted,
		model.StatusMissed,
	} {
		seedMetricsTask(t, taskRepo, &model.Task{Title: "t-" + string(s), Status: s})
	}

	bd, err := metricsRepo.StatusBreakdown(ctx, from, to)
	if err != nil {
		t.Fatalf("StatusBreakdown: %v", err)
	}
	if bd.Pending != 1 || bd.InProgress != 1 || bd.Completed != 2 || bd.Missed != 1 {
		t.Fatalf("breakdown = %+v", bd)
	}

	c, err := metricsRepo.CompletionCounts(ctx, from, to)
	if err != nil {
		t.Fatalf("CompletionCounts: %v", err)
	}
	if c.N != 2 || c.Total != 5 {
		t.Fatalf("completion = %+v", c)
	}
}

func TestMetrics_CarryOver_And_AvgActualMinutes(t *testing.T) {
	pool := newTestPool(t)
	taskRepo := repository.NewPostgresTaskRepository(pool)
	metricsRepo := repository.NewPostgresMetricsRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	// Two carried-over tasks
	for i := 0; i < 2; i++ {
		seedMetricsTask(t, taskRepo, &model.Task{Title: "carried", CarryOverCount: 1})
	}
	// Three normal tasks; two completed with actual minutes
	for i := 0; i < 3; i++ {
		seedMetricsTask(t, taskRepo, &model.Task{Title: "normal"})
	}

	carry, err := metricsRepo.CarryOverCounts(ctx, from, to)
	if err != nil {
		t.Fatalf("CarryOverCounts: %v", err)
	}
	if carry.N != 2 || carry.Total != 5 {
		t.Fatalf("carry = %+v", carry)
	}

	// Mark two as completed with explicit actual_minutes by direct update.
	if _, err := pool.Exec(ctx,
		`UPDATE tasks SET status = 'completed', completed_at = NOW(), actual_minutes = 30
		 WHERE title = 'normal' AND id IN (SELECT id FROM tasks WHERE title = 'normal' LIMIT 2)`); err != nil {
		t.Fatalf("update: %v", err)
	}

	avg, err := metricsRepo.AvgActualMinutes(ctx, from, to)
	if err != nil {
		t.Fatalf("AvgActualMinutes: %v", err)
	}
	if avg < 29.5 || avg > 30.5 {
		t.Fatalf("avg = %v, want ~30", avg)
	}
}

func TestMetrics_OverdueLive(t *testing.T) {
	pool := newTestPool(t)
	taskRepo := repository.NewPostgresTaskRepository(pool)
	metricsRepo := repository.NewPostgresMetricsRepository(pool)
	ctx := context.Background()

	past := time.Now().Add(-2 * time.Hour)
	future := time.Now().Add(2 * time.Hour)

	seedMetricsTask(t, taskRepo, &model.Task{Title: "overdue-pending", DueDate: &past, Status: model.StatusPending})
	seedMetricsTask(t, taskRepo, &model.Task{Title: "overdue-inprog", DueDate: &past, Status: model.StatusInProgress})
	seedMetricsTask(t, taskRepo, &model.Task{Title: "not-due", DueDate: &future, Status: model.StatusPending})
	seedMetricsTask(t, taskRepo, &model.Task{Title: "no-due", Status: model.StatusPending})
	// Completed should not count even if overdue.
	completed := time.Now().Add(-1 * time.Hour)
	seedMetricsTask(t, taskRepo, &model.Task{Title: "completed", DueDate: &past, Status: model.StatusCompleted, CompletedAt: &completed})

	n, err := metricsRepo.OverdueLive(ctx, time.Now())
	if err != nil {
		t.Fatalf("OverdueLive: %v", err)
	}
	if n != 2 {
		t.Fatalf("overdue = %d, want 2", n)
	}
}

func TestMetrics_CategoryStats_And_MostMissed(t *testing.T) {
	pool := newTestPool(t)
	taskRepo := repository.NewPostgresTaskRepository(pool)
	metricsRepo := repository.NewPostgresMetricsRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now.Add(24 * time.Hour)

	seedMetricsTask(t, taskRepo, &model.Task{Title: "ja-1", Category: model.CategoryJobApply})
	seedMetricsTask(t, taskRepo, &model.Task{Title: "ja-2", Category: model.CategoryJobApply})
	seedMetricsTask(t, taskRepo, &model.Task{Title: "gh-1", Category: model.CategoryGithub})

	// Force gh-1 to missed.
	if _, err := pool.Exec(ctx,
		`UPDATE tasks SET status = 'missed' WHERE title = 'gh-1'`); err != nil {
		t.Fatalf("update missed: %v", err)
	}
	// Complete one job_apply with 60 actual minutes.
	if _, err := pool.Exec(ctx,
		`UPDATE tasks SET status='completed', completed_at=NOW(), actual_minutes=60, estimated_minutes=30
		 WHERE title='ja-1'`); err != nil {
		t.Fatalf("update completed: %v", err)
	}

	cats, err := metricsRepo.CategoryStats(ctx, from, to)
	if err != nil {
		t.Fatalf("CategoryStats: %v", err)
	}
	if len(cats) < 2 {
		t.Fatalf("expected at least 2 categories, got %d", len(cats))
	}
	var ja, gh *model.CategoryStats
	for i := range cats {
		switch cats[i].Category {
		case model.CategoryJobApply:
			ja = &cats[i]
		case model.CategoryGithub:
			gh = &cats[i]
		}
	}
	if ja == nil || gh == nil {
		t.Fatalf("missing category rows: %+v", cats)
	}
	if ja.Total != 2 || ja.Completed != 1 {
		t.Errorf("job_apply = %+v, want total=2 completed=1", ja)
	}
	if ja.AvgActualMinutes != 60 {
		t.Errorf("job_apply avg actual = %v, want 60", ja.AvgActualMinutes)
	}
	if gh.Missed != 1 {
		t.Errorf("github missed = %d, want 1", gh.Missed)
	}

	mm, err := metricsRepo.MostMissedCategory(ctx, from, to)
	if err != nil {
		t.Fatalf("MostMissedCategory: %v", err)
	}
	if mm == nil || mm.Category != model.CategoryGithub || mm.Count != 1 {
		t.Fatalf("most-missed = %+v", mm)
	}
}

func TestMetrics_DailyCompletionCounts(t *testing.T) {
	pool := newTestPool(t)
	taskRepo := repository.NewPostgresTaskRepository(pool)
	metricsRepo := repository.NewPostgresMetricsRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -7)
	to := now.Add(24 * time.Hour)

	seedMetricsTask(t, taskRepo, &model.Task{Title: "a"})
	seedMetricsTask(t, taskRepo, &model.Task{Title: "b"})

	yesterday := time.Now().AddDate(0, 0, -1)
	today := time.Now()
	if _, err := pool.Exec(ctx,
		`UPDATE tasks SET status='completed', completed_at=$1 WHERE title='a'`, yesterday); err != nil {
		t.Fatalf("update a: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE tasks SET status='completed', completed_at=$1 WHERE title='b'`, today); err != nil {
		t.Fatalf("update b: %v", err)
	}

	rows, err := metricsRepo.DailyCompletionCounts(ctx, from, to)
	if err != nil {
		t.Fatalf("DailyCompletionCounts: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Count != 1 || rows[1].Count != 1 {
		t.Errorf("rows = %+v", rows)
	}
}
