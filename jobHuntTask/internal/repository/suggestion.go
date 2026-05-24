package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// SuggestionFilter narrows a List() query. Zero-valued fields are ignored.
type SuggestionFilter struct {
	Statuses []model.SuggestionStatus
	Kinds    []model.SuggestionKind
	From     *time.Time // generated_at >= From
	To       *time.Time // generated_at <= To
	Limit    int
	Offset   int
}

// SuggestionRepository persists rule-engine output and exposes the queries
// the service layer needs to manage their lifecycle.
type SuggestionRepository interface {
	// Upsert inserts the suggestion if no ACTIVE row with the same
	// dedup_key exists, otherwise returns the existing row. created=true
	// means a new row was written.
	Upsert(ctx context.Context, s *model.Suggestion) (created bool, err error)

	GetByID(ctx context.Context, id uuid.UUID) (*model.Suggestion, error)

	List(ctx context.Context, f SuggestionFilter) ([]*model.Suggestion, error)

	// Dismiss transitions an active row to dismissed. Returns
	// ErrSuggestionNotFound when no active row matches.
	Dismiss(ctx context.Context, id uuid.UUID, at time.Time) (*model.Suggestion, error)

	// ExpireActiveExcept transitions every currently-active suggestion
	// whose kind is NOT in `keep` to status=expired. Used by the service
	// after a refresh so the active set tracks what the rules currently
	// say. Returns the number of rows expired.
	ExpireActiveExcept(ctx context.Context, keep []model.SuggestionKind, at time.Time) (int, error)

	Delete(ctx context.Context, id uuid.UUID) error
}
