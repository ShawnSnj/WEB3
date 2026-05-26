package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

type analyticsMetricsRepo struct {
	completionCounts map[string]model.Counts
	daily            []model.DailyCompletion
}

func (r *analyticsMetricsRepo) StatusBreakdown(_ context.Context, _, _ time.Time) (model.StatusBreakdown, error) {
	return model.StatusBreakdown{Completed: 3}, nil
}
func (r *analyticsMetricsRepo) StatusBreakdownDueBefore(_ context.Context, _ time.Time) (model.StatusBreakdown, error) {
	return model.StatusBreakdown{Completed: 3}, nil
}
func (r *analyticsMetricsRepo) CompletionCounts(_ context.Context, from, to time.Time) (model.Counts, error) {
	key := from.Format("2006-01-02") + "/" + to.Format("2006-01-02")
	if c, ok := r.completionCounts[key]; ok {
		return c, nil
	}
	return model.Counts{N: 2, Total: 4}, nil
}
func (r *analyticsMetricsRepo) CarryOverCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return model.Counts{N: 1, Total: 5}, nil
}
func (r *analyticsMetricsRepo) CarryOverCountsDueBefore(_ context.Context, _ time.Time) (model.Counts, error) {
	return model.Counts{N: 1, Total: 5}, nil
}
func (r *analyticsMetricsRepo) OverdueLive(_ context.Context, _ time.Time) (int, error) { return 0, nil }
func (r *analyticsMetricsRepo) AvgActualMinutes(_ context.Context, _, _ time.Time) (float64, error) {
	return 30, nil
}
func (r *analyticsMetricsRepo) CategoryStats(_ context.Context, _, _ time.Time) ([]model.CategoryStats, error) {
	return nil, nil
}
func (r *analyticsMetricsRepo) MostMissedCategory(_ context.Context, _, _ time.Time) (*model.CategoryMissed, error) {
	return nil, nil
}
func (r *analyticsMetricsRepo) DailyCompletionCounts(_ context.Context, _, _ time.Time) ([]model.DailyCompletion, error) {
	return r.daily, nil
}
func (r *analyticsMetricsRepo) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return 0, 0, 0, nil
}

type fixedAnalyticsClock struct{ t time.Time }

func (c fixedAnalyticsClock) Now() time.Time { return c.t }

func TestParseAnalyticsRange(t *testing.T) {
	t.Parallel()
	if got := service.ParseAnalyticsRange("30"); got != service.AnalyticsRange30 {
		t.Fatalf("got %q", got)
	}
	if got := service.ParseAnalyticsRange("bogus"); got != service.AnalyticsRange7 {
		t.Fatalf("default got %q", got)
	}
}

func TestRangeWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	svc := service.NewMetricsService(&analyticsMetricsRepo{}, fixedAnalyticsClock{t: now}, nil)
	from, to := svc.RangeWindow(service.AnalyticsRange7)
	if to.Sub(from) != 7*24*time.Hour {
		t.Fatalf("window = %v", to.Sub(from))
	}
	if from.Day() != 18 {
		t.Fatalf("from = %v", from)
	}
}

func TestTrendComparisonFor(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	repo := &analyticsMetricsRepo{
		completionCounts: map[string]model.Counts{
			"2026-05-18/2026-05-25": {N: 6, Total: 10},
			"2026-05-11/2026-05-18": {N: 4, Total: 10},
		},
	}
	svc := service.NewMetricsService(repo, fixedAnalyticsClock{t: now}, nil)
	from := now.AddDate(0, 0, -6)
	to := now.Add(24 * time.Hour)
	trend, err := svc.TrendComparisonFor(context.Background(), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if trend.CompletedDelta != 2 {
		t.Fatalf("delta = %d", trend.CompletedDelta)
	}
}

func TestStreakHistoryFillsGaps(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	repo := &analyticsMetricsRepo{
		daily: []model.DailyCompletion{{Date: now, Count: 2}},
	}
	svc := service.NewMetricsService(repo, fixedAnalyticsClock{t: now}, nil)
	from := now.AddDate(0, 0, -2)
	to := now.Add(24 * time.Hour)
	days, err := svc.StreakHistory(context.Background(), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 3 {
		t.Fatalf("len = %d", len(days))
	}
	if days[2].Count != 2 {
		t.Fatalf("last count = %d", days[2].Count)
	}
}
