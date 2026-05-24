package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shawn/jobhunttask/internal/model"
)

type PostgresMetricsRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresMetricsRepository(pool *pgxpool.Pool) *PostgresMetricsRepository {
	return &PostgresMetricsRepository{pool: pool}
}

var _ MetricsRepository = (*PostgresMetricsRepository)(nil)

// ---------------------------------------------------------------------------
// StatusBreakdown
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) StatusBreakdown(ctx context.Context, from, to time.Time) (model.StatusBreakdown, error) {
	const q = `
        SELECT
            COUNT(*) FILTER (WHERE status = 'pending')     AS pending,
            COUNT(*) FILTER (WHERE status = 'in_progress') AS in_progress,
            COUNT(*) FILTER (WHERE status = 'completed')   AS completed,
            COUNT(*) FILTER (WHERE status = 'missed')      AS missed
        FROM tasks
        WHERE created_at >= $1 AND created_at < $2
    `
	var b model.StatusBreakdown
	err := r.pool.QueryRow(ctx, q, from, to).Scan(
		&b.Pending, &b.InProgress, &b.Completed, &b.Missed,
	)
	if err != nil {
		return model.StatusBreakdown{}, fmt.Errorf("status breakdown: %w", err)
	}
	return b, nil
}

// ---------------------------------------------------------------------------
// CompletionCounts
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) CompletionCounts(ctx context.Context, from, to time.Time) (model.Counts, error) {
	const q = `
        SELECT
            COUNT(*) FILTER (WHERE status = 'completed') AS completed,
            COUNT(*) AS total
        FROM tasks
        WHERE created_at >= $1 AND created_at < $2
    `
	var c model.Counts
	if err := r.pool.QueryRow(ctx, q, from, to).Scan(&c.N, &c.Total); err != nil {
		return model.Counts{}, fmt.Errorf("completion counts: %w", err)
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// CarryOverCounts
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) CarryOverCounts(ctx context.Context, from, to time.Time) (model.Counts, error) {
	const q = `
        SELECT
            COUNT(*) FILTER (WHERE carry_over_count > 0) AS carried,
            COUNT(*) AS total
        FROM tasks
        WHERE created_at >= $1 AND created_at < $2
    `
	var c model.Counts
	if err := r.pool.QueryRow(ctx, q, from, to).Scan(&c.N, &c.Total); err != nil {
		return model.Counts{}, fmt.Errorf("carry-over counts: %w", err)
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// OverdueLive
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) OverdueLive(ctx context.Context, now time.Time) (int, error) {
	const q = `
        SELECT COUNT(*) FROM tasks
        WHERE status IN ('pending','in_progress')
          AND due_date IS NOT NULL
          AND due_date < $1
    `
	var n int
	if err := r.pool.QueryRow(ctx, q, now).Scan(&n); err != nil {
		return 0, fmt.Errorf("overdue live: %w", err)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// AvgActualMinutes
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) AvgActualMinutes(ctx context.Context, from, to time.Time) (float64, error) {
	const q = `
        SELECT COALESCE(AVG(actual_minutes)::float, 0)
        FROM tasks
        WHERE status = 'completed'
          AND completed_at >= $1 AND completed_at < $2
          AND actual_minutes > 0
    `
	var avg float64
	if err := r.pool.QueryRow(ctx, q, from, to).Scan(&avg); err != nil {
		return 0, fmt.Errorf("avg actual minutes: %w", err)
	}
	return avg, nil
}

// ---------------------------------------------------------------------------
// CategoryStats
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) CategoryStats(ctx context.Context, from, to time.Time) ([]model.CategoryStats, error) {
	const q = `
        SELECT
            category,
            COUNT(*)                                                            AS total,
            COUNT(*) FILTER (WHERE status = 'completed')                        AS completed,
            COUNT(*) FILTER (WHERE status = 'missed')                           AS missed,
            COALESCE(AVG(actual_minutes)
                     FILTER (WHERE status = 'completed' AND actual_minutes > 0), 0)::float
                                                                                AS avg_actual,
            COALESCE(AVG(estimated_minutes) FILTER (WHERE estimated_minutes > 0), 0)::float
                                                                                AS avg_estimated,
            COALESCE(SUM(actual_minutes)
                     FILTER (WHERE status = 'completed' AND actual_minutes > 0), 0)
                                                                                AS total_actual
        FROM tasks
        WHERE created_at >= $1 AND created_at < $2
        GROUP BY category
        ORDER BY total DESC, category ASC
    `
	rows, err := r.pool.Query(ctx, q, from, to)
	if err != nil {
		return nil, fmt.Errorf("category stats: %w", err)
	}
	defer rows.Close()

	out := make([]model.CategoryStats, 0, 8)
	for rows.Next() {
		var (
			cat            string
			total, done, missed int
			avgActual, avgEst float64
			totalActual int64
		)
		if err := rows.Scan(&cat, &total, &done, &missed, &avgActual, &avgEst, &totalActual); err != nil {
			return nil, err
		}
		cs := model.CategoryStats{
			Category:         model.Category(cat),
			Total:            total,
			Completed:        done,
			Missed:           missed,
			AvgActualMinutes: avgActual,
			AvgEstimateMin:   avgEst,
		}
		if total > 0 {
			cs.CompletionRate = float64(done) / float64(total)
		}
		if avgActual > 0 {
			cs.TimeEfficiency = avgEst / avgActual
		}
		// tasks_per_hour = completed / hours of actual work
		if totalActual > 0 {
			hours := float64(totalActual) / 60.0
			cs.TasksPerHour = float64(done) / hours
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// MostMissedCategory
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) MostMissedCategory(ctx context.Context, from, to time.Time) (*model.CategoryMissed, error) {
	const q = `
        SELECT category, COUNT(*) AS n
        FROM tasks
        WHERE status = 'missed'
          AND updated_at >= $1 AND updated_at < $2
        GROUP BY category
        ORDER BY n DESC, category ASC
        LIMIT 1
    `
	var (
		cat string
		n   int
	)
	err := r.pool.QueryRow(ctx, q, from, to).Scan(&cat, &n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("most missed: %w", err)
	}
	return &model.CategoryMissed{Category: model.Category(cat), Count: n}, nil
}

// ---------------------------------------------------------------------------
// DailyCompletionCounts
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) DailyCompletionCounts(ctx context.Context, from, to time.Time) ([]model.DailyCompletion, error) {
	const q = `
        SELECT
            (completed_at AT TIME ZONE 'UTC')::date AS day,
            COUNT(*) AS n
        FROM tasks
        WHERE status = 'completed'
          AND completed_at >= $1 AND completed_at < $2
        GROUP BY day
        ORDER BY day ASC
    `
	rows, err := r.pool.Query(ctx, q, from, to)
	if err != nil {
		return nil, fmt.Errorf("daily completions: %w", err)
	}
	defer rows.Close()

	out := make([]model.DailyCompletion, 0, 8)
	for rows.Next() {
		var dc model.DailyCompletion
		if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
			return nil, err
		}
		dc.Date = dc.Date.UTC()
		out = append(out, dc)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// EffortDistribution
// ---------------------------------------------------------------------------

func (r *PostgresMetricsRepository) EffortDistribution(ctx context.Context, from, to time.Time, largeMinutes int) (float64, int, int, error) {
	const q = `
        SELECT
            COALESCE(AVG(estimated_minutes)::float, 0)                     AS avg_est,
            COUNT(*) FILTER (WHERE estimated_minutes >= $3)                AS large,
            COUNT(*)                                                       AS total
        FROM tasks
        WHERE created_at >= $1 AND created_at < $2
          AND estimated_minutes > 0
    `
	var (
		avg          float64
		large, total int
	)
	if err := r.pool.QueryRow(ctx, q, from, to, largeMinutes).Scan(&avg, &large, &total); err != nil {
		return 0, 0, 0, fmt.Errorf("effort distribution: %w", err)
	}
	return avg, large, total, nil
}
