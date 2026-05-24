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

type PostgresSuggestionRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSuggestionRepository(pool *pgxpool.Pool) *PostgresSuggestionRepository {
	return &PostgresSuggestionRepository{pool: pool}
}

var _ SuggestionRepository = (*PostgresSuggestionRepository)(nil)

const suggestionColumns = `
    id, kind, severity, status, title, message, payload,
    dedup_key, generated_at, expires_at, dismissed_at,
    created_at, updated_at
`

// ---------------------------------------------------------------------------
// Upsert
// ---------------------------------------------------------------------------

func (r *PostgresSuggestionRepository) Upsert(ctx context.Context, s *model.Suggestion) (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	payload, err := json.Marshal(s.Payload)
	if err != nil {
		return false, fmt.Errorf("marshal payload: %w", err)
	}
	if s.GeneratedAt.IsZero() {
		s.GeneratedAt = time.Now().UTC()
	}
	if s.Status == "" {
		s.Status = model.SuggestionStatusActive
	}

	const q = `
        INSERT INTO suggestions (
            kind, severity, status, title, message, payload,
            dedup_key, generated_at, expires_at
        ) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9)
        ON CONFLICT (dedup_key) DO NOTHING
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q,
		string(s.Kind), string(s.Severity), string(s.Status),
		s.Title, s.Message, payload,
		s.DedupKey, s.GeneratedAt, s.ExpiresAt,
	)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Row already exists for this (kind, week) — return it as-is
			// regardless of its status. Dismissed/expired stay suppressed
			// within the week.
			existing, fetchErr := r.getByDedupKey(ctx, s.DedupKey)
			if fetchErr != nil {
				return false, fetchErr
			}
			*s = *existing
			return false, nil
		}
		return false, translateSuggestionErr(err)
	}
	return true, nil
}

func (r *PostgresSuggestionRepository) getByDedupKey(ctx context.Context, key string) (*model.Suggestion, error) {
	q := fmt.Sprintf(`SELECT %s FROM suggestions WHERE dedup_key = $1`, suggestionColumns)
	row := r.pool.QueryRow(ctx, q, key)
	s, err := scanSuggestion(row)
	if err != nil {
		return nil, translateSuggestionErr(err)
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

func (r *PostgresSuggestionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Suggestion, error) {
	q := fmt.Sprintf(`SELECT %s FROM suggestions WHERE id = $1`, suggestionColumns)
	row := r.pool.QueryRow(ctx, q, id)
	s, err := scanSuggestion(row)
	if err != nil {
		return nil, translateSuggestionErr(err)
	}
	return s, nil
}

func (r *PostgresSuggestionRepository) List(ctx context.Context, f SuggestionFilter) ([]*model.Suggestion, error) {
	where := make([]string, 0, 4)
	args := make([]any, 0, 6)
	idx := 1

	if len(f.Statuses) > 0 {
		where = append(where, fmt.Sprintf("status = ANY($%d)", idx))
		args = append(args, suggestionStatusesToStrings(f.Statuses))
		idx++
	}
	if len(f.Kinds) > 0 {
		where = append(where, fmt.Sprintf("kind = ANY($%d)", idx))
		args = append(args, suggestionKindsToStrings(f.Kinds))
		idx++
	}
	if f.From != nil {
		where = append(where, fmt.Sprintf("generated_at >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where = append(where, fmt.Sprintf("generated_at <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}

	q := fmt.Sprintf(`SELECT %s FROM suggestions`, suggestionColumns)
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY generated_at DESC"

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
		return nil, translateSuggestionErr(err)
	}
	defer rows.Close()

	out := make([]*model.Suggestion, 0, limit)
	for rows.Next() {
		s, err := scanSuggestion(rows)
		if err != nil {
			return nil, translateSuggestionErr(err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// State transitions
// ---------------------------------------------------------------------------

func (r *PostgresSuggestionRepository) Dismiss(ctx context.Context, id uuid.UUID, at time.Time) (*model.Suggestion, error) {
	q := fmt.Sprintf(`
        UPDATE suggestions SET
            status = 'dismissed',
            dismissed_at = $1
        WHERE id = $2 AND status = 'active'
        RETURNING %s
    `, suggestionColumns)
	row := r.pool.QueryRow(ctx, q, at, id)
	s, err := scanSuggestion(row)
	if err != nil {
		return nil, translateSuggestionErr(err)
	}
	return s, nil
}

func (r *PostgresSuggestionRepository) ExpireActiveExcept(ctx context.Context, keep []model.SuggestionKind, at time.Time) (int, error) {
	keepStrs := suggestionKindsToStrings(keep)

	const q = `
        UPDATE suggestions SET
            status = 'expired',
            expires_at = $1
        WHERE status = 'active' AND NOT (kind = ANY($2))
    `
	tag, err := r.pool.Exec(ctx, q, at, keepStrs)
	if err != nil {
		return 0, translateSuggestionErr(err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *PostgresSuggestionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM suggestions WHERE id = $1`, id)
	if err != nil {
		return translateSuggestionErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrSuggestionNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func scanSuggestion(s scanner) (*model.Suggestion, error) {
	var (
		sg          model.Suggestion
		kind        string
		severity    string
		status      string
		payloadRaw  []byte
		expiresAt   *time.Time
		dismissedAt *time.Time
	)
	if err := s.Scan(
		&sg.ID, &kind, &severity, &status,
		&sg.Title, &sg.Message, &payloadRaw,
		&sg.DedupKey, &sg.GeneratedAt, &expiresAt, &dismissedAt,
		&sg.CreatedAt, &sg.UpdatedAt,
	); err != nil {
		return nil, err
	}
	sg.Kind = model.SuggestionKind(kind)
	sg.Severity = model.SuggestionSeverity(severity)
	sg.Status = model.SuggestionStatus(status)
	sg.ExpiresAt = expiresAt
	sg.DismissedAt = dismissedAt
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &sg.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	if sg.Payload == nil {
		sg.Payload = map[string]any{}
	}
	return &sg, nil
}

func translateSuggestionErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrSuggestionNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		switch pgErr.ConstraintName {
		case "suggestions_kind_valid":
			return model.ErrInvalidSuggestionKind
		case "suggestions_severity_valid":
			return model.ErrInvalidSuggestionSeverity
		case "suggestions_status_valid":
			return model.ErrInvalidSuggestionStatus
		}
	}
	return err
}

func suggestionKindsToStrings(in []model.SuggestionKind) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func suggestionStatusesToStrings(in []model.SuggestionStatus) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
