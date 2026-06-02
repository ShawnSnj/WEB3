// Package repository defines the storage-facing contracts used by the
// service layer and provides a PostgreSQL implementation. The service
// layer depends ONLY on the interfaces declared here — never on pgx
// directly — which keeps business logic test-friendly and the storage
// engine swappable.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// TaskFilter captures the optional query parameters accepted by List.
// All fields are pointers / slices so the zero value means "no filter".
type TaskFilter struct {
	Statuses     []model.Status
	Categories   []model.Category
	Priorities   []model.Priority
	DueBefore    *time.Time
	DueAfter     *time.Time
	CompletedAfter  *time.Time
	CompletedBefore *time.Time
	UpdatedAfter    *time.Time
	UpdatedBefore   *time.Time
	OnlyOverdue  bool
	CarriedOver  *bool

	// Title matches case-insensitively (trimmed) when set.
	Title *string

	// Pagination. Limit == 0 means "use a sensible default" (see impl).
	Limit  int
	Offset int

	// OrderBy: one of "due_date", "priority", "created_at". Empty == created_at desc.
	OrderBy string
}

// TaskUpdate is a partial-update DTO. A nil pointer field means "leave
// the column untouched" — letting clients PATCH a single attribute
// without racing against unrelated fields.
type TaskUpdate struct {
	Title            *string
	Description      *string
	Priority         *model.Priority
	Category         *model.Category
	Status           *model.Status
	EstimatedMinutes *int
	ActualMinutes    *int
	DueDate          *time.Time
	ClearDueDate     bool // explicit: when true, set due_date = NULL
	CarryOverCount   *int
	CompletedAt      *time.Time
	ClearCompletedAt bool // explicit: when true, set completed_at = NULL
}

// TaskRepository is the storage contract for tasks. All methods are
// safe for concurrent use.
type TaskRepository interface {
	// Create persists a new task. On success t.ID, t.CreatedAt, t.UpdatedAt
	// are populated.
	Create(ctx context.Context, t *model.Task) error

	// GetByID returns the task or model.ErrTaskNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)

	// Update applies the partial update. Returns model.ErrTaskNotFound
	// if no row matches.
	Update(ctx context.Context, id uuid.UUID, u TaskUpdate) (*model.Task, error)

	// Delete removes the task. Returns model.ErrTaskNotFound if no row matches.
	Delete(ctx context.Context, id uuid.UUID) error

	// List returns tasks matching filter. Caller controls ordering and paging
	// via the filter fields.
	List(ctx context.Context, f TaskFilter) ([]*model.Task, error)

	// ListOverdue returns non-terminal tasks with due_date < before.
	// Callers should pass start-of-today in the app timezone.
	ListOverdue(ctx context.Context, before time.Time) ([]*model.Task, error)
}
