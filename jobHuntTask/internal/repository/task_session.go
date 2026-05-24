package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// SessionFilter captures optional list parameters.
type SessionFilter struct {
	TaskID    *uuid.UUID
	Statuses  []model.SessionStatus
	StartedAt *time.Time // inclusive lower bound
	EndedAt   *time.Time // inclusive upper bound on ended_at
	Limit     int
	Offset    int
}

// SessionUpdate is a partial-update DTO; nil fields mean "leave unchanged".
type SessionUpdate struct {
	Status              *model.SessionStatus
	EndedAt             *time.Time
	ClearEndedAt        bool
	PausedAt            *time.Time
	ClearPausedAt       bool
	TotalPausedSeconds  *int
	Interruptions       *int
	CompletionQuality   *int
	Notes               *string
}

// TaskSessionRepository is the storage contract for task execution sessions.
type TaskSessionRepository interface {
	// Create persists a new session. Returns model.ErrSessionAlreadyRunning
	// if another session for the same task is already active or paused
	// (enforced by a partial unique index).
	Create(ctx context.Context, s *model.TaskSession) error

	// GetByID returns the session or model.ErrSessionNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*model.TaskSession, error)

	// Update applies the partial update; returns model.ErrSessionNotFound
	// if the row does not exist.
	Update(ctx context.Context, id uuid.UUID, u SessionUpdate) (*model.TaskSession, error)

	// Delete removes the session.
	Delete(ctx context.Context, id uuid.UUID) error

	// List returns sessions matching filter.
	List(ctx context.Context, f SessionFilter) ([]*model.TaskSession, error)

	// FindRunningByTask returns the single active-or-paused session for the
	// given task, or model.ErrSessionNotFound if none.
	FindRunningByTask(ctx context.Context, taskID uuid.UUID) (*model.TaskSession, error)

	// SumEffectiveMinutesByTask returns the total effective work minutes
	// across every session for the given task. In-flight pauses on still-
	// running sessions are correctly excluded.
	SumEffectiveMinutesByTask(ctx context.Context, taskID uuid.UUID, now time.Time) (int, error)

	// SumEffectiveMinutesInRange returns total effective work minutes for
	// sessions whose started_at falls in [from, to).
	SumEffectiveMinutesInRange(ctx context.Context, from, to, now time.Time) (int, error)
}
