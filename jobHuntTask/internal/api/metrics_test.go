package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/api"
	"github.com/shawn/jobhunttask/internal/config"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Minimal fake repo that returns the same canned values for any window
// ---------------------------------------------------------------------------

type stubMetricsRepo struct {
	breakdown StatusBreakdownByCall
	completion model.Counts
	carry      model.Counts
	overdue    int
	avg        float64
	cats       []model.CategoryStats
	missed     *model.CategoryMissed
	daily      []model.DailyCompletion
}

// allow returning different breakdowns to today vs weekly without exact key
// matching by using counters for simplicity.
type StatusBreakdownByCall struct {
	First model.StatusBreakdown
	Then  model.StatusBreakdown
	calls int
}

func (r *stubMetricsRepo) StatusBreakdown(_ context.Context, _, _ time.Time) (model.StatusBreakdown, error) {
	r.breakdown.calls++
	if r.breakdown.calls == 1 {
		return r.breakdown.First, nil
	}
	return r.breakdown.Then, nil
}
func (r *stubMetricsRepo) CompletionCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return r.completion, nil
}
func (r *stubMetricsRepo) CarryOverCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return r.carry, nil
}
func (r *stubMetricsRepo) OverdueLive(_ context.Context, _ time.Time) (int, error) {
	return r.overdue, nil
}
func (r *stubMetricsRepo) AvgActualMinutes(_ context.Context, _, _ time.Time) (float64, error) {
	return r.avg, nil
}
func (r *stubMetricsRepo) CategoryStats(_ context.Context, _, _ time.Time) ([]model.CategoryStats, error) {
	return r.cats, nil
}
func (r *stubMetricsRepo) MostMissedCategory(_ context.Context, _, _ time.Time) (*model.CategoryMissed, error) {
	return r.missed, nil
}
func (r *stubMetricsRepo) DailyCompletionCounts(_ context.Context, _, _ time.Time) ([]model.DailyCompletion, error) {
	return r.daily, nil
}
func (r *stubMetricsRepo) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return 0, 0, 0, nil
}

func newMetricsRouter(t *testing.T, repo *stubMetricsRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := service.NewMetricsService(repo, service.SystemClock)
	return api.NewRouter(api.Deps{
		Config:         config.Config{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		MetricsService: svc,
	})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAPI_Metrics_Today(t *testing.T) {
	t.Parallel()
	repo := &stubMetricsRepo{
		breakdown: StatusBreakdownByCall{
			First: model.StatusBreakdown{Completed: 4, Pending: 1},
		},
		overdue: 3,
		avg:     22.0,
	}
	r := newMetricsRouter(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/today", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if got := resp["completed_total"]; got.(float64) != 4 {
		t.Errorf("completed_total = %v, want 4", got)
	}
	if got := resp["overdue_count"]; got.(float64) != 3 {
		t.Errorf("overdue_count = %v, want 3", got)
	}
}

func TestAPI_Metrics_Dashboard(t *testing.T) {
	t.Parallel()
	repo := &stubMetricsRepo{
		breakdown: StatusBreakdownByCall{
			First: model.StatusBreakdown{Completed: 2, Pending: 1},
			Then:  model.StatusBreakdown{Completed: 5, Missed: 1},
		},
		completion: model.Counts{N: 5, Total: 6},
		missed:     &model.CategoryMissed{Category: model.CategoryTwitter, Count: 1},
	}
	r := newMetricsRouter(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["today"] == nil {
		t.Fatal("missing today")
	}
	if resp["weekly"] == nil {
		t.Fatal("missing weekly")
	}
	if resp["trend"] == nil {
		t.Fatal("missing trend")
	}
	if resp["streak"] == nil {
		t.Fatal("missing streak")
	}
	mm := resp["most_missed_category"].(map[string]any)
	if mm["category"] != "twitter" {
		t.Errorf("most_missed category = %v, want twitter", mm["category"])
	}
}

func TestAPI_Metrics_CategoriesBadParam(t *testing.T) {
	t.Parallel()
	r := newMetricsRouter(t, &stubMetricsRepo{})

	cases := []struct {
		name string
		path string
		want int
	}{
		{"only_from", "/api/v1/metrics/categories?from=2026-05-01", http.StatusBadRequest},
		{"bad_date", "/api/v1/metrics/categories?from=foo&to=bar", http.StatusBadRequest},
		{"to_before_from", "/api/v1/metrics/categories?from=2026-05-10&to=2026-05-01", http.StatusBadRequest},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestAPI_Metrics_CategoriesOK(t *testing.T) {
	t.Parallel()
	repo := &stubMetricsRepo{
		cats: []model.CategoryStats{
			{Category: model.CategoryJobApply, Total: 5, Completed: 4, CompletionRate: 0.8},
			{Category: model.CategoryGithub, Total: 3, Completed: 1, CompletionRate: 0.33},
		},
	}
	r := newMetricsRouter(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/categories?from=2026-05-01&to=2026-05-24", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["count"].(float64) != 2 {
		t.Errorf("count = %v, want 2", resp["count"])
	}
}
