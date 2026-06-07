package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// TaskNoteUpdate is a partial-update DTO for task notes.
type TaskNoteUpdate struct {
	Title   *string
	Content *string
}

// TaskNoteRepository persists notes linked to tasks.
type TaskNoteRepository interface {
	Create(ctx context.Context, n *model.TaskNote) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.TaskNote, error)
	Update(ctx context.Context, id uuid.UUID, u TaskNoteUpdate) (*model.TaskNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]*model.TaskNote, error)
}
