package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

// ---------------------------------------------------------------------------
// Lightweight stub services
//
// Each stub satisfies just the methods the dashboard handler calls.
// We avoid the production constructors because they want full repos —
// for handler tests we only care about projection.
// ---------------------------------------------------------------------------

type stubMetricsRepo struct {
	statusBreakdown  model.StatusBreakdown
	overdueLive      int
	carryCounts      model.Counts
	avgActual        float64
	dailyCompletions []model.DailyCompletion
}

func (r *stubMetricsRepo) StatusBreakdown(_ context.Context, _, _ time.Time) (model.StatusBreakdown, error) {
	return r.statusBreakdown, nil
}
func (r *stubMetricsRepo) StatusBreakdownDueBefore(_ context.Context, _ time.Time) (model.StatusBreakdown, error) {
	return r.statusBreakdown, nil
}
func (r *stubMetricsRepo) CompletionCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return model.Counts{}, nil
}
func (r *stubMetricsRepo) CarryOverCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return r.carryCounts, nil
}
func (r *stubMetricsRepo) CarryOverCountsDueBefore(_ context.Context, _ time.Time) (model.Counts, error) {
	return r.carryCounts, nil
}
func (r *stubMetricsRepo) OverdueLive(_ context.Context, _ time.Time) (int, error) {
	return r.overdueLive, nil
}
func (r *stubMetricsRepo) AvgActualMinutes(_ context.Context, _, _ time.Time) (float64, error) {
	return r.avgActual, nil
}
func (r *stubMetricsRepo) CategoryStats(_ context.Context, _, _ time.Time) ([]model.CategoryStats, error) {
	return nil, nil
}
func (r *stubMetricsRepo) MostMissedCategory(_ context.Context, _, _ time.Time) (*model.CategoryMissed, error) {
	return nil, nil
}
func (r *stubMetricsRepo) DailyCompletionCounts(_ context.Context, _, _ time.Time) ([]model.DailyCompletion, error) {
	return r.dailyCompletions, nil
}
func (r *stubMetricsRepo) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return 0, 0, 0, nil
}

// ---------------------------------------------------------------------------
// In-memory task / review / reminder / suggestion repos (the absolute
// minimum to satisfy the service constructors).
// ---------------------------------------------------------------------------

type stubTaskRepo struct {
	mu    sync.Mutex
	items []*model.Task
}

func (r *stubTaskRepo) Create(_ context.Context, t *model.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	r.items = append(r.items, t)
	return nil
}
func (r *stubTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	for _, t := range r.items {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, model.ErrTaskNotFound
}
func (r *stubTaskRepo) Update(_ context.Context, _ uuid.UUID, _ repository.TaskUpdate) (*model.Task, error) {
	return nil, model.ErrTaskNotFound
}
func (r *stubTaskRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (r *stubTaskRepo) List(_ context.Context, f repository.TaskFilter) ([]*model.Task, error) {
	out := make([]*model.Task, 0, len(r.items))
	for _, t := range r.items {
		if len(f.Statuses) > 0 {
			match := false
			for _, s := range f.Statuses {
				if t.Status == s {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, t)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out, nil
}
func (r *stubTaskRepo) ListOverdue(_ context.Context, _ time.Time) ([]*model.Task, error) {
	return nil, nil
}

type stubReviewRepo struct {
	mu    sync.Mutex
	items []*model.DailyReview
}

func (r *stubReviewRepo) Upsert(_ context.Context, _ *model.DailyReview) error { return nil }
func (r *stubReviewRepo) GetByDate(_ context.Context, _ time.Time) (*model.DailyReview, error) {
	return nil, model.ErrReviewNotFound
}
func (r *stubReviewRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.DailyReview, error) {
	return nil, model.ErrReviewNotFound
}
func (r *stubReviewRepo) List(_ context.Context, f repository.ReviewFilter) ([]*model.DailyReview, error) {
	out := r.items
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}
func (r *stubReviewRepo) Delete(_ context.Context, _ time.Time) error { return nil }

type stubReminderRepo struct {
	items []*model.Reminder
}

func (r *stubReminderRepo) Schedule(_ context.Context, _ *model.Reminder) (bool, error) {
	return true, nil
}
func (r *stubReminderRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Reminder, error) {
	return nil, model.ErrReminderNotFound
}
func (r *stubReminderRepo) ListDue(_ context.Context, _ time.Time, _ int) ([]*model.Reminder, error) {
	return nil, nil
}
func (r *stubReminderRepo) List(_ context.Context, f repository.ReminderFilter) ([]*model.Reminder, error) {
	out := r.items
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}
func (r *stubReminderRepo) MarkSent(_ context.Context, _ uuid.UUID, _ time.Time, _ int) (*model.Reminder, error) {
	return nil, nil
}
func (r *stubReminderRepo) MarkFailed(_ context.Context, _ uuid.UUID, _ time.Time, _ int, _ string) (*model.Reminder, error) {
	return nil, nil
}
func (r *stubReminderRepo) MarkCancelled(_ context.Context, _ uuid.UUID) (*model.Reminder, error) {
	return nil, nil
}
func (r *stubReminderRepo) Requeue(_ context.Context, _ uuid.UUID, _ time.Time) (*model.Reminder, error) {
	return nil, nil
}
func (r *stubReminderRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

type stubSuggestionRepo struct {
	items []*model.Suggestion
}

func (r *stubSuggestionRepo) Upsert(_ context.Context, _ *model.Suggestion) (bool, error) {
	return true, nil
}
func (r *stubSuggestionRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Suggestion, error) {
	return nil, model.ErrSuggestionNotFound
}
func (r *stubSuggestionRepo) List(_ context.Context, f repository.SuggestionFilter) ([]*model.Suggestion, error) {
	out := r.items
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}
func (r *stubSuggestionRepo) Dismiss(_ context.Context, _ uuid.UUID, _ time.Time) (*model.Suggestion, error) {
	return nil, nil
}
func (r *stubSuggestionRepo) ExpireActiveExcept(_ context.Context, _ []model.SuggestionKind, _ time.Time) (int, error) {
	return 0, nil
}
func (r *stubSuggestionRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type dashboardHarness struct {
	router  *gin.Engine
	tasks   *stubTaskRepo
	reviews *stubReviewRepo
	reminds *stubReminderRepo
	sugs    *stubSuggestionRepo
	metrics *stubMetricsRepo
}

func newDashboardHarness(t *testing.T) *dashboardHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}

	mr := &stubMetricsRepo{}
	tr := &stubTaskRepo{}
	rr := &stubReviewRepo{}
	rmr := &stubReminderRepo{}
	sr := &stubSuggestionRepo{}

	taskSvc := service.NewTaskService(tr, service.SystemClock, nil)
	reviewSvc := service.NewDailyReviewService(rr, service.SystemClock)
	metricsSvc := service.NewMetricsService(mr, service.SystemClock, nil)
	reminderSvc := service.NewReminderService(rmr, nil, service.SystemClock,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		service.ReminderServiceConfig{MaxAttempts: 3, BatchSize: 50})
	sugSvc := service.NewSuggestionService(sr, mr, metricsSvc, nil, service.SystemClock, service.SuggestionServiceConfig{})

	h := web.NewDashboardHandler(rd, taskSvc, reviewSvc, reminderSvc, metricsSvc, sugSvc,
		service.SystemClock, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := gin.New()
	if err := web.MountStatic(r); err != nil {
		t.Fatalf("MountStatic: %v", err)
	}
	web.RegisterRoutes(r, rd)
	h.Register(r)

	return &dashboardHarness{
		router: r, tasks: tr, reviews: rr, reminds: rmr, sugs: sr, metrics: mr,
	}
}

func doGet(t *testing.T, r *gin.Engine, path string, htmx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDashboard_FullPageRendersAllCards(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)

	w := doGet(t, h.router, "/dashboard", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%.300s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`data-card="summary"`,
		`data-card="streak"`,
		`data-card="quick-actions"`,
		`data-card="activity"`,
		`data-card="trend"`,
		`data-card="suggestions"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing card marker %q", want)
		}
	}
	if !strings.Contains(body, `id="sidebar"`) {
		t.Error("full page missing sidebar")
	}
}

func TestDashboard_SummaryReflectsMetrics(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	h.metrics.statusBreakdown = model.StatusBreakdown{
		Completed: 3, Pending: 2, Missed: 1,
	}
	h.metrics.overdueLive = 2
	h.metrics.carryCounts = model.Counts{N: 1, Total: 1}

	w := doGet(t, h.router, "/dashboard/cards/summary", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// total = 6, completed = 3 -> 50% (server rounds; bucket maps to p50)
	if !strings.Contains(body, "summary-progress-bar--p50") {
		t.Errorf("expected progress bucket p50, got: %.500s", body)
	}
	for _, want := range []string{"Done", "3", "Overdue", "2", "Carried", "1", "summary-hero-value"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in summary", want)
		}
	}
}

func TestDashboard_SummaryEmptyState(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	// Default: zero everything.
	w := doGet(t, h.router, "/dashboard/cards/summary", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No tasks today") {
		t.Error("expected empty state for zero-task day")
	}
}

func TestDashboard_StreakIncludesLongestAndMissed(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	h.metrics.dailyCompletions = []model.DailyCompletion{
		{Date: today.AddDate(0, 0, -1), Count: 1},
		{Date: today, Count: 2},
	}

	w := doGet(t, h.router, "/dashboard/cards/streak", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Longest") {
		t.Error("missing 'Longest' label")
	}
	if !strings.Contains(body, "Missed this week") {
		t.Error("missing missed-day label")
	}
}

func TestDashboard_ActivityRendersCompletedTasks(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	completedAt := time.Now().Add(-2 * time.Hour)
	_ = h.tasks // ensure stub is wired
	h.tasks.items = append(h.tasks.items, &model.Task{
		ID:          uuid.New(),
		Title:       "Ship the dashboard",
		Status:      model.StatusCompleted,
		Category:    model.CategoryGithub,
		CompletedAt: &completedAt,
		CreatedAt:   completedAt,
	})

	w := doGet(t, h.router, "/dashboard/cards/activity", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Ship the dashboard") {
		t.Error("completed task title missing from activity card")
	}
	if !strings.Contains(body, "github") {
		t.Error("category missing from activity card")
	}
}

func TestDashboard_TrendBarsAndToday(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// 7-day rolling window starts at today-6; the service fills gaps but
	// we only need a single populated day to verify bucketing.
	h.metrics.dailyCompletions = []model.DailyCompletion{
		{Date: today.AddDate(0, 0, -2), Count: 2},
		{Date: today, Count: 4},
	}

	w := doGet(t, h.router, "/dashboard/cards/trend", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "trend-bars") {
		t.Error("trend chart container missing")
	}
	if !strings.Contains(body, "trend-bar--h10") {
		t.Errorf("expected peak day bucket h10, got: %.500s", body)
	}
	if !strings.Contains(body, "trend-bar-cell--today") {
		t.Errorf("expected today marker class")
	}
}

func TestDashboard_TrendEmpty(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	// no daily completions
	w := doGet(t, h.router, "/dashboard/cards/trend", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No data yet") {
		t.Error("expected empty state for no trend data")
	}
}

func TestDashboard_SuggestionsRenderItems(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	h.sugs.items = []*model.Suggestion{
		{
			ID:       uuid.New(),
			Kind:     model.SuggestionReduceWorkload,
			Severity: model.SeverityWarning,
			Status:   model.SuggestionStatusActive,
			Title:    "Reduce workload",
			Message:  "Too many missed tasks this week.",
			DedupKey: "reduce_workload:2026-W21",
			Payload:  map[string]any{},
		},
	}

	w := doGet(t, h.router, "/dashboard/cards/suggestions", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Reduce workload") {
		t.Error("suggestion title missing")
	}
	if !strings.Contains(body, "suggestion--warning") {
		t.Error("suggestion severity class missing")
	}
}

func TestDashboard_SuggestionsEmpty(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	w := doGet(t, h.router, "/dashboard/cards/suggestions", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Nothing to flag") {
		t.Error("expected empty state for zero suggestions")
	}
}

func TestDashboard_NilServicesRenderEmptyCards(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	// Every service nil — handler should still render the page without panicking.
	h := web.NewDashboardHandler(rd, nil, nil, nil, nil, nil, service.SystemClock, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := gin.New()
	if err := web.MountStatic(r); err != nil {
		t.Fatalf("MountStatic: %v", err)
	}
	web.RegisterRoutes(r, rd)
	h.Register(r)

	w := doGet(t, r, "/dashboard", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%.300s", w.Code, w.Body.String())
	}
	// Quick-actions card is static, so it must still render.
	if !strings.Contains(w.Body.String(), "Quick actions") {
		t.Error("expected static quick-actions card to render")
	}
}

func TestDashboard_AllCardFragmentsRespond(t *testing.T) {
	t.Parallel()
	h := newDashboardHarness(t)
	for _, path := range []string{
		"/dashboard/cards/summary",
		"/dashboard/cards/streak",
		"/dashboard/cards/activity",
		"/dashboard/cards/trend",
		"/dashboard/cards/suggestions",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			w := doGet(t, h.router, path, true)
			if w.Code != http.StatusOK {
				t.Fatalf("%s status = %d body=%s", path, w.Code, w.Body.String())
			}
			body := w.Body.String()
			if strings.Contains(body, "<!DOCTYPE html>") {
				t.Errorf("%s returned full page instead of fragment", path)
			}
			if !strings.Contains(body, `class="card`) {
				t.Errorf("%s did not return a card element", path)
			}
		})
	}
}
