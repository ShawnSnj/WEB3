package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// TaskNoteUpdate is a partial-update DTO for task notes.
type TaskNoteUpdate struct {
	NoteType          *model.NoteType
	Title             *string
	Content           *string
	PersonName        *string
	Company           *string
	RoleTitle         *string
	Platform          *string
	ProfileURL        *string
	MessageContent    *string
	SentAt            *time.Time
	ReplyStatus       *model.ReplyStatus
	ReplyAt           *time.Time
	JobTitle          *string
	JobURL            *string
	ApplicationStatus *model.ApplicationStatus
	AppliedAt         *time.Time
	ResumeVersion     *string
	FitScore          *int
	Source            *model.ApplicationSource
	Notes             *string
	IsMarked          *bool
}

// TaskNoteRepository persists notes linked to tasks.
type TaskNoteRepository interface {
	Create(ctx context.Context, n *model.TaskNote) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.TaskNote, error)
	Update(ctx context.Context, id uuid.UUID, u TaskNoteUpdate) (*model.TaskNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]*model.TaskNote, error)

	CountByNoteType(ctx context.Context, noteType model.NoteType) (int, error)
	CountOutreachTasks(ctx context.Context) (int, error)
	CountApplicationTasks(ctx context.Context) (int, error)
	ListByNoteType(ctx context.Context, noteType model.NoteType, limit int) ([]*model.TaskNoteWithTask, error)
	ListOutreachTasks(ctx context.Context, limit int) ([]*model.Task, error)
	ListApplicationTasks(ctx context.Context, limit int) ([]*model.Task, error)

	GetMarkedWithTask(ctx context.Context) (*model.TaskNoteWithTask, error)
	SetMarked(ctx context.Context, id uuid.UUID, marked bool) error
}
