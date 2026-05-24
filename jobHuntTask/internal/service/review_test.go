package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Fake review repo
// ---------------------------------------------------------------------------

type fakeReviewRepo struct {
	mu      sync.Mutex
	byDate  map[time.Time]*model.DailyReview
	byID    map[uuid.UUID]*model.DailyReview
	now     func() time.Time
}

func newFakeReviewRepo(now func() time.Time) *fakeReviewRepo {
	return &fakeReviewRepo{
		byDate: map[time.Time]*model.DailyReview{},
		byID:   map[uuid.UUID]*model.DailyReview{},
		now:    now,
	}
}

func (r *fakeReviewRepo) Upsert(_ context.Context, rv *model.DailyReview) error {
	if err := rv.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rv.ReviewDate = model.NormalizeDate(rv.ReviewDate)
	now := r.now()
	if existing, ok := r.byDate[rv.ReviewDate]; ok {
		rv.ID = existing.ID
		rv.CreatedAt = existing.CreatedAt
	} else {
		rv.ID = uuid.New()
		rv.CreatedAt = now
	}
	rv.UpdatedAt = now
	cp := *rv
	r.byDate[rv.ReviewDate] = &cp
	r.byID[rv.ID] = &cp
	return nil
}

func (r *fakeReviewRepo) GetByDate(_ context.Context, date time.Time) (*model.DailyReview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rv, ok := r.byDate[model.NormalizeDate(date)]
	if !ok {
		return nil, model.ErrReviewNotFound
	}
	cp := *rv
	return &cp, nil
}
func (r *fakeReviewRepo) GetByID(_ context.Context, id uuid.UUID) (*model.DailyReview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rv, ok := r.byID[id]
	if !ok {
		return nil, model.ErrReviewNotFound
	}
	cp := *rv
	return &cp, nil
}
func (r *fakeReviewRepo) List(_ context.Context, _ repository.ReviewFilter) ([]*model.DailyReview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.DailyReview, 0, len(r.byDate))
	for _, rv := range r.byDate {
		cp := *rv
		out = append(out, &cp)
	}
	return out, nil
}
func (r *fakeReviewRepo) Delete(_ context.Context, date time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := model.NormalizeDate(date)
	rv, ok := r.byDate[d]
	if !ok {
		return model.ErrReviewNotFound
	}
	delete(r.byDate, d)
	delete(r.byID, rv.ID)
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func newReviewSvc(t *testing.T) (*service.DailyReviewService, *fakeReviewRepo, *fixedClock) {
	t.Helper()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	repo := newFakeReviewRepo(clk.Now)
	return service.NewDailyReviewService(repo, clk), repo, clk
}

func strPtr(s string) *string         { return &s }
func intPtr(i int) *int               { return &i }
func sliceStrPtr(v []string) *[]string { return &v }

func TestReviewService_UpsertCreatesAndUpdates(t *testing.T) {
	t.Parallel()
	svc, _, clk := newReviewSvc(t)
	ctx := context.Background()

	r, err := svc.Upsert(ctx, service.UpsertReviewInput{
		Date:              clk.Now(),
		Reflection:        strPtr("  focused day  "),
		Blockers:          sliceStrPtr([]string{"recruiter ghosted", "  ", "bug in resume"}),
		Wins:              sliceStrPtr([]string{"sent 5 apps"}),
		EnergyLevel:       intPtr(7),
		ProductivityScore: intPtr(8),
	})
	if err != nil {
		t.Fatalf("Upsert create: %v", err)
	}
	if r.Reflection != "focused day" {
		t.Errorf("reflection not trimmed: %q", r.Reflection)
	}
	if len(r.Blockers) != 2 {
		t.Errorf("blockers not sanitized: %v", r.Blockers)
	}
	if r.EnergyLevel != 7 || r.ProductivityScore != 8 {
		t.Errorf("scores not set: %d / %d", r.EnergyLevel, r.ProductivityScore)
	}

	// Upsert again with only one field — others must persist.
	r2, err := svc.Upsert(ctx, service.UpsertReviewInput{
		Date:        clk.Now(),
		EnergyLevel: intPtr(3),
	})
	if err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	if r2.EnergyLevel != 3 {
		t.Errorf("energy not updated: %d", r2.EnergyLevel)
	}
	if r2.Reflection != "focused day" {
		t.Errorf("reflection clobbered: %q", r2.Reflection)
	}
	if r2.ProductivityScore != 8 {
		t.Errorf("productivity clobbered: %d", r2.ProductivityScore)
	}
	if r2.ID != r.ID {
		t.Errorf("ID changed across upsert: %v vs %v", r2.ID, r.ID)
	}
}

func TestReviewService_UpsertRejectsBadScores(t *testing.T) {
	t.Parallel()
	svc, _, _ := newReviewSvc(t)
	if _, err := svc.Upsert(context.Background(), service.UpsertReviewInput{
		EnergyLevel: intPtr(11),
	}); !errors.Is(err, model.ErrInvalidEnergyLevel) {
		t.Errorf("want ErrInvalidEnergyLevel, got %v", err)
	}
	if _, err := svc.Upsert(context.Background(), service.UpsertReviewInput{
		ProductivityScore: intPtr(-1),
	}); !errors.Is(err, model.ErrInvalidProductivity) {
		t.Errorf("want ErrInvalidProductivity, got %v", err)
	}
}

func TestReviewService_DefaultsDateToNow(t *testing.T) {
	t.Parallel()
	svc, _, clk := newReviewSvc(t)
	r, err := svc.Upsert(context.Background(), service.UpsertReviewInput{
		Reflection: strPtr("anon"),
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !r.ReviewDate.Equal(model.NormalizeDate(clk.Now())) {
		t.Errorf("date defaulting failed: %v", r.ReviewDate)
	}
}
