package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Fake MetricsRepository
// ---------------------------------------------------------------------------

type fakeMetricsRepo struct {
	statusBreakdown    map[string]model.StatusBreakdown
	completionCounts   map[string]model.Counts
	carryOverCounts    map[string]model.Counts
	overdueLive        int
	avgActualMinutes   map[string]float64
	categoryStats      map[string][]model.CategoryStats
	mostMissed         map[string]*model.CategoryMissed
	dailyCompletions   map[string][]model.DailyCompletion

	statusCalls    int
	completionCalls int
}

func key(from, to time.Time) string {
	return from.UTC().Format("20060102") + ":" + to.UTC().Format("20060102")
}

func (r *fakeMetricsRepo) StatusBreakdown(_ context.Context, from, to time.Time) (model.StatusBreakdown, error) {
	r.statusCalls++
	return r.statusBreakdown[key(from, to)], nil
}
func (r *fakeMetricsRepo) CompletionCounts(_ context.Context, from, to time.Time) (model.Counts, error) {
	r.completionCalls++
	return r.completionCounts[key(from, to)], nil
}
func (r *fakeMetricsRepo) CarryOverCounts(_ context.Context, from, to time.Time) (model.Counts, error) {
	return r.carryOverCounts[key(from, to)], nil
}
func (r *fakeMetricsRepo) OverdueLive(_ context.Context, _ time.Time) (int, error) {
	return r.overdueLive, nil
}
func (r *fakeMetricsRepo) AvgActualMinutes(_ context.Context, from, to time.Time) (float64, error) {
	return r.avgActualMinutes[key(from, to)], nil
}
func (r *fakeMetricsRepo) CategoryStats(_ context.Context, from, to time.Time) ([]model.CategoryStats, error) {
	return r.categoryStats[key(from, to)], nil
}
func (r *fakeMetricsRepo) MostMissedCategory(_ context.Context, from, to time.Time) (*model.CategoryMissed, error) {
	return r.mostMissed[key(from, to)], nil
}
func (r *fakeMetricsRepo) DailyCompletionCounts(_ context.Context, from, to time.Time) ([]model.DailyCompletion, error) {
	return r.dailyCompletions[key(from, to)], nil
}
func (r *fakeMetricsRepo) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return 0, 0, 0, nil
}

func newFakeMetricsRepo() *fakeMetricsRepo {
	return &fakeMetricsRepo{
		statusBreakdown:  map[string]model.StatusBreakdown{},
		completionCounts: map[string]model.Counts{},
		carryOverCounts:  map[string]model.Counts{},
		avgActualMinutes: map[string]float64{},
		categoryStats:    map[string][]model.CategoryStats{},
		mostMissed:       map[string]*model.CategoryMissed{},
		dailyCompletions: map[string][]model.DailyCompletion{},
	}
}

// ---------------------------------------------------------------------------
// Helpers — replicate the service's window math to seed the fake repo
// ---------------------------------------------------------------------------

func startOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func windows(now time.Time) (today, tomorrow, weekFrom, weekTo, prevFrom, prevTo time.Time) {
	today = startOfDay(now)
	tomorrow = today.Add(24 * time.Hour)
	weekFrom = today.AddDate(0, 0, -6)
	weekTo = tomorrow
	prevFrom = weekFrom.AddDate(0, 0, -7)
	prevTo = weekTo.AddDate(0, 0, -7)
	return
}

// ---------------------------------------------------------------------------
// Today
// ---------------------------------------------------------------------------

func TestMetrics_Today(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 14, 0, 0, 0, time.UTC)}
	repo := newFakeMetricsRepo()
	today, tomorrow, _, _, _, _ := windows(clk.t)

	repo.statusBreakdown[key(today, tomorrow)] = model.StatusBreakdown{
		Pending: 2, InProgress: 1, Completed: 3, Missed: 0,
	}
	repo.avgActualMinutes[key(today, tomorrow)] = 27.5
	repo.overdueLive = 4

	svc := service.NewMetricsService(repo, clk)
	out, err := svc.Today(context.Background())
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if out.CompletedTotal != 3 {
		t.Errorf("completed total = %d, want 3", out.CompletedTotal)
	}
	if got := out.CompletionRate; got <= 0.49 || got >= 0.51 { // 3/6
		t.Errorf("completion rate = %v, want ~0.5", got)
	}
	if out.OverdueCount != 4 {
		t.Errorf("overdue = %d, want 4", out.OverdueCount)
	}
	if out.AvgActualMinutes != 27.5 {
		t.Errorf("avg = %v, want 27.5", out.AvgActualMinutes)
	}
}

// ---------------------------------------------------------------------------
// Weekly + gap-filling
// ---------------------------------------------------------------------------

func TestMetrics_WeeklyFillsGaps(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	repo := newFakeMetricsRepo()
	_, _, weekFrom, weekTo, _, _ := windows(clk.t)

	repo.statusBreakdown[key(weekFrom, weekTo)] = model.StatusBreakdown{
		Pending: 1, Completed: 6, Missed: 3,
	}
	repo.carryOverCounts[key(weekFrom, weekTo)] = model.Counts{N: 2, Total: 10}

	// only 2 days have completions
	repo.dailyCompletions[key(weekFrom, weekTo)] = []model.DailyCompletion{
		{Date: weekFrom.AddDate(0, 0, 2), Count: 3},
		{Date: weekFrom.AddDate(0, 0, 5), Count: 3},
	}

	svc := service.NewMetricsService(repo, clk)
	w, err := svc.Weekly(context.Background())
	if err != nil {
		t.Fatalf("Weekly: %v", err)
	}
	if len(w.DailyCompletions) != 7 {
		t.Fatalf("daily = %d, want 7", len(w.DailyCompletions))
	}
	// day 2 should be 3, day 0 should be 0
	if w.DailyCompletions[0].Count != 0 {
		t.Errorf("day 0 = %d, want 0", w.DailyCompletions[0].Count)
	}
	if w.DailyCompletions[2].Count != 3 {
		t.Errorf("day 2 = %d, want 3", w.DailyCompletions[2].Count)
	}
	if w.CarryOverRate != 0.2 {
		t.Errorf("carry rate = %v, want 0.2", w.CarryOverRate)
	}
	// overdue_rate = missed/total = 3/10
	if w.OverdueRate < 0.29 || w.OverdueRate > 0.31 {
		t.Errorf("overdue rate = %v, want ~0.3", w.OverdueRate)
	}
}

// ---------------------------------------------------------------------------
// Trend
// ---------------------------------------------------------------------------

func TestMetrics_Trend(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	repo := newFakeMetricsRepo()
	_, _, weekFrom, weekTo, prevFrom, prevTo := windows(clk.t)

	repo.completionCounts[key(weekFrom, weekTo)] = model.Counts{N: 8, Total: 10}
	repo.completionCounts[key(prevFrom, prevTo)] = model.Counts{N: 4, Total: 10}

	svc := service.NewMetricsService(repo, clk)
	tr, err := svc.TrendComparison(context.Background())
	if err != nil {
		t.Fatalf("Trend: %v", err)
	}
	if tr.CompletedNow != 8 || tr.CompletedPrev != 4 {
		t.Errorf("counts wrong: now=%d prev=%d", tr.CompletedNow, tr.CompletedPrev)
	}
	if tr.CompletedDelta != 4 {
		t.Errorf("delta = %d, want 4", tr.CompletedDelta)
	}
	if got := tr.CompletionRateDelta; got < 0.39 || got > 0.41 {
		t.Errorf("rate delta = %v, want ~0.4", got)
	}
}

// ---------------------------------------------------------------------------
// Streak: standard, grace-day, and broken
// ---------------------------------------------------------------------------

func TestMetrics_Streak_TodayCounted(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 14, 0, 0, 0, time.UTC)}
	today := startOfDay(clk.t)
	repo := newFakeMetricsRepo()

	from := today.AddDate(0, 0, -365)
	to := today.Add(24 * time.Hour)
	// 4-day streak ending today, preceded by a gap and an earlier 2-day run.
	repo.dailyCompletions[key(from, to)] = []model.DailyCompletion{
		{Date: today.AddDate(0, 0, -10), Count: 1},
		{Date: today.AddDate(0, 0, -9), Count: 1},
		{Date: today.AddDate(0, 0, -3), Count: 1},
		{Date: today.AddDate(0, 0, -2), Count: 2},
		{Date: today.AddDate(0, 0, -1), Count: 1},
		{Date: today, Count: 1},
	}

	svc := service.NewMetricsService(repo, clk)
	s, err := svc.Streak(context.Background())
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	if s.CurrentStreak != 4 {
		t.Errorf("current streak = %d, want 4", s.CurrentStreak)
	}
	if s.LongestStreak != 4 {
		t.Errorf("longest streak = %d, want 4", s.LongestStreak)
	}
	if s.TodayCompletedCount != 1 {
		t.Errorf("today = %d, want 1", s.TodayCompletedCount)
	}
	// Last 7 days excluding today: [-7,-6,-5,-4,-3,-2,-1]
	// completions: -3,-2,-1  → missed = 4 days (-7,-6,-5,-4)
	if s.MissedDayCount != 4 {
		t.Errorf("missed = %d, want 4", s.MissedDayCount)
	}
}

func TestMetrics_Streak_LongestExceedsCurrent(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	today := startOfDay(clk.t)
	repo := newFakeMetricsRepo()
	from := today.AddDate(0, 0, -365)
	to := today.Add(24 * time.Hour)
	// 6-day historical run, then a gap, then a 2-day current streak.
	repo.dailyCompletions[key(from, to)] = []model.DailyCompletion{
		{Date: today.AddDate(0, 0, -20), Count: 1},
		{Date: today.AddDate(0, 0, -19), Count: 1},
		{Date: today.AddDate(0, 0, -18), Count: 1},
		{Date: today.AddDate(0, 0, -17), Count: 1},
		{Date: today.AddDate(0, 0, -16), Count: 1},
		{Date: today.AddDate(0, 0, -15), Count: 1},
		{Date: today.AddDate(0, 0, -1), Count: 1},
		{Date: today, Count: 1},
	}
	svc := service.NewMetricsService(repo, clk)
	s, _ := svc.Streak(context.Background())
	if s.CurrentStreak != 2 {
		t.Errorf("current = %d, want 2", s.CurrentStreak)
	}
	if s.LongestStreak != 6 {
		t.Errorf("longest = %d, want 6", s.LongestStreak)
	}
}

func TestMetrics_Streak_TodayEmptyGraceDay(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	today := startOfDay(clk.t)
	repo := newFakeMetricsRepo()

	from := today.AddDate(0, 0, -365)
	to := today.Add(24 * time.Hour)
	// today empty; 3-day streak through yesterday
	repo.dailyCompletions[key(from, to)] = []model.DailyCompletion{
		{Date: today.AddDate(0, 0, -3), Count: 1},
		{Date: today.AddDate(0, 0, -2), Count: 1},
		{Date: today.AddDate(0, 0, -1), Count: 2},
	}

	svc := service.NewMetricsService(repo, clk)
	s, err := svc.Streak(context.Background())
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	if s.CurrentStreak != 3 {
		t.Errorf("streak = %d, want 3 (grace day)", s.CurrentStreak)
	}
	if s.TodayCompletedCount != 0 {
		t.Errorf("today = %d, want 0", s.TodayCompletedCount)
	}
}

func TestMetrics_Streak_Broken(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	today := startOfDay(clk.t)
	repo := newFakeMetricsRepo()
	from := today.AddDate(0, 0, -365)
	to := today.Add(24 * time.Hour)
	// today empty AND yesterday empty -> streak 0
	repo.dailyCompletions[key(from, to)] = []model.DailyCompletion{
		{Date: today.AddDate(0, 0, -3), Count: 1},
	}
	svc := service.NewMetricsService(repo, clk)
	s, _ := svc.Streak(context.Background())
	if s.CurrentStreak != 0 {
		t.Errorf("streak = %d, want 0", s.CurrentStreak)
	}
}

// ---------------------------------------------------------------------------
// Dashboard end-to-end (with fake repo)
// ---------------------------------------------------------------------------

func TestMetrics_Dashboard(t *testing.T) {
	t.Parallel()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	today, tomorrow, weekFrom, weekTo, prevFrom, prevTo := windows(clk.t)
	repo := newFakeMetricsRepo()

	repo.statusBreakdown[key(today, tomorrow)] = model.StatusBreakdown{Completed: 2, Pending: 1}
	repo.statusBreakdown[key(weekFrom, weekTo)] = model.StatusBreakdown{Completed: 5, Missed: 2}
	repo.carryOverCounts[key(weekFrom, weekTo)] = model.Counts{N: 1, Total: 7}
	repo.completionCounts[key(weekFrom, weekTo)] = model.Counts{N: 5, Total: 7}
	repo.completionCounts[key(prevFrom, prevTo)] = model.Counts{N: 3, Total: 7}
	repo.mostMissed[key(weekFrom, weekTo)] = &model.CategoryMissed{
		Category: model.CategoryGithub, Count: 2,
	}
	from := today.AddDate(0, 0, -365)
	to := today.Add(24 * time.Hour)
	repo.dailyCompletions[key(from, to)] = []model.DailyCompletion{
		{Date: today, Count: 2},
	}

	svc := service.NewMetricsService(repo, clk)
	d, err := svc.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if d.Today.CompletedTotal != 2 {
		t.Errorf("today completed = %d, want 2", d.Today.CompletedTotal)
	}
	if d.Trend.CompletedDelta != 2 {
		t.Errorf("trend delta = %d, want 2", d.Trend.CompletedDelta)
	}
	if d.MostMissedCategory == nil || d.MostMissedCategory.Category != model.CategoryGithub {
		t.Errorf("most missed wrong: %+v", d.MostMissedCategory)
	}
	if d.Streak.CurrentStreak != 1 {
		t.Errorf("streak = %d, want 1", d.Streak.CurrentStreak)
	}
}
