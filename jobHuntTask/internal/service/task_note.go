package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// CreateTaskNoteInput is the service DTO for creating a note.
type CreateTaskNoteInput struct {
	TaskID  uuid.UUID
	Title   string
	Content string
}

// UpdateTaskNoteInput is the service DTO for updating a note.
type UpdateTaskNoteInput struct {
	Title   *string
	Content *string
}

// TaskNoteService manages notes attached to tasks.
type TaskNoteService struct {
	notes repository.TaskNoteRepository
	tasks repository.TaskRepository
}

func NewTaskNoteService(notes repository.TaskNoteRepository, tasks repository.TaskRepository) *TaskNoteService {
	return &TaskNoteService{notes: notes, tasks: tasks}
}

func (s *TaskNoteService) Create(ctx context.Context, in CreateTaskNoteInput) (*model.TaskNote, error) {
	if _, err := s.tasks.GetByID(ctx, in.TaskID); err != nil {
		return nil, err
	}
	n := &model.TaskNote{
		TaskID:  in.TaskID,
		Title:   strings.TrimSpace(in.Title),
		Content: strings.TrimSpace(in.Content),
	}
	if n.Title == "" {
		n.Title = defaultNoteTitle(n.Content)
	}
	if err := n.Validate(); err != nil {
		return nil, err
	}
	if err := s.notes.Create(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *TaskNoteService) Get(ctx context.Context, id uuid.UUID) (*model.TaskNote, error) {
	return s.notes.GetByID(ctx, id)
}

func (s *TaskNoteService) Update(ctx context.Context, id uuid.UUID, in UpdateTaskNoteInput) (*model.TaskNote, error) {
	upd := repository.TaskNoteUpdate{}
	if in.Title != nil {
		v := strings.TrimSpace(*in.Title)
		if v == "" {
			return nil, model.ErrTaskNoteTitleEmpty
		}
		upd.Title = &v
	}
	if in.Content != nil {
		v := strings.TrimSpace(*in.Content)
		upd.Content = &v
	}
	return s.notes.Update(ctx, id, upd)
}

func (s *TaskNoteService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.notes.Delete(ctx, id)
}

func (s *TaskNoteService) ListByTask(ctx context.Context, taskID uuid.UUID) ([]*model.TaskNote, error) {
	if _, err := s.tasks.GetByID(ctx, taskID); err != nil {
		return nil, err
	}
	return s.notes.ListByTaskID(ctx, taskID)
}

func defaultNoteTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "Untitled note"
	}
	if idx := strings.IndexAny(content, "\n\r"); idx > 0 {
		content = content[:idx]
	}
	if len(content) > 60 {
		content = content[:60]
	}
	return content
}
