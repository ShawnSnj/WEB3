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

type PostgresTaskSessionRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTaskSessionRepository(pool *pgxpool.Pool) *PostgresTaskSessionRepository {
	return &PostgresTaskSessionRepository{pool: pool}
}

var _ TaskSessionRepository = (*PostgresTaskSessionRepository)(nil)

const sessionColumns = `
    id, task_id, status, started_at, ended_at, paused_at,
    total_paused_seconds, interruptions, completion_quality, notes,
    created_at, updated_at
`

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func (r *PostgresTaskSessionRepository) Create(ctx context.Context, s *model.TaskSession) error {
	if err := s.Validate(); err != nil {
		return err
	}
	const q = `
        INSERT INTO task_execution_sessions (
            task_id, status, started_at, ended_at, paused_at,
            total_paused_seconds, interruptions, completion_quality, notes
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q,
		s.TaskID, string(s.Status), s.StartedAt, s.EndedAt, s.PausedAt,
		s.TotalPausedSeconds, s.Interruptions, s.CompletionQuality, s.Notes,
	)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return translateSessionErr(err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (r *PostgresTaskSessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.TaskSession, error) {
	q := fmt.Sprintf(`SELECT %s FROM task_execution_sessions WHERE id = $1`, sessionColumns)
	row := r.pool.QueryRow(ctx, q, id)
	s, err := scanSession(row)
	if err != nil {
		return nil, translateSessionErr(err)
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Update (partial)
// ---------------------------------------------------------------------------

func (r *PostgresTaskSessionRepository) Update(ctx context.Context, id uuid.UUID, u SessionUpdate) (*model.TaskSession, error) {
	sets := make([]string, 0, 8)
	args := make([]any, 0, 8)
	idx := 1
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if u.Status != nil {
		add("status", string(*u.Status))
	}
	switch {
	case u.ClearEndedAt:
		add("ended_at", nil)
	case u.EndedAt != nil:
		add("ended_at", *u.EndedAt)
	}
	switch {
	case u.ClearPausedAt:
		add("paused_at", nil)
	case u.PausedAt != nil:
		add("paused_at", *u.PausedAt)
	}
	if u.TotalPausedSeconds != nil {
		add("total_paused_seconds", *u.TotalPausedSeconds)
	}
	if u.Interruptions != nil {
		add("interruptions", *u.Interruptions)
	}
	if u.CompletionQuality != nil {
		add("completion_quality", *u.CompletionQuality)
	}
	if u.Notes != nil {
		add("notes", *u.Notes)
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(
		`UPDATE task_execution_sessions SET %s WHERE id = $%d RETURNING %s`,
		strings.Join(sets, ", "), idx, sessionColumns,
	)
	row := r.pool.QueryRow(ctx, q, args...)
	s, err := scanSession(row)
	if err != nil {
		return nil, translateSessionErr(err)
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (r *PostgresTaskSessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM task_execution_sessions WHERE id = $1`, id)
	if err != nil {
		return translateSessionErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrSessionNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (r *PostgresTaskSessionRepository) List(ctx context.Context, f SessionFilter) ([]*model.TaskSession, error) {
	where := make([]string, 0, 4)
	args := make([]any, 0, 6)
	idx := 1

	if f.TaskID != nil {
		where = append(where, fmt.Sprintf("task_id = $%d", idx))
		args = append(args, *f.TaskID)
		idx++
	}
	if len(f.Statuses) > 0 {
		where = append(where, fmt.Sprintf("status = ANY($%d)", idx))
		args = append(args, sessionStatusesToStrings(f.Statuses))
		idx++
	}
	if f.StartedAt != nil {
		where = append(where, fmt.Sprintf("started_at >= $%d", idx))
		args = append(args, *f.StartedAt)
		idx++
	}
	if f.EndedAt != nil {
		where = append(where, fmt.Sprintf("ended_at <= $%d", idx))
		args = append(args, *f.EndedAt)
		idx++
	}

	q := fmt.Sprintf(`SELECT %s FROM task_execution_sessions`, sessionColumns)
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY started_at DESC"

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	q += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, translateSessionErr(err)
	}
	defer rows.Close()

	out := make([]*model.TaskSession, 0, limit)
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, translateSessionErr(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, translateSessionErr(err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// FindRunningByTask
// ---------------------------------------------------------------------------

func (r *PostgresTaskSessionRepository) FindRunningByTask(ctx context.Context, taskID uuid.UUID) (*model.TaskSession, error) {
	q := fmt.Sprintf(`
        SELECT %s FROM task_execution_sessions
        WHERE task_id = $1 AND status IN ('active','paused')
        ORDER BY started_at DESC
        LIMIT 1
    `, sessionColumns)
	row := r.pool.QueryRow(ctx, q, taskID)
	s, err := scanSession(row)
	if err != nil {
		return nil, translateSessionErr(err)
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// SumEffectiveMinutesByTask
// ---------------------------------------------------------------------------

func (r *PostgresTaskSessionRepository) SumEffectiveMinutesByTask(ctx context.Context, taskID uuid.UUID, now time.Time) (int, error) {
	// Effective seconds per session:
	//   (coalesce(ended_at, $now) - started_at) - total_paused_seconds
	//   - extra subtraction when currently paused (paused_at -> $now)
	// Summed and floored to minutes.
	const q = `
        SELECT COALESCE(SUM(
            GREATEST(
                EXTRACT(EPOCH FROM (COALESCE(ended_at, $2) - started_at))::bigint
                - total_paused_seconds
                - CASE
                      WHEN status = 'paused' AND paused_at IS NOT NULL
                      THEN EXTRACT(EPOCH FROM ($2 - paused_at))::bigint
                      ELSE 0
                  END,
                0
            )
        ), 0) / 60
        FROM task_execution_sessions
        WHERE task_id = $1
    `
	var minutes int64
	if err := r.pool.QueryRow(ctx, q, taskID, now).Scan(&minutes); err != nil {
		return 0, translateSessionErr(err)
	}
	return int(minutes), nil
}

// SumEffectiveMinutesInRange aggregates effective work minutes for every
// session started within [from, to).
func (r *PostgresTaskSessionRepository) SumEffectiveMinutesInRange(ctx context.Context, from, to, now time.Time) (int, error) {
	const q = `
        SELECT COALESCE(SUM(
            GREATEST(
                EXTRACT(EPOCH FROM (COALESCE(ended_at, $3) - started_at))::bigint
                - total_paused_seconds
                - CASE
                      WHEN status = 'paused' AND paused_at IS NOT NULL
                      THEN EXTRACT(EPOCH FROM ($3 - paused_at))::bigint
                      ELSE 0
                  END,
                0
            )
        ), 0) / 60
        FROM task_execution_sessions
        WHERE started_at >= $1 AND started_at < $2
    `
	var minutes int64
	if err := r.pool.QueryRow(ctx, q, from, to, now).Scan(&minutes); err != nil {
		return 0, translateSessionErr(err)
	}
	return int(minutes), nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func scanSession(s scanner) (*model.TaskSession, error) {
	var (
		ts        model.TaskSession
		status    string
		endedAt   *time.Time
		pausedAt  *time.Time
		quality   int16
	)
	if err := s.Scan(
		&ts.ID, &ts.TaskID, &status, &ts.StartedAt, &endedAt, &pausedAt,
		&ts.TotalPausedSeconds, &ts.Interruptions, &quality, &ts.Notes,
		&ts.CreatedAt, &ts.UpdatedAt,
	); err != nil {
		return nil, err
	}
	ts.Status = model.SessionStatus(status)
	ts.EndedAt = endedAt
	ts.PausedAt = pausedAt
	ts.CompletionQuality = int(quality)
	return &ts, nil
}

func translateSessionErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrSessionNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			if pgErr.ConstraintName == "uniq_sessions_active_per_task" {
				return model.ErrSessionAlreadyRunning
			}
		case "23514": // check_violation
			switch pgErr.ConstraintName {
			case "sessions_status_valid":
				return model.ErrInvalidSessionStatus
			case "sessions_quality_range":
				return model.ErrInvalidQuality
			case "sessions_interruptions_nonneg":
				return model.ErrInvalidInterruptions
			}
		case "23503": // foreign_key_violation -- task_id missing
			return model.ErrTaskNotFound
		}
	}
	return err
}

func sessionStatusesToStrings(in []model.SessionStatus) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
