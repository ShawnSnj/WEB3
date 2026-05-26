package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// MetricsService composes individual aggregation queries from the repo into
// the higher-level dashboard views. It owns the time-window semantics:
//   - "today" counts tasks due today or earlier (due_date < start-of-tomorrow),
//     matching /tasks?view=today
//   - "weekly" is a rolling 7-day window ending today (inclusive of today)
//   - "trend" compares the rolling 7-day window to the prior 7-day window
//
// Streak computation is done in Go from a daily-count map so the SQL stays
// portable (no calendar-arithmetic dialect quirks).
type MetricsService struct {
	repo  repository.MetricsRepository
	clock Clock
	cal   *calendar.Calendar
}

// NewMetricsService wires the service to a repository. When clock is nil,
// a real wall-clock is used. When cal is nil, UTC calendar-day boundaries are used.
func NewMetricsService(repo repository.MetricsRepository, clock Clock, cal *calendar.Calendar) *MetricsService {
	if clock == nil {
		clock = realClock{}
	}
	if cal == nil {
		cal = calendar.UTC()
	}
	return &MetricsService{repo: repo, clock: clock, cal: cal}
}

// ---------------------------------------------------------------------------
// Window helpers
// ---------------------------------------------------------------------------

func (s *MetricsService) startOfDay(t time.Time) time.Time {
	return s.cal.StartOfDay(t)
}

// todayWindow returns [start-of-today, start-of-tomorrow) in APP_TIMEZONE.
func (s *MetricsService) todayWindow() (time.Time, time.Time) {
	from := s.startOfDay(s.clock.Now())
	return from, from.Add(24 * time.Hour)
}

// weeklyWindow returns the rolling 7-day window ending now: the lower bound
// is start-of-day 6 days ago; the upper bound is start-of-tomorrow.
func (s *MetricsService) weeklyWindow() (time.Time, time.Time) {
	today := s.startOfDay(s.clock.Now())
	return today.AddDate(0, 0, -6), today.Add(24 * time.Hour)
}

// previousWeeklyWindow is the 7-day window immediately preceding weeklyWindow.
func (s *MetricsService) previousWeeklyWindow() (time.Time, time.Time) {
	from, to := s.weeklyWindow()
	return from.AddDate(0, 0, -7), to.AddDate(0, 0, -7)
}

// ---------------------------------------------------------------------------
// Today
// ---------------------------------------------------------------------------

// TodayCarryOver returns the count of carry-over tasks (carry_over_count > 0)
// created today. Used by the dashboard summary card.
func (s *MetricsService) TodayCarryOver(ctx context.Context) (int, error) {
	_, to := s.todayWindow()
	counts, err := s.repo.CarryOverCountsDueBefore(ctx, to)
	if err != nil {
		return 0, err
	}
	return counts.N, nil
}

// Today returns the dashboard's "today" panel.
func (s *MetricsService) Today(ctx context.Context) (model.DailyStats, error) {
	from, to := s.todayWindow()
	now := s.clock.Now()

	// Due-date semantics match /tasks?view=today (due today or earlier).
	breakdown, err := s.repo.StatusBreakdownDueBefore(ctx, to)
	if err != nil {
		return model.DailyStats{}, fmt.Errorf("today breakdown: %w", err)
	}
	overdue, err := s.repo.OverdueLive(ctx, now)
	if err != nil {
		return model.DailyStats{}, fmt.Errorf("today overdue: %w", err)
	}
	avg, err := s.repo.AvgActualMinutes(ctx, from, to)
	if err != nil {
		return model.DailyStats{}, fmt.Errorf("today avg minutes: %w", err)
	}

	total := breakdown.Total()
	var rate float64
	if total > 0 {
		rate = float64(breakdown.Completed) / float64(total)
	}

	return model.DailyStats{
		Date:             from,
		Breakdown:        breakdown,
		CompletionRate:   rate,
		OverdueCount:     overdue,
		AvgActualMinutes: avg,
		CompletedTotal:   breakdown.Completed,
	}, nil
}

// ---------------------------------------------------------------------------
// Weekly
// ---------------------------------------------------------------------------

// Weekly returns the dashboard's "weekly" panel covering the rolling 7-day
// window ending today (inclusive). DailyCompletions is back-filled with zero
// for days that had no completions so the UI can render a uniform sparkline.
func (s *MetricsService) Weekly(ctx context.Context) (model.WeeklyStats, error) {
	from, to := s.weeklyWindow()
	return s.weeklyForRange(ctx, from, to)
}

// WeeklyFor returns stats for a custom 7-day window starting at `from`
// (normalised to UTC midnight). Used by the weekly review page when browsing
// past windows.
func (s *MetricsService) WeeklyFor(ctx context.Context, from time.Time) (model.WeeklyStats, error) {
	from = s.startOfDay(from)
	to := from.AddDate(0, 0, 7)
	return s.weeklyForRange(ctx, from, to)
}

// CarryOverRateFor returns the carry-over rate for [from, from+7d).
func (s *MetricsService) CarryOverRateFor(ctx context.Context, from time.Time) (float64, int, error) {
	from = s.startOfDay(from)
	to := from.AddDate(0, 0, 7)
	counts, err := s.repo.CarryOverCounts(ctx, from, to)
	if err != nil {
		return 0, 0, err
	}
	return counts.Rate(), counts.N, nil
}

func (s *MetricsService) weeklyForRange(ctx context.Context, from, to time.Time) (model.WeeklyStats, error) {
	breakdown, err := s.repo.StatusBreakdown(ctx, from, to)
	if err != nil {
		return model.WeeklyStats{}, fmt.Errorf("weekly breakdown: %w", err)
	}
	carry, err := s.repo.CarryOverCounts(ctx, from, to)
	if err != nil {
		return model.WeeklyStats{}, fmt.Errorf("weekly carry: %w", err)
	}
	avg, err := s.repo.AvgActualMinutes(ctx, from, to)
	if err != nil {
		return model.WeeklyStats{}, fmt.Errorf("weekly avg minutes: %w", err)
	}
	daily, err := s.repo.DailyCompletionCounts(ctx, from, to)
	if err != nil {
		return model.WeeklyStats{}, fmt.Errorf("weekly daily: %w", err)
	}

	total := breakdown.Total()
	var compRate float64
	if total > 0 {
		compRate = float64(breakdown.Completed) / float64(total)
	}
	// Overdue rate in a window is: (created-and-missed) / total.
	// "Missed" is the post-due terminal status, so it's a fair proxy.
	var overdueRate float64
	if total > 0 {
		overdueRate = float64(breakdown.Missed) / float64(total)
	}

	return model.WeeklyStats{
		From:             from,
		To:               to,
		Breakdown:        breakdown,
		CompletionRate:   compRate,
		CarryOverRate:    carry.Rate(),
		OverdueRate:      overdueRate,
		AvgActualMinutes: avg,
		DailyCompletions: s.fillDailyGaps(daily, from, to),
	}, nil
}

// fillDailyGaps takes a sparse list of (day, count) and emits a dense list
// with one entry per day in [from, to).
func (s *MetricsService) fillDailyGaps(sparse []model.DailyCompletion, from, to time.Time) []model.DailyCompletion {
	from = s.startOfDay(from)
	to = s.startOfDay(to)
	byDay := make(map[time.Time]int, len(sparse))
	for _, d := range sparse {
		byDay[s.startOfDay(d.Date)] = d.Count
	}
	out := make([]model.DailyCompletion, 0, 7)
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		out = append(out, model.DailyCompletion{Date: d, Count: byDay[d]})
	}
	return out
}

// ---------------------------------------------------------------------------
// Trend
// ---------------------------------------------------------------------------

// TrendComparison returns the rolling-7 vs. previous-7 comparison.
func (s *MetricsService) TrendComparison(ctx context.Context) (model.Trend, error) {
	curFrom, curTo := s.weeklyWindow()
	prevFrom, prevTo := s.previousWeeklyWindow()

	cur, err := s.repo.CompletionCounts(ctx, curFrom, curTo)
	if err != nil {
		return model.Trend{}, fmt.Errorf("trend cur: %w", err)
	}
	prev, err := s.repo.CompletionCounts(ctx, prevFrom, prevTo)
	if err != nil {
		return model.Trend{}, fmt.Errorf("trend prev: %w", err)
	}

	return model.Trend{
		CurrentFrom:         curFrom,
		CurrentTo:           curTo,
		PreviousFrom:        prevFrom,
		PreviousTo:          prevTo,
		CompletionRateNow:   cur.Rate(),
		CompletionRatePrev:  prev.Rate(),
		CompletionRateDelta: cur.Rate() - prev.Rate(),
		CompletedNow:        cur.N,
		CompletedPrev:       prev.N,
		CompletedDelta:      cur.N - prev.N,
	}, nil
}

// ---------------------------------------------------------------------------
// Streak
// ---------------------------------------------------------------------------

// streakLookback bounds the streak computation to a sensible window. A
// 365-day cap is generous and keeps the query bounded.
const streakLookback = 365

// Streak returns the current consecutive-day completion streak. Today is
// counted if it has completions; otherwise the streak is allowed to extend
// from yesterday (a one-day grace so a check at 9 AM doesn't read "0").
func (s *MetricsService) Streak(ctx context.Context) (model.Streak, error) {
	today := s.startOfDay(s.clock.Now())
	from := today.AddDate(0, 0, -streakLookback)
	to := today.Add(24 * time.Hour)

	counts, err := s.repo.DailyCompletionCounts(ctx, from, to)
	if err != nil {
		return model.Streak{}, fmt.Errorf("streak counts: %w", err)
	}

	byDay := make(map[time.Time]int, len(counts))
	var lastCompletion time.Time
	for _, c := range counts {
		day := s.startOfDay(c.Date)
		byDay[day] = c.Count
		if c.Count > 0 && day.After(lastCompletion) {
			lastCompletion = day
		}
	}

	todayN := byDay[today]
	cursor := today
	if todayN == 0 {
		// grace day — start counting from yesterday
		cursor = today.AddDate(0, 0, -1)
	}

	streak := 0
	for byDay[cursor] > 0 {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
		if cursor.Before(from) {
			break
		}
	}

	// Longest streak across the whole lookback window. Walk day by day
	// from `from` to `today` (inclusive) so adjacency reflects calendar
	// days, not the sparse map order.
	longest := 0
	run := 0
	for d := from; !d.After(today); d = d.AddDate(0, 0, 1) {
		if byDay[d] > 0 {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}

	// Missed days = count of zero-completion days in the last 7 days
	// EXCLUDING today (today is still in progress). Iterate the 7 calendar
	// days ending yesterday.
	missed := 0
	for i := 1; i <= 7; i++ {
		d := today.AddDate(0, 0, -i)
		if byDay[d] == 0 {
			missed++
		}
	}

	return model.Streak{
		CurrentStreak:       streak,
		LongestStreak:       longest,
		MissedDayCount:      missed,
		TodayCompletedCount: todayN,
		LastCompletionDate:  lastCompletion,
	}, nil
}

// ---------------------------------------------------------------------------
// Category ROI + most-missed
// ---------------------------------------------------------------------------

// Categories returns per-category stats for the given window. If from/to are
// zero values, the rolling weekly window is used.
func (s *MetricsService) Categories(ctx context.Context, from, to time.Time) ([]model.CategoryStats, error) {
	if from.IsZero() || to.IsZero() {
		from, to = s.weeklyWindow()
	}
	rows, err := s.repo.CategoryStats(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("categories: %w", err)
	}
	return rows, nil
}

// MostMissed returns the worst-offender category within the rolling weekly
// window (by updated_at, since that's when status transitioned to missed).
func (s *MetricsService) MostMissed(ctx context.Context) (*model.CategoryMissed, error) {
	from, to := s.weeklyWindow()
	return s.repo.MostMissedCategory(ctx, from, to)
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

// Dashboard returns the full summary used by the top-level dashboard view.
// It performs the underlying aggregations sequentially; each call is short
// (single aggregate) so even ~8 round-trips is bounded.
func (s *MetricsService) Dashboard(ctx context.Context) (model.DashboardSummary, error) {
	today, err := s.Today(ctx)
	if err != nil {
		return model.DashboardSummary{}, err
	}
	weekly, err := s.Weekly(ctx)
	if err != nil {
		return model.DashboardSummary{}, err
	}
	trend, err := s.TrendComparison(ctx)
	if err != nil {
		return model.DashboardSummary{}, err
	}
	streak, err := s.Streak(ctx)
	if err != nil {
		return model.DashboardSummary{}, err
	}
	mm, err := s.MostMissed(ctx)
	if err != nil {
		return model.DashboardSummary{}, err
	}
	return model.DashboardSummary{
		Today:              today,
		Weekly:             weekly,
		Trend:              trend,
		Streak:             streak,
		MostMissedCategory: mm,
	}, nil
}
