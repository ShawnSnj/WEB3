package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Fake suggestion repository
// ---------------------------------------------------------------------------

type fakeSuggestionRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.Suggestion
	byKey  map[string]*model.Suggestion // active rows only
}

func newFakeSuggestionRepo() *fakeSuggestionRepo {
	return &fakeSuggestionRepo{
		byID:  map[uuid.UUID]*model.Suggestion{},
		byKey: map[string]*model.Suggestion{},
	}
}

func (r *fakeSuggestionRepo) Upsert(_ context.Context, s *model.Suggestion) (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.byKey[s.DedupKey]; ok {
		*s = *existing
		return false, nil
	}
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	s.UpdatedAt = s.CreatedAt
	cp := *s
	r.byID[s.ID] = &cp
	r.byKey[s.DedupKey] = &cp
	return true, nil
}
func (r *fakeSuggestionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Suggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.byID[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, model.ErrSuggestionNotFound
}
func (r *fakeSuggestionRepo) List(_ context.Context, f repository.SuggestionFilter) ([]*model.Suggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Suggestion, 0, len(r.byID))
	for _, s := range r.byID {
		if len(f.Statuses) > 0 {
			ok := false
			for _, st := range f.Statuses {
				if s.Status == st {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if len(f.Kinds) > 0 {
			ok := false
			for _, k := range f.Kinds {
				if s.Kind == k {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		cp := *s
		out = append(out, &cp)
	}
	return out, nil
}
func (r *fakeSuggestionRepo) Dismiss(_ context.Context, id uuid.UUID, at time.Time) (*model.Suggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok || s.Status != model.SuggestionStatusActive {
		return nil, model.ErrSuggestionNotFound
	}
	s.Status = model.SuggestionStatusDismissed
	s.DismissedAt = &at
	// Note: do NOT remove from byKey — total uniqueness on dedup_key.
	cp := *s
	return &cp, nil
}
func (r *fakeSuggestionRepo) ExpireActiveExcept(_ context.Context, keep []model.SuggestionKind, at time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	keepSet := map[model.SuggestionKind]struct{}{}
	for _, k := range keep {
		keepSet[k] = struct{}{}
	}
	n := 0
	for _, s := range r.byID {
		if s.Status != model.SuggestionStatusActive {
			continue
		}
		if _, ok := keepSet[s.Kind]; ok {
			continue
		}
		s.Status = model.SuggestionStatusExpired
		s.ExpiresAt = &at
		n++
	}
	return n, nil
}
func (r *fakeSuggestionRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok {
		return model.ErrSuggestionNotFound
	}
	delete(r.byID, id)
	delete(r.byKey, s.DedupKey)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers — seed the fake metrics repo so EvaluateRefresh has both rules
// firing.
// ---------------------------------------------------------------------------

func seedAlarmingMetrics(repo *fakeMetricsRepo, clk *fixedClock) {
	today, tomorrow, weekFrom, weekTo, _, _ := windows(clk.t)

	repo.statusBreakdown[key(today, tomorrow)] = model.StatusBreakdown{
		Completed: 0, Pending: 2, Missed: 0,
	}
	// Weekly: dramatic missed rate, eligible for reduce_workload AND smaller_tasks.
	repo.statusBreakdown[key(weekFrom, weekTo)] = model.StatusBreakdown{
		Completed: 1, Missed: 6, Pending: 3,
	}
	// Categories: two laggards -> focus_shift
	repo.categoryStats[key(weekFrom, weekTo)] = []model.CategoryStats{
		{Category: model.CategoryGithub, Total: 4, Completed: 0, CompletionRate: 0},
		{Category: model.CategoryTwitter, Total: 4, Completed: 1, CompletionRate: 0.25},
	}
	// streak window: today empty, yesterday empty -> low streak
	streakFrom := startOfDay(clk.t).AddDate(0, 0, -365)
	streakTo := startOfDay(clk.t).Add(24 * time.Hour)
	repo.dailyCompletions[key(streakFrom, streakTo)] = nil
}

// ---------------------------------------------------------------------------
// fakeMetricsRepo extension: EffortDistribution lookup
// ---------------------------------------------------------------------------

type effortMetricsRepo struct {
	*fakeMetricsRepo
	avg   float64
	large int
	total int
}

func (r *effortMetricsRepo) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return r.avg, r.large, r.total, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSuggestion_Refresh_FiresAndPersists(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	metricsRepo := &effortMetricsRepo{
		fakeMetricsRepo: newFakeMetricsRepo(),
		avg:             80, large: 6, total: 10, // high-effort signal
	}
	seedAlarmingMetrics(metricsRepo.fakeMetricsRepo, clk)
	metricsSvc := service.NewMetricsService(metricsRepo, clk)

	sugRepo := newFakeSuggestionRepo()
	svc := service.NewSuggestionService(
		sugRepo, metricsRepo, metricsSvc, nil, clk,
		service.SuggestionServiceConfig{},
	)

	res, err := svc.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(res.Created) != 4 {
		t.Fatalf("expected 4 fresh suggestions, got %d (%+v)", len(res.Created), kindsOf(res.Created))
	}

	// Second Refresh on the same week is a no-op for Created — every active
	// row already exists; we expect 4 in Kept, 0 in Created.
	res2, err := svc.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh #2: %v", err)
	}
	if len(res2.Created) != 0 {
		t.Errorf("second refresh created %d (expected 0)", len(res2.Created))
	}
	if len(res2.Kept) != 4 {
		t.Errorf("second refresh kept %d (expected 4)", len(res2.Kept))
	}
}

func TestSuggestion_Refresh_ExpiresStaleKinds(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	metricsRepo := &effortMetricsRepo{
		fakeMetricsRepo: newFakeMetricsRepo(),
		avg:             80, large: 6, total: 10,
	}
	seedAlarmingMetrics(metricsRepo.fakeMetricsRepo, clk)
	metricsSvc := service.NewMetricsService(metricsRepo, clk)

	sugRepo := newFakeSuggestionRepo()
	svc := service.NewSuggestionService(
		sugRepo, metricsRepo, metricsSvc, nil, clk,
		service.SuggestionServiceConfig{},
	)

	// First refresh: 4 fire.
	if _, err := svc.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh #1: %v", err)
	}

	// Now flip the metrics to healthy.
	_, _, weekFrom, weekTo, _, _ := windows(clk.t)
	metricsRepo.fakeMetricsRepo.statusBreakdown[key(weekFrom, weekTo)] = model.StatusBreakdown{
		Completed: 10, Pending: 0, Missed: 0,
	}
	metricsRepo.fakeMetricsRepo.categoryStats[key(weekFrom, weekTo)] = []model.CategoryStats{
		{Category: model.CategoryJobApply, Total: 10, Completed: 9, CompletionRate: 0.9},
	}
	streakFrom := startOfDay(clk.t).AddDate(0, 0, -365)
	streakTo := startOfDay(clk.t).Add(24 * time.Hour)
	metricsRepo.fakeMetricsRepo.dailyCompletions[key(streakFrom, streakTo)] = []model.DailyCompletion{
		{Date: startOfDay(clk.t), Count: 3},
		{Date: startOfDay(clk.t).AddDate(0, 0, -1), Count: 3},
		{Date: startOfDay(clk.t).AddDate(0, 0, -2), Count: 3},
	}
	metricsRepo.avg = 20
	metricsRepo.large = 1
	metricsRepo.total = 10

	res2, err := svc.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh #2: %v", err)
	}
	if len(res2.Created) != 0 {
		t.Errorf("refresh #2 created = %d, want 0", len(res2.Created))
	}
	if res2.ExpiredCount != 4 {
		t.Errorf("expired = %d, want 4", res2.ExpiredCount)
	}

	active, _ := svc.ListActive(context.Background())
	if len(active) != 0 {
		t.Errorf("active list = %d, want 0", len(active))
	}
}

func TestSuggestion_Dismiss(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	metricsRepo := &effortMetricsRepo{
		fakeMetricsRepo: newFakeMetricsRepo(),
		avg:             80, large: 6, total: 10,
	}
	seedAlarmingMetrics(metricsRepo.fakeMetricsRepo, clk)
	metricsSvc := service.NewMetricsService(metricsRepo, clk)

	sugRepo := newFakeSuggestionRepo()
	svc := service.NewSuggestionService(
		sugRepo, metricsRepo, metricsSvc, nil, clk,
		service.SuggestionServiceConfig{},
	)

	res, err := svc.Refresh(context.Background())
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	id := res.Created[0].ID

	got, err := svc.Dismiss(context.Background(), id)
	if err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if got.Status != model.SuggestionStatusDismissed {
		t.Errorf("status = %v, want dismissed", got.Status)
	}

	// Dismissing again should yield InvalidTransition (terminal state).
	if _, err := svc.Dismiss(context.Background(), id); err == nil {
		t.Error("expected error dismissing already-dismissed row")
	}
}

// Dismissals must stick within the same ISO week — re-running Refresh
// after a dismiss should NOT resurrect the same suggestion.
func TestSuggestion_DismissedStaysSuppressedWithinWeek(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	metricsRepo := &effortMetricsRepo{
		fakeMetricsRepo: newFakeMetricsRepo(),
		avg:             80, large: 6, total: 10,
	}
	seedAlarmingMetrics(metricsRepo.fakeMetricsRepo, clk)
	metricsSvc := service.NewMetricsService(metricsRepo, clk)

	sugRepo := newFakeSuggestionRepo()
	svc := service.NewSuggestionService(
		sugRepo, metricsRepo, metricsSvc, nil, clk,
		service.SuggestionServiceConfig{},
	)

	res, _ := svc.Refresh(context.Background())
	first := res.Created[0]
	if _, err := svc.Dismiss(context.Background(), first.ID); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	res2, _ := svc.Refresh(context.Background())
	for _, sg := range res2.Created {
		if sg.Kind == first.Kind {
			t.Errorf("dismissed kind %q was re-created", first.Kind)
		}
	}
	// And it should remain dismissed.
	got, err := svc.Get(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.SuggestionStatusDismissed {
		t.Errorf("status = %v, want dismissed", got.Status)
	}
}

func kindsOf(in []*model.Suggestion) []model.SuggestionKind {
	out := make([]model.SuggestionKind, 0, len(in))
	for _, s := range in {
		out = append(out, s.Kind)
	}
	return out
}
