package repository

import (
	"context"
	"encoding/json"
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

type PostgresReminderRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReminderRepository(pool *pgxpool.Pool) *PostgresReminderRepository {
	return &PostgresReminderRepository{pool: pool}
}

var _ ReminderRepository = (*PostgresReminderRepository)(nil)

const reminderColumns = `
    id, kind, status, dedup_key, scheduled_for, payload,
    attempts, last_attempt_at, last_error, sent_at,
    created_at, updated_at
`

// ---------------------------------------------------------------------------
// Schedule (dedup-safe upsert)
// ---------------------------------------------------------------------------

func (r *PostgresReminderRepository) Schedule(ctx context.Context, rem *model.Reminder) (bool, error) {
	if err := rem.Validate(); err != nil {
		return false, err
	}
	payload, err := json.Marshal(rem.Payload)
	if err != nil {
		return false, fmt.Errorf("marshal payload: %w", err)
	}

	const q = `
        INSERT INTO reminders (
            kind, status, dedup_key, scheduled_for, payload
        ) VALUES ($1, $2, $3, $4, $5::jsonb)
        ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q,
		string(rem.Kind), string(rem.Status), rem.DedupKey, rem.ScheduledFor, payload,
	)
	if err := row.Scan(&rem.ID, &rem.CreatedAt, &rem.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Conflict — fetch and return the existing row.
			if rem.DedupKey == nil {
				return false, fmt.Errorf("insert returned no rows but no dedup_key present")
			}
			existing, ferr := r.getByDedupKey(ctx, *rem.DedupKey)
			if ferr != nil {
				return false, ferr
			}
			*rem = *existing
			return false, nil
		}
		return false, translateReminderErr(err)
	}
	return true, nil
}

func (r *PostgresReminderRepository) getByDedupKey(ctx context.Context, key string) (*model.Reminder, error) {
	q := fmt.Sprintf(`SELECT %s FROM reminders WHERE dedup_key = $1`, reminderColumns)
	row := r.pool.QueryRow(ctx, q, key)
	rem, err := scanReminder(row)
	if err != nil {
		return nil, translateReminderErr(err)
	}
	return rem, nil
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

func (r *PostgresReminderRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	q := fmt.Sprintf(`SELECT %s FROM reminders WHERE id = $1`, reminderColumns)
	row := r.pool.QueryRow(ctx, q, id)
	rem, err := scanReminder(row)
	if err != nil {
		return nil, translateReminderErr(err)
	}
	return rem, nil
}

func (r *PostgresReminderRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]*model.Reminder, error) {
	if limit <= 0 {
		limit = 100
	}
	q := fmt.Sprintf(`
        SELECT %s FROM reminders
        WHERE status IN ('pending', 'failed') AND scheduled_for <= $1
        ORDER BY scheduled_for ASC
        LIMIT $2
    `, reminderColumns)
	rows, err := r.pool.Query(ctx, q, now, limit)
	if err != nil {
		return nil, translateReminderErr(err)
	}
	defer rows.Close()

	out := make([]*model.Reminder, 0, limit)
	for rows.Next() {
		rem, err := scanReminder(rows)
		if err != nil {
			return nil, translateReminderErr(err)
		}
		out = append(out, rem)
	}
	return out, rows.Err()
}

func (r *PostgresReminderRepository) List(ctx context.Context, f ReminderFilter) ([]*model.Reminder, error) {
	where := make([]string, 0, 4)
	args := make([]any, 0, 6)
	idx := 1

	if len(f.Kinds) > 0 {
		where = append(where, fmt.Sprintf("kind = ANY($%d)", idx))
		args = append(args, reminderKindsToStrings(f.Kinds))
		idx++
	}
	if len(f.Statuses) > 0 {
		where = append(where, fmt.Sprintf("status = ANY($%d)", idx))
		args = append(args, reminderStatusesToStrings(f.Statuses))
		idx++
	}
	if f.From != nil {
		where = append(where, fmt.Sprintf("scheduled_for >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where = append(where, fmt.Sprintf("scheduled_for <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}

	q := fmt.Sprintf(`SELECT %s FROM reminders`, reminderColumns)
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY scheduled_for DESC"

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
		return nil, translateReminderErr(err)
	}
	defer rows.Close()

	out := make([]*model.Reminder, 0, limit)
	for rows.Next() {
		rem, err := scanReminder(rows)
		if err != nil {
			return nil, translateReminderErr(err)
		}
		out = append(out, rem)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// State transitions
// ---------------------------------------------------------------------------

func (r *PostgresReminderRepository) MarkSent(ctx context.Context, id uuid.UUID, at time.Time, attempts int) (*model.Reminder, error) {
	q := fmt.Sprintf(`
        UPDATE reminders SET
            status = 'sent',
            sent_at = $1,
            last_attempt_at = $1,
            last_error = NULL,
            attempts = $2
        WHERE id = $3
        RETURNING %s
    `, reminderColumns)
	row := r.pool.QueryRow(ctx, q, at, attempts, id)
	rem, err := scanReminder(row)
	if err != nil {
		return nil, translateReminderErr(err)
	}
	return rem, nil
}

func (r *PostgresReminderRepository) MarkFailed(ctx context.Context, id uuid.UUID, at time.Time, attempts int, errMsg string) (*model.Reminder, error) {
	q := fmt.Sprintf(`
        UPDATE reminders SET
            status = 'failed',
            last_attempt_at = $1,
            last_error = $2,
            attempts = $3
        WHERE id = $4
        RETURNING %s
    `, reminderColumns)
	row := r.pool.QueryRow(ctx, q, at, errMsg, attempts, id)
	rem, err := scanReminder(row)
	if err != nil {
		return nil, translateReminderErr(err)
	}
	return rem, nil
}

func (r *PostgresReminderRepository) MarkCancelled(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	q := fmt.Sprintf(`
        UPDATE reminders SET status = 'cancelled'
        WHERE id = $1
        RETURNING %s
    `, reminderColumns)
	row := r.pool.QueryRow(ctx, q, id)
	rem, err := scanReminder(row)
	if err != nil {
		return nil, translateReminderErr(err)
	}
	return rem, nil
}

func (r *PostgresReminderRepository) Requeue(ctx context.Context, id uuid.UUID, scheduledFor time.Time) (*model.Reminder, error) {
	q := fmt.Sprintf(`
        UPDATE reminders SET
            status = 'pending',
            scheduled_for = $1,
            last_error = NULL
        WHERE id = $2
        RETURNING %s
    `, reminderColumns)
	row := r.pool.QueryRow(ctx, q, scheduledFor, id)
	rem, err := scanReminder(row)
	if err != nil {
		return nil, translateReminderErr(err)
	}
	return rem, nil
}

func (r *PostgresReminderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reminders WHERE id = $1`, id)
	if err != nil {
		return translateReminderErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrReminderNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func scanReminder(s scanner) (*model.Reminder, error) {
	var (
		rem         model.Reminder
		kind        string
		status      string
		dedupKey    *string
		payloadRaw  []byte
		lastAttempt *time.Time
		lastError   *string
		sentAt      *time.Time
	)
	if err := s.Scan(
		&rem.ID, &kind, &status, &dedupKey, &rem.ScheduledFor, &payloadRaw,
		&rem.Attempts, &lastAttempt, &lastError, &sentAt,
		&rem.CreatedAt, &rem.UpdatedAt,
	); err != nil {
		return nil, err
	}
	rem.Kind = model.ReminderKind(kind)
	rem.Status = model.ReminderStatus(status)
	rem.DedupKey = dedupKey
	rem.LastAttemptAt = lastAttempt
	rem.LastError = lastError
	rem.SentAt = sentAt
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &rem.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	if rem.Payload == nil {
		rem.Payload = map[string]any{}
	}
	return &rem, nil
}

func translateReminderErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrReminderNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		switch pgErr.ConstraintName {
		case "reminders_kind_valid":
			return model.ErrInvalidReminderKind
		case "reminders_status_valid":
			return model.ErrInvalidReminderStatus
		}
	}
	return err
}

func reminderKindsToStrings(in []model.ReminderKind) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func reminderStatusesToStrings(in []model.ReminderStatus) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
