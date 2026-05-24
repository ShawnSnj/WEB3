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

func TestPostgres_ReviewUpsertAndGet(t *testing.T) {
	pool := newTestPool(t)
	// newTestPool truncates tasks (CASCADE) but not daily_reviews — clean
	// our own table here.
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE daily_reviews CASCADE`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE daily_reviews CASCADE`)
	})

	repo := repository.NewPostgresReviewRepository(pool)
	ctx := context.Background()
	date := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

	rv := &model.DailyReview{
		ReviewDate:        date,
		Reflection:        "focused",
		Blockers:          []string{"recruiter ghosted"},
		Wins:              []string{"3 apps"},
		EnergyLevel:       7,
		ProductivityScore: 8,
	}
	if err := repo.Upsert(ctx, rv); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	firstID := rv.ID

	// Re-upsert same date: ID must remain stable, fields update.
	rv2 := &model.DailyReview{
		ReviewDate:  date,
		Reflection:  "updated",
		EnergyLevel: 4,
	}
	if err := repo.Upsert(ctx, rv2); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	if rv2.ID != firstID {
		t.Errorf("ID changed: %v vs %v", rv2.ID, firstID)
	}

	got, err := repo.GetByDate(ctx, date)
	if err != nil {
		t.Fatalf("GetByDate: %v", err)
	}
	if got.Reflection != "updated" {
		t.Errorf("reflection: %q", got.Reflection)
	}
	if got.EnergyLevel != 4 {
		t.Errorf("energy: %d", got.EnergyLevel)
	}

	// Missing date.
	if _, err := repo.GetByDate(ctx, date.Add(24*time.Hour)); !errors.Is(err, model.ErrReviewNotFound) {
		t.Errorf("want ErrReviewNotFound, got %v", err)
	}
}

func TestPostgres_ReviewListAndDelete(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE daily_reviews CASCADE`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE daily_reviews CASCADE`)
	})

	repo := repository.NewPostgresReviewRepository(pool)
	ctx := context.Background()
	base := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		d := base.AddDate(0, 0, i)
		if err := repo.Upsert(ctx, &model.DailyReview{ReviewDate: d, EnergyLevel: i + 1}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	from := base.AddDate(0, 0, 1)
	to := base.AddDate(0, 0, 3)
	got, err := repo.List(ctx, repository.ReviewFilter{From: &from, To: &to})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3, got %d", len(got))
	}

	if err := repo.Delete(ctx, base); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, base); !errors.Is(err, model.ErrReviewNotFound) {
		t.Errorf("want ErrReviewNotFound, got %v", err)
	}
}
