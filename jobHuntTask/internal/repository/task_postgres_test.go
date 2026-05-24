//go:build integration

// Postgres-backed integration test for the task repository.
//
// Run with:
//
//	TEST_DATABASE_URL=postgres://jobhunt:jobhunt@localhost:5432/jobhunt?sslmode=disable \
//	    go test -tags=integration ./internal/repository -count=1
//
// The test owns its data — it truncates the tasks table at start and end.
package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// CASCADE so children in task_execution_sessions are wiped too.
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE tasks CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE tasks CASCADE`)
		pool.Close()
	})
	return pool
}

func TestPostgres_CRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := repository.NewPostgresTaskRepository(pool)
	ctx := context.Background()

	task := &model.Task{
		Title:            "Apply to Acme",
		Description:      "Write tailored cover letter",
		Priority:         model.PriorityHigh,
		Category:         model.CategoryJobApply,
		Status:           model.StatusPending,
		EstimatedMinutes: 60,
	}
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID.String() == "" || task.CreatedAt.IsZero() {
		t.Fatal("Create did not populate id/timestamps")
	}

	got, err := repo.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != task.Title {
		t.Errorf("title mismatch: %q", got.Title)
	}

	newTitle := "Apply to BigCo"
	updated, err := repo.Update(ctx, task.ID, repository.TaskUpdate{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("update did not persist: %q", updated.Title)
	}

	if err := repo.Delete(ctx, task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, task.ID); !errors.Is(err, model.ErrTaskNotFound) {
		t.Errorf("after delete: want ErrTaskNotFound, got %v", err)
	}
}

func TestPostgres_ListOverdue(t *testing.T) {
	pool := newTestPool(t)
	repo := repository.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	mk := func(title string, due *time.Time, status model.Status) {
		t.Helper()
		err := repo.Create(ctx, &model.Task{
			Title:    title,
			Priority: model.PriorityMedium,
			Category: model.CategoryMisc,
			Status:   status,
			DueDate:  due,
		})
		if err != nil {
			t.Fatalf("seed %s: %v", title, err)
		}
	}
	mk("overdue-pending", &past, model.StatusPending)
	mk("overdue-inprogress", &past, model.StatusInProgress)
	mk("overdue-completed", &past, model.StatusCompleted) // should be excluded
	mk("future-pending", &future, model.StatusPending)    // should be excluded

	got, err := repo.ListOverdue(ctx, now)
	if err != nil {
		t.Fatalf("ListOverdue: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 overdue, got %d", len(got))
	}
}

func TestPostgres_RejectsInvalidEnum(t *testing.T) {
	pool := newTestPool(t)
	repo := repository.NewPostgresTaskRepository(pool)
	// Bypass model.Validate by writing through Update which doesn't validate.
	t1 := &model.Task{
		Title: "x", Priority: model.PriorityMedium,
		Category: model.CategoryMisc, Status: model.StatusPending,
	}
	if err := repo.Create(context.Background(), t1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	badStatus := model.Status("invalid_status")
	_, err := repo.Update(context.Background(), t1.ID, repository.TaskUpdate{Status: &badStatus})
	if !errors.Is(err, model.ErrInvalidStatus) {
		t.Errorf("want ErrInvalidStatus, got %v", err)
	}
}
