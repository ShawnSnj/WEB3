package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shawn/jobhunttask/internal/model"
)

// PostgresTaskNoteRepository is the pgx-backed implementation.
type PostgresTaskNoteRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTaskNoteRepository(pool *pgxpool.Pool) *PostgresTaskNoteRepository {
	return &PostgresTaskNoteRepository{pool: pool}
}

var _ TaskNoteRepository = (*PostgresTaskNoteRepository)(nil)

const taskNoteColumns = `
    id, task_id, title, content, created_at, updated_at
`

func (r *PostgresTaskNoteRepository) Create(ctx context.Context, n *model.TaskNote) error {
	if err := n.Validate(); err != nil {
		return err
	}
	const q = `
        INSERT INTO task_notes (task_id, title, content)
        VALUES ($1, $2, $3)
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q, n.TaskID, strings.TrimSpace(n.Title), n.Content)
	if err := row.Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return translateTaskNoteErr(err)
	}
	return nil
}

func (r *PostgresTaskNoteRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.TaskNote, error) {
	q := fmt.Sprintf(`SELECT %s FROM task_notes WHERE id = $1`, taskNoteColumns)
	row := r.pool.QueryRow(ctx, q, id)
	n, err := scanTaskNote(row)
	if err != nil {
		return nil, translateTaskNoteErr(err)
	}
	return n, nil
}

func (r *PostgresTaskNoteRepository) Update(ctx context.Context, id uuid.UUID, u TaskNoteUpdate) (*model.TaskNote, error) {
	sets := make([]string, 0, 2)
	args := make([]any, 0, 3)
	idx := 1

	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if u.Title != nil {
		add("title", strings.TrimSpace(*u.Title))
	}
	if u.Content != nil {
		add("content", *u.Content)
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
        UPDATE task_notes SET %s
        WHERE id = $%d
        RETURNING %s
    `, strings.Join(sets, ", "), idx, taskNoteColumns)

	row := r.pool.QueryRow(ctx, q, args...)
	n, err := scanTaskNote(row)
	if err != nil {
		return nil, translateTaskNoteErr(err)
	}
	if err := n.Validate(); err != nil {
		return nil, err
	}
	return n, nil
}

func (r *PostgresTaskNoteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM task_notes WHERE id = $1`, id)
	if err != nil {
		return translateTaskNoteErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrTaskNoteNotFound
	}
	return nil
}

func (r *PostgresTaskNoteRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]*model.TaskNote, error) {
	q := fmt.Sprintf(`
        SELECT %s FROM task_notes
        WHERE task_id = $1
        ORDER BY updated_at DESC
    `, taskNoteColumns)
	rows, err := r.pool.Query(ctx, q, taskID)
	if err != nil {
		return nil, translateTaskNoteErr(err)
	}
	defer rows.Close()

	out := make([]*model.TaskNote, 0)
	for rows.Next() {
		n, err := scanTaskNote(rows)
		if err != nil {
			return nil, translateTaskNoteErr(err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

type taskNoteScanner interface {
	Scan(dest ...any) error
}

func scanTaskNote(row taskNoteScanner) (*model.TaskNote, error) {
	var n model.TaskNote
	if err := row.Scan(
		&n.ID, &n.TaskID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &n, nil
}

func translateTaskNoteErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrTaskNoteNotFound
	}
	return err
}
