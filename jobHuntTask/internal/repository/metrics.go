package repository

import (
	"context"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

// MetricsRepository exposes the aggregation queries used by the analytics
// service. Each method maps to a single, focused SQL statement.
type MetricsRepository interface {
	// StatusBreakdown returns counts grouped by status for tasks created in
	// [from, to).
	StatusBreakdown(ctx context.Context, from, to time.Time) (model.StatusBreakdown, error)

	// CompletionCounts returns (completed, total) for tasks created in
	// [from, to). Equivalent to StatusBreakdown but cheaper when only the
	// rate is needed.
	CompletionCounts(ctx context.Context, from, to time.Time) (model.Counts, error)

	// CarryOverCounts returns (carried_over, total) for tasks created in
	// [from, to). "Carried-over" means carry_over_count > 0.
	CarryOverCounts(ctx context.Context, from, to time.Time) (model.Counts, error)

	// OverdueLive returns the current count of non-terminal, overdue tasks
	// regardless of date range — it's a snapshot of "now".
	OverdueLive(ctx context.Context, now time.Time) (int, error)

	// AvgActualMinutes returns the average actual_minutes across completed
	// tasks finished in [from, to). actual_minutes <= 0 rows are excluded.
	AvgActualMinutes(ctx context.Context, from, to time.Time) (float64, error)

	// CategoryStats returns one row per category with full breakdown for
	// tasks created in [from, to).
	CategoryStats(ctx context.Context, from, to time.Time) ([]model.CategoryStats, error)

	// MostMissedCategory returns the category with the highest missed-count
	// in [from, to), or nil if there are no missed tasks.
	MostMissedCategory(ctx context.Context, from, to time.Time) (*model.CategoryMissed, error)

	// DailyCompletionCounts returns one row per calendar day in [from, to]
	// (inclusive) where status='completed' completed_at falls in that day.
	// Days with zero completions are NOT included — the service fills gaps.
	DailyCompletionCounts(ctx context.Context, from, to time.Time) ([]model.DailyCompletion, error)

	// EffortDistribution returns the mean estimated_minutes and the count
	// of tasks whose estimate >= largeMinutes for tasks CREATED in [from,
	// to). Used by the suggestion engine's "smaller_tasks" rule. Only
	// tasks with estimated_minutes > 0 contribute to the average; the
	// total is the count of those same tasks.
	EffortDistribution(ctx context.Context, from, to time.Time, largeMinutes int) (avg float64, large, total int, err error)
}
