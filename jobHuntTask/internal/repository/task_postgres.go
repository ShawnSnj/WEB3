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

// PostgresTaskRepository is the pgx-backed implementation of TaskRepository.
type PostgresTaskRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresTaskRepository constructs a repository bound to the given pool.
// The pool's lifecycle is owned by the caller.
func NewPostgresTaskRepository(pool *pgxpool.Pool) *PostgresTaskRepository {
	return &PostgresTaskRepository{pool: pool}
}

// Compile-time interface conformance check.
var _ TaskRepository = (*PostgresTaskRepository)(nil)

// taskColumns lists every column we select, in the order scanTask expects.
const taskColumns = `
    id, title, description, priority, category, status,
    estimated_minutes, actual_minutes, due_date, carry_over_count,
    completed_at, created_at, updated_at
`

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func (r *PostgresTaskRepository) Create(ctx context.Context, t *model.Task) error {
	if err := t.Validate(); err != nil {
		return err
	}
	const q = `
        INSERT INTO tasks (
            title, description, priority, category, status,
            estimated_minutes, actual_minutes, due_date, carry_over_count,
            completed_at
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q,
		t.Title, t.Description, string(t.Priority), string(t.Category), string(t.Status),
		t.EstimatedMinutes, t.ActualMinutes, t.DueDate, t.CarryOverCount,
		t.CompletedAt,
	)
	if err := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return translatePgError(err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (r *PostgresTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	q := fmt.Sprintf(`SELECT %s FROM tasks WHERE id = $1`, taskColumns)
	row := r.pool.QueryRow(ctx, q, id)
	t, err := scanTask(row)
	if err != nil {
		return nil, translatePgError(err)
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// Update (partial)
// ---------------------------------------------------------------------------

func (r *PostgresTaskRepository) Update(ctx context.Context, id uuid.UUID, u TaskUpdate) (*model.Task, error) {
	sets := make([]string, 0, 12)
	args := make([]any, 0, 12)
	idx := 1

	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if u.Title != nil {
		add("title", *u.Title)
	}
	if u.Description != nil {
		add("description", *u.Description)
	}
	if u.Priority != nil {
		add("priority", string(*u.Priority))
	}
	if u.Category != nil {
		add("category", string(*u.Category))
	}
	if u.Status != nil {
		add("status", string(*u.Status))
	}
	if u.EstimatedMinutes != nil {
		add("estimated_minutes", *u.EstimatedMinutes)
	}
	if u.ActualMinutes != nil {
		add("actual_minutes", *u.ActualMinutes)
	}
	switch {
	case u.ClearDueDate:
		add("due_date", nil)
	case u.DueDate != nil:
		add("due_date", *u.DueDate)
	}
	if u.CarryOverCount != nil {
		add("carry_over_count", *u.CarryOverCount)
	}
	switch {
	case u.ClearCompletedAt:
		add("completed_at", nil)
	case u.CompletedAt != nil:
		add("completed_at", *u.CompletedAt)
	}

	if len(sets) == 0 {
		// Nothing to update — just return the current row.
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(
		`UPDATE tasks SET %s WHERE id = $%d RETURNING %s`,
		strings.Join(sets, ", "), idx, taskColumns,
	)
	row := r.pool.QueryRow(ctx, q, args...)
	t, err := scanTask(row)
	if err != nil {
		return nil, translatePgError(err)
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (r *PostgresTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return translatePgError(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrTaskNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (r *PostgresTaskRepository) List(ctx context.Context, f TaskFilter) ([]*model.Task, error) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 6)
	idx := 1

	if len(f.Statuses) > 0 {
		where = append(where, fmt.Sprintf("status = ANY($%d)", idx))
		args = append(args, statusesToStrings(f.Statuses))
		idx++
	}
	if len(f.Categories) > 0 {
		where = append(where, fmt.Sprintf("category = ANY($%d)", idx))
		args = append(args, categoriesToStrings(f.Categories))
		idx++
	}
	if len(f.Priorities) > 0 {
		where = append(where, fmt.Sprintf("priority = ANY($%d)", idx))
		args = append(args, prioritiesToStrings(f.Priorities))
		idx++
	}
	if f.DueBefore != nil {
		where = append(where, fmt.Sprintf("due_date < $%d", idx))
		args = append(args, *f.DueBefore)
		idx++
	}
	if f.DueAfter != nil {
		where = append(where, fmt.Sprintf("due_date >= $%d", idx))
		args = append(args, *f.DueAfter)
		idx++
	}
	if f.OnlyOverdue {
		where = append(where,
			fmt.Sprintf("due_date < $%d", idx),
			"status IN ('pending','in_progress')",
		)
		args = append(args, time.Now())
		idx++
	}
	if f.CarriedOver != nil {
		if *f.CarriedOver {
			where = append(where, "carry_over_count > 0")
		} else {
			where = append(where, "carry_over_count = 0")
		}
	}

	q := fmt.Sprintf(`SELECT %s FROM tasks`, taskColumns)
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY " + orderByClause(f.OrderBy)

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
		return nil, translatePgError(err)
	}
	defer rows.Close()

	out := make([]*model.Task, 0, limit)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, translatePgError(err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, translatePgError(err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// ListOverdue
// ---------------------------------------------------------------------------

func (r *PostgresTaskRepository) ListOverdue(ctx context.Context, now time.Time) ([]*model.Task, error) {
	q := fmt.Sprintf(`
        SELECT %s FROM tasks
        WHERE due_date IS NOT NULL
          AND due_date < $1
          AND status IN ('pending','in_progress')
        ORDER BY due_date ASC
    `, taskColumns)
	rows, err := r.pool.Query(ctx, q, now)
	if err != nil {
		return nil, translatePgError(err)
	}
	defer rows.Close()

	out := make([]*model.Task, 0, 32)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, translatePgError(err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, translatePgError(err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// scanner abstracts pgx.Row and pgx.Rows for scanTask reuse.
type scanner interface {
	Scan(dest ...any) error
}

func scanTask(s scanner) (*model.Task, error) {
	var (
		t                                  model.Task
		priority, category, status         string
		dueDate, completedAt               *time.Time
	)
	if err := s.Scan(
		&t.ID, &t.Title, &t.Description, &priority, &category, &status,
		&t.EstimatedMinutes, &t.ActualMinutes, &dueDate, &t.CarryOverCount,
		&completedAt, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.Priority = model.Priority(priority)
	t.Category = model.Category(category)
	t.Status = model.Status(status)
	t.DueDate = dueDate
	t.CompletedAt = completedAt
	return &t, nil
}

func translatePgError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrTaskNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 23514 = check_violation
		if pgErr.Code == "23514" {
			switch pgErr.ConstraintName {
			case "tasks_status_valid":
				return model.ErrInvalidStatus
			case "tasks_priority_valid":
				return model.ErrInvalidPriority
			case "tasks_category_valid":
				return model.ErrInvalidCategory
			case "tasks_title_not_blank":
				return model.ErrTitleRequired
			case "tasks_estimated_nonneg":
				return model.ErrEstimatedNegative
			case "tasks_actual_nonneg":
				return model.ErrActualNegative
			}
		}
	}
	return err
}

func orderByClause(orderBy string) string {
	switch orderBy {
	case "due_date":
		return "due_date ASC NULLS LAST, created_at DESC"
	case "priority":
		// urgent > high > medium > low → use CASE for deterministic ordering
		return `CASE priority
                    WHEN 'urgent' THEN 0
                    WHEN 'high'   THEN 1
                    WHEN 'medium' THEN 2
                    WHEN 'low'    THEN 3
                END ASC, due_date ASC NULLS LAST`
	default:
		return "created_at DESC"
	}
}

func statusesToStrings(in []model.Status) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func categoriesToStrings(in []model.Category) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func prioritiesToStrings(in []model.Priority) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
