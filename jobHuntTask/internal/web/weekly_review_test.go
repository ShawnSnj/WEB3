package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
	"github.com/shawn/jobhunttask/internal/web"
)

type memWeeklyReviewRepo struct {
	mu     sync.Mutex
	byWeek map[time.Time]*model.WeeklyReview
}

func newMemWeeklyReviewRepo() *memWeeklyReviewRepo {
	return &memWeeklyReviewRepo{byWeek: map[time.Time]*model.WeeklyReview{}}
}

func (r *memWeeklyReviewRepo) Upsert(_ context.Context, rv *model.WeeklyReview) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rv.WeekStart = model.NormalizeWeekStart(rv.WeekStart)
	if existing, ok := r.byWeek[rv.WeekStart]; ok {
		rv.ID = existing.ID
		rv.CreatedAt = existing.CreatedAt
	} else {
		rv.ID = uuid.New()
		rv.CreatedAt = time.Now()
	}
	rv.UpdatedAt = time.Now()
	cp := *rv
	r.byWeek[rv.WeekStart] = &cp
	return nil
}

func (r *memWeeklyReviewRepo) GetByWeekStart(_ context.Context, ws time.Time) (*model.WeeklyReview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rv, ok := r.byWeek[model.NormalizeWeekStart(ws)]
	if !ok {
		return nil, model.ErrWeeklyReviewNotFound
	}
	cp := *rv
	return &cp, nil
}

func (r *memWeeklyReviewRepo) Delete(_ context.Context, _ time.Time) error { return nil }

type stubMetricsRepoWeekly struct {
	breakdown model.StatusBreakdown
	carry     model.Counts
	daily     []model.DailyCompletion
	categories []model.CategoryStats
}

func (r *stubMetricsRepoWeekly) StatusBreakdown(_ context.Context, _, _ time.Time) (model.StatusBreakdown, error) {
	return r.breakdown, nil
}
func (r *stubMetricsRepoWeekly) CompletionCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return model.Counts{}, nil
}
func (r *stubMetricsRepoWeekly) CarryOverCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return r.carry, nil
}
func (r *stubMetricsRepoWeekly) OverdueLive(_ context.Context, _ time.Time) (int, error) { return 0, nil }
func (r *stubMetricsRepoWeekly) AvgActualMinutes(_ context.Context, _, _ time.Time) (float64, error) {
	return 0, nil
}
func (r *stubMetricsRepoWeekly) CategoryStats(_ context.Context, _, _ time.Time) ([]model.CategoryStats, error) {
	return r.categories, nil
}
func (r *stubMetricsRepoWeekly) MostMissedCategory(_ context.Context, _, _ time.Time) (*model.CategoryMissed, error) {
	return nil, nil
}
func (r *stubMetricsRepoWeekly) DailyCompletionCounts(_ context.Context, _, _ time.Time) ([]model.DailyCompletion, error) {
	return r.daily, nil
}
func (r *stubMetricsRepoWeekly) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return 0, 0, 0, nil
}

type weeklyHarness struct {
	router *gin.Engine
	reviews *memWeeklyReviewRepo
	clock  *fixedReviewClock
}

func newWeeklyHarness(t *testing.T) *weeklyHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	clk := &fixedReviewClock{t: now}

	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}

	reviews := newMemWeeklyReviewRepo()
	metricsRepo := &stubMetricsRepoWeekly{
		breakdown: model.StatusBreakdown{Completed: 5, Pending: 2, Missed: 1},
		carry:     model.Counts{N: 2, Total: 8},
		daily: []model.DailyCompletion{
			{Date: now.AddDate(0, 0, -2), Count: 2},
			{Date: now, Count: 3},
		},
		categories: []model.CategoryStats{
			{Category: model.CategoryGithub, Total: 4, Completed: 3, CompletionRate: 0.75},
			{Category: model.CategoryLearning, Total: 3, Completed: 1, CompletionRate: 0.33},
		},
	}
	metricsSvc := service.NewMetricsService(metricsRepo, clk)
	reviewSvc := service.NewWeeklyReviewService(reviews, clk)
	taskSvc := service.NewTaskService(newInMemTaskRepo(), clk)
	sessionSvc := service.NewTaskSessionService(&memSessionRepo{}, taskSvc, clk)
	sugSvc := service.NewSuggestionService(
		&weeklyStubSuggestionRepo{items: []*model.Suggestion{
			{Title: "Easier wins", Message: "Build momentum", Severity: model.SeverityInfo, Status: model.SuggestionStatusActive, Kind: model.SuggestionEasierWins, DedupKey: "x"},
		}},
		metricsRepo, metricsSvc, nil, clk,
		service.SuggestionServiceConfig{},
	)

	h := web.NewWeeklyReviewHandler(rd, reviewSvc, metricsSvc, sessionSvc, sugSvc, clk,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := gin.New()
	if err := web.MountStatic(r); err != nil {
		t.Fatalf("MountStatic: %v", err)
	}
	web.RegisterRoutes(r, rd)
	h.Register(r)
	return &weeklyHarness{router: r, reviews: reviews, clock: clk}
}

func TestWeeklyReviewPage_RendersReport(t *testing.T) {
	t.Parallel()
	h := newWeeklyHarness(t)
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/reviews/weekly", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%.400s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"Weekly Review", "Completion rate", "Missed tasks", "Carry-over",
		"Streak stats", "Category ROI", "Suggestion summary", "Wins",
		"Bottlenecks", "Improvement notes", "Next week priorities",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWeeklyReview_AutosaveWins(t *testing.T) {
	t.Parallel()
	h := newWeeklyHarness(t)
	form := url.Values{}
	form.Set("wins", "Great week overall")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/reviews/weekly/autosave?section=wins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	ws := model.NormalizeDate(h.clock.Now()).AddDate(0, 0, -6)
	rv, _ := h.reviews.GetByWeekStart(context.Background(), ws)
	if rv.Wins != "Great week overall" {
		t.Errorf("wins=%q", rv.Wins)
	}
}

func TestWeeklyReview_StatsFragment(t *testing.T) {
	t.Parallel()
	h := newWeeklyHarness(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reviews/weekly/cards/stats", nil)
	req.Header.Set("HX-Request", "true")
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Completion rate") || !strings.Contains(body, "5 of 8") {
		t.Errorf("expected completion stats, got: %.300s", body)
	}
}

func TestWeeklyReview_CategoriesFragment(t *testing.T) {
	t.Parallel()
	h := newWeeklyHarness(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reviews/weekly/cards/categories", nil)
	req.Header.Set("HX-Request", "true")
	h.router.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "GitHub") || !strings.Contains(body, "Learning") {
		t.Errorf("expected category names, got: %.300s", body)
	}
}

func TestWeeklyReview_SuggestionsFragment(t *testing.T) {
	t.Parallel()
	h := newWeeklyHarness(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reviews/weekly/cards/suggestions", nil)
	req.Header.Set("HX-Request", "true")
	h.router.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "Easier wins") {
		t.Error("expected suggestion in fragment")
	}
}

func TestWeeklyReview_ChartsFragment(t *testing.T) {
	t.Parallel()
	h := newWeeklyHarness(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reviews/weekly/cards/charts", nil)
	req.Header.Set("HX-Request", "true")
	h.router.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "Daily completions") || !strings.Contains(body, "Carry-over trend") {
		t.Errorf("expected chart sections, got: %.300s", body)
	}
}

type weeklyStubSuggestionRepo struct{ items []*model.Suggestion }

func (r *weeklyStubSuggestionRepo) Upsert(_ context.Context, _ *model.Suggestion) (bool, error) {
	return true, nil
}
func (r *weeklyStubSuggestionRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Suggestion, error) {
	return nil, model.ErrSuggestionNotFound
}
func (r *weeklyStubSuggestionRepo) List(_ context.Context, _ repository.SuggestionFilter) ([]*model.Suggestion, error) {
	return r.items, nil
}
func (r *weeklyStubSuggestionRepo) Dismiss(_ context.Context, _ uuid.UUID, _ time.Time) (*model.Suggestion, error) {
	return nil, nil
}
func (r *weeklyStubSuggestionRepo) ExpireActiveExcept(_ context.Context, _ []model.SuggestionKind, _ time.Time) (int, error) {
	return 0, nil
}
func (r *weeklyStubSuggestionRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
