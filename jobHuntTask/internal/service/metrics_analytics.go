package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

// AnalyticsRange is a supported analytics time window (days).
type AnalyticsRange string

const (
	AnalyticsRange7  AnalyticsRange = "7"
	AnalyticsRange30 AnalyticsRange = "30"
	AnalyticsRange90 AnalyticsRange = "90"
)

// ParseAnalyticsRange normalises a query param to a supported range.
func ParseAnalyticsRange(s string) AnalyticsRange {
	switch AnalyticsRange(s) {
	case AnalyticsRange30, AnalyticsRange90:
		return AnalyticsRange(s)
	default:
		return AnalyticsRange7
	}
}

func (r AnalyticsRange) Days() int {
	switch r {
	case AnalyticsRange90:
		return 90
	case AnalyticsRange30:
		return 30
	default:
		return 7
	}
}

func (r AnalyticsRange) Label() string {
	switch r {
	case AnalyticsRange90:
		return "Last 90 days"
	case AnalyticsRange30:
		return "Last 30 days"
	default:
		return "Last 7 days"
	}
}

// RangeWindow returns [from, to) for the given analytics range ending today.
func (s *MetricsService) RangeWindow(r AnalyticsRange) (time.Time, time.Time) {
	today := startOfDayUTC(s.clock.Now())
	to := today.Add(24 * time.Hour)
	from := today.AddDate(0, 0, -(r.Days() - 1))
	return from, to
}

// PeriodStats returns aggregated stats for an arbitrary [from, to) window.
func (s *MetricsService) PeriodStats(ctx context.Context, from, to time.Time) (model.WeeklyStats, error) {
	from = startOfDayUTC(from)
	to = startOfDayUTC(to)
	return s.weeklyForRange(ctx, from, to)
}

// TrendComparisonFor compares [from, to) against the immediately preceding
// period of equal length.
func (s *MetricsService) TrendComparisonFor(ctx context.Context, from, to time.Time) (model.Trend, error) {
	from = startOfDayUTC(from)
	to = startOfDayUTC(to)
	duration := to.Sub(from)
	prevFrom := from.Add(-duration)
	prevTo := from

	cur, err := s.repo.CompletionCounts(ctx, from, to)
	if err != nil {
		return model.Trend{}, fmt.Errorf("trend cur: %w", err)
	}
	prev, err := s.repo.CompletionCounts(ctx, prevFrom, prevTo)
	if err != nil {
		return model.Trend{}, fmt.Errorf("trend prev: %w", err)
	}

	return model.Trend{
		CurrentFrom:         from,
		CurrentTo:           to,
		PreviousFrom:        prevFrom,
		PreviousTo:          prevTo,
		CompletionRateNow:   cur.Rate(),
		CompletionRatePrev:  prev.Rate(),
		CompletionRateDelta:   cur.Rate() - prev.Rate(),
		CompletedNow:        cur.N,
		CompletedPrev:       prev.N,
		CompletedDelta:      cur.N - prev.N,
	}, nil
}

// WeeklyBucket is one slice in a multi-week trend chart.
type WeeklyBucket struct {
	Label         string
	From          time.Time
	To            time.Time
	Completed     int
	CarryOverRate float64
	OverdueRate   float64
	AvgMinutes    float64
}

// WeeklyBuckets splits [from, to) into consecutive 7-day windows and
// aggregates metrics for each bucket.
func (s *MetricsService) WeeklyBuckets(ctx context.Context, from, to time.Time) ([]WeeklyBucket, error) {
	from = startOfDayUTC(from)
	to = startOfDayUTC(to)
	var out []WeeklyBucket
	for cur := from; cur.Before(to); cur = cur.AddDate(0, 0, 7) {
		end := cur.AddDate(0, 0, 7)
		if end.After(to) {
			end = to
		}
		stats, err := s.weeklyForRange(ctx, cur, end)
		if err != nil {
			return nil, err
		}
		label := cur.Format("Jan 2")
		if end.Sub(cur) > 24*time.Hour {
			label = cur.Format("Jan 2") + "–" + end.AddDate(0, 0, -1).Format("Jan 2")
		}
		out = append(out, WeeklyBucket{
			Label:         label,
			From:          cur,
			To:            end,
			Completed:     stats.Breakdown.Completed,
			CarryOverRate: stats.CarryOverRate,
			OverdueRate:   stats.OverdueRate,
			AvgMinutes:    stats.AvgActualMinutes,
		})
	}
	return out, nil
}

// StreakHistory returns daily completion counts for [from, to) with gaps
// filled — used by the analytics streak heatmap / bar chart.
func (s *MetricsService) StreakHistory(ctx context.Context, from, to time.Time) ([]model.DailyCompletion, error) {
	from = startOfDayUTC(from)
	to = startOfDayUTC(to)
	sparse, err := s.repo.DailyCompletionCounts(ctx, from, to)
	if err != nil {
		return nil, err
	}
	return fillDailyGaps(sparse, from, to), nil
}
