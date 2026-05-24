package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
	"github.com/shawn/jobhunttask/internal/web"
)

type stubAnalyticsMetricsRepo struct {
	breakdown  model.StatusBreakdown
	completed  model.Counts
	carry      model.Counts
	avgActual  float64
	daily      []model.DailyCompletion
	categories []model.CategoryStats
}

func (r *stubAnalyticsMetricsRepo) StatusBreakdown(_ context.Context, _, _ time.Time) (model.StatusBreakdown, error) {
	return r.breakdown, nil
}
func (r *stubAnalyticsMetricsRepo) CompletionCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return r.completed, nil
}
func (r *stubAnalyticsMetricsRepo) CarryOverCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return r.carry, nil
}
func (r *stubAnalyticsMetricsRepo) OverdueLive(_ context.Context, _ time.Time) (int, error) {
	return 0, nil
}
func (r *stubAnalyticsMetricsRepo) AvgActualMinutes(_ context.Context, _, _ time.Time) (float64, error) {
	return r.avgActual, nil
}
func (r *stubAnalyticsMetricsRepo) CategoryStats(_ context.Context, _, _ time.Time) ([]model.CategoryStats, error) {
	return r.categories, nil
}
func (r *stubAnalyticsMetricsRepo) MostMissedCategory(_ context.Context, _, _ time.Time) (*model.CategoryMissed, error) {
	return nil, nil
}
func (r *stubAnalyticsMetricsRepo) DailyCompletionCounts(_ context.Context, _, _ time.Time) ([]model.DailyCompletion, error) {
	return r.daily, nil
}
func (r *stubAnalyticsMetricsRepo) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return 0, 0, 0, nil
}

func newAnalyticsHarness(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	clk := &fixedReviewClock{t: now}

	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}

	metricsRepo := &stubAnalyticsMetricsRepo{
		breakdown: model.StatusBreakdown{Completed: 8, Pending: 2, Missed: 1},
		completed: model.Counts{N: 8, Total: 11},
		carry:     model.Counts{N: 2, Total: 10},
		avgActual: 42,
		daily: []model.DailyCompletion{
			{Date: now.AddDate(0, 0, -2), Count: 2},
			{Date: now.AddDate(0, 0, -1), Count: 3},
			{Date: now, Count: 3},
		},
		categories: []model.CategoryStats{
			{Category: model.CategoryJobApply, Total: 5, Completed: 4, CompletionRate: 0.8},
			{Category: model.CategoryNetworking, Total: 3, Completed: 2, CompletionRate: 0.67},
		},
	}
	metricsSvc := service.NewMetricsService(metricsRepo, clk)

	r := gin.New()
	h := web.NewAnalyticsHandler(rd, metricsSvc, clk, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.Register(r)
	return r
}

func TestAnalyticsPageRenders(t *testing.T) {
	t.Parallel()
	r := newAnalyticsHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/analytics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, needle := range []string{
		"Analytics",
		"Task completion trend",
		"Category ROI",
		"Weekly productivity",
		"Carry-over trend",
		"Overdue rate",
		"Average execution time",
		"Streak history",
		"chart-completion",
		"analytics-chart-config",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("expected page to contain %q", needle)
		}
	}
}

func TestAnalyticsKPIsPartial(t *testing.T) {
	t.Parallel()
	r := newAnalyticsHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/analytics/kpis?range=7", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Tasks completed") || !strings.Contains(body, "Completion rate") {
		t.Fatalf("unexpected KPI partial: %s", body)
	}
}

func TestAnalyticsRefreshPanels(t *testing.T) {
	t.Parallel()
	r := newAnalyticsHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/analytics/refresh?range=30", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("HX-Push-Url"); got != "/analytics?range=30" {
		t.Errorf("HX-Push-Url = %q, want /analytics?range=30", got)
	}
	if !strings.Contains(w.Body.String(), "analytics-charts-grid") {
		t.Error("refresh should return full panels partial")
	}
}

func TestAnalyticsChartRefresh(t *testing.T) {
	t.Parallel()
	r := newAnalyticsHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/analytics/charts/completion?range=7", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="card-chart-completion"`) {
		t.Error("expected chart card wrapper")
	}
	if !strings.Contains(body, "analytics-chart-config") {
		t.Error("expected chart config JSON block")
	}
}

func TestAnalyticsComparisonPartial(t *testing.T) {
	t.Parallel()
	r := newAnalyticsHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/analytics/comparison?range=7", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Trend comparison") {
		t.Error("expected comparison card")
	}
}
