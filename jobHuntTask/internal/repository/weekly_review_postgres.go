package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shawn/jobhunttask/internal/model"
)

type PostgresWeeklyReviewRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresWeeklyReviewRepository(pool *pgxpool.Pool) *PostgresWeeklyReviewRepository {
	return &PostgresWeeklyReviewRepository{pool: pool}
}

var _ WeeklyReviewRepository = (*PostgresWeeklyReviewRepository)(nil)

const weeklyReviewColumns = `
    id, week_start, wins, bottlenecks, improvement_notes, next_week_priorities,
    created_at, updated_at
`

func (r *PostgresWeeklyReviewRepository) Upsert(ctx context.Context, rv *model.WeeklyReview) error {
	rv.WeekStart = model.NormalizeWeekStart(rv.WeekStart)
	const q = `
        INSERT INTO weekly_reviews (
            week_start, wins, bottlenecks, improvement_notes, next_week_priorities
        )
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (week_start) DO UPDATE
            SET wins                 = EXCLUDED.wins,
                bottlenecks          = EXCLUDED.bottlenecks,
                improvement_notes    = EXCLUDED.improvement_notes,
                next_week_priorities = EXCLUDED.next_week_priorities
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q,
		rv.WeekStart, rv.Wins, rv.Bottlenecks, rv.ImprovementNotes, rv.NextWeekPriorities,
	)
	if err := row.Scan(&rv.ID, &rv.CreatedAt, &rv.UpdatedAt); err != nil {
		return err
	}
	return nil
}

func (r *PostgresWeeklyReviewRepository) GetByWeekStart(ctx context.Context, weekStart time.Time) (*model.WeeklyReview, error) {
	q := `SELECT ` + weeklyReviewColumns + ` FROM weekly_reviews WHERE week_start = $1`
	row := r.pool.QueryRow(ctx, q, model.NormalizeWeekStart(weekStart))
	rv, err := scanWeeklyReview(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrWeeklyReviewNotFound
		}
		return nil, err
	}
	return rv, nil
}

func (r *PostgresWeeklyReviewRepository) Delete(ctx context.Context, weekStart time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM weekly_reviews WHERE week_start = $1`,
		model.NormalizeWeekStart(weekStart),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrWeeklyReviewNotFound
	}
	return nil
}

type weeklyReviewScanner interface {
	Scan(dest ...any) error
}

func scanWeeklyReview(row weeklyReviewScanner) (*model.WeeklyReview, error) {
	var rv model.WeeklyReview
	if err := row.Scan(
		&rv.ID, &rv.WeekStart, &rv.Wins, &rv.Bottlenecks,
		&rv.ImprovementNotes, &rv.NextWeekPriorities,
		&rv.CreatedAt, &rv.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &rv, nil
}
