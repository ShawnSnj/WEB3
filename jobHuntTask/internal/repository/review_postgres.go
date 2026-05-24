package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shawn/jobhunttask/internal/model"
)

// PostgresReviewRepository is the pgx-backed implementation.
type PostgresReviewRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReviewRepository(pool *pgxpool.Pool) *PostgresReviewRepository {
	return &PostgresReviewRepository{pool: pool}
}

var _ ReviewRepository = (*PostgresReviewRepository)(nil)

const reviewColumns = `
    id, review_date, reflection, blockers, wins, distractions, notes,
    energy_level, productivity_score, created_at, updated_at
`

// Upsert inserts a fresh review or updates the existing row for review_date.
// Using ON CONFLICT keeps the operation atomic and race-safe.
func (r *PostgresReviewRepository) Upsert(ctx context.Context, rv *model.DailyReview) error {
	if err := rv.Validate(); err != nil {
		return err
	}
	rv.ReviewDate = model.NormalizeDate(rv.ReviewDate)

	const q = `
        INSERT INTO daily_reviews (
            review_date, reflection, blockers, wins, distractions, notes,
            energy_level, productivity_score
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        ON CONFLICT (review_date) DO UPDATE
            SET reflection         = EXCLUDED.reflection,
                blockers           = EXCLUDED.blockers,
                wins               = EXCLUDED.wins,
                distractions       = EXCLUDED.distractions,
                notes              = EXCLUDED.notes,
                energy_level       = EXCLUDED.energy_level,
                productivity_score = EXCLUDED.productivity_score
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q,
		rv.ReviewDate, rv.Reflection, rv.Blockers, rv.Wins, rv.Distractions, rv.Notes,
		rv.EnergyLevel, rv.ProductivityScore,
	)
	if err := row.Scan(&rv.ID, &rv.CreatedAt, &rv.UpdatedAt); err != nil {
		return translateReviewErr(err)
	}
	return nil
}

func (r *PostgresReviewRepository) GetByDate(ctx context.Context, date time.Time) (*model.DailyReview, error) {
	q := fmt.Sprintf(`SELECT %s FROM daily_reviews WHERE review_date = $1`, reviewColumns)
	row := r.pool.QueryRow(ctx, q, model.NormalizeDate(date))
	rv, err := scanReview(row)
	if err != nil {
		return nil, translateReviewErr(err)
	}
	return rv, nil
}

func (r *PostgresReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.DailyReview, error) {
	q := fmt.Sprintf(`SELECT %s FROM daily_reviews WHERE id = $1`, reviewColumns)
	row := r.pool.QueryRow(ctx, q, id)
	rv, err := scanReview(row)
	if err != nil {
		return nil, translateReviewErr(err)
	}
	return rv, nil
}

func (r *PostgresReviewRepository) List(ctx context.Context, f ReviewFilter) ([]*model.DailyReview, error) {
	where := make([]string, 0, 2)
	args := make([]any, 0, 4)
	idx := 1

	if f.From != nil {
		where = append(where, fmt.Sprintf("review_date >= $%d", idx))
		args = append(args, model.NormalizeDate(*f.From))
		idx++
	}
	if f.To != nil {
		where = append(where, fmt.Sprintf("review_date <= $%d", idx))
		args = append(args, model.NormalizeDate(*f.To))
		idx++
	}

	q := fmt.Sprintf(`SELECT %s FROM daily_reviews`, reviewColumns)
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY review_date DESC"

	limit := f.Limit
	if limit <= 0 {
		limit = 60
	}
	if limit > 365 {
		limit = 365
	}
	q += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, translateReviewErr(err)
	}
	defer rows.Close()

	out := make([]*model.DailyReview, 0, limit)
	for rows.Next() {
		rv, err := scanReview(rows)
		if err != nil {
			return nil, translateReviewErr(err)
		}
		out = append(out, rv)
	}
	if err := rows.Err(); err != nil {
		return nil, translateReviewErr(err)
	}
	return out, nil
}

func (r *PostgresReviewRepository) Delete(ctx context.Context, date time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM daily_reviews WHERE review_date = $1`,
		model.NormalizeDate(date),
	)
	if err != nil {
		return translateReviewErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrReviewNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func scanReview(s scanner) (*model.DailyReview, error) {
	var rv model.DailyReview
	if err := s.Scan(
		&rv.ID, &rv.ReviewDate, &rv.Reflection, &rv.Blockers, &rv.Wins,
		&rv.Distractions, &rv.Notes,
		&rv.EnergyLevel, &rv.ProductivityScore, &rv.CreatedAt, &rv.UpdatedAt,
	); err != nil {
		return nil, err
	}
	rv.ReviewDate = model.NormalizeDate(rv.ReviewDate)
	return &rv, nil
}

func translateReviewErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrReviewNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		switch pgErr.ConstraintName {
		case "daily_reviews_energy_range":
			return model.ErrInvalidEnergyLevel
		case "daily_reviews_productivity_range":
			return model.ErrInvalidProductivity
		}
	}
	return err
}
