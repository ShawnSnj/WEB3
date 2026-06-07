package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTaskNoteNotFound   = errors.New("task note not found")
	ErrTaskNoteTitleEmpty = errors.New("task note title is required")
)

// TaskNote is a free-form note attached to a task.
type TaskNote struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	Title     string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (n *TaskNote) Validate() error {
	if strings.TrimSpace(n.Title) == "" {
		return ErrTaskNoteTitleEmpty
	}
	return nil
}
