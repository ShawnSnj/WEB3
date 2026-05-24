package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Session-specific errors
// ---------------------------------------------------------------------------

var (
	ErrSessionNotFound          = errors.New("session not found")
	ErrInvalidSessionStatus     = errors.New("invalid session status")
	ErrInvalidSessionTransition = errors.New("invalid session transition")
	ErrSessionAlreadyRunning    = errors.New("a session is already active or paused for this task")
	ErrSessionNotRunning        = errors.New("session is not currently active or paused")
	ErrInvalidQuality           = errors.New("completion_quality must be between 0 and 5")
	ErrInvalidInterruptions     = errors.New("interruptions cannot be negative")
)

// ---------------------------------------------------------------------------
// SessionStatus
// ---------------------------------------------------------------------------

type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusStopped   SessionStatus = "stopped"
	SessionStatusCompleted SessionStatus = "completed"
)

// IsValid reports whether s is a known status value.
func (s SessionStatus) IsValid() bool {
	switch s {
	case SessionStatusActive, SessionStatusPaused, SessionStatusStopped, SessionStatusCompleted:
		return true
	}
	return false
}

// IsTerminal reports whether the session is in a final state (no further
// transitions allowed).
func (s SessionStatus) IsTerminal() bool {
	return s == SessionStatusStopped || s == SessionStatusCompleted
}

// IsRunning reports whether the session is still in flight (active or paused).
func (s SessionStatus) IsRunning() bool {
	return s == SessionStatusActive || s == SessionStatusPaused
}

// CanTransitionTo encodes the allowed state machine:
//
//	active  -> paused | stopped | completed
//	paused  -> active | stopped | completed
//	stopped, completed: terminal
func (s SessionStatus) CanTransitionTo(next SessionStatus) bool {
	if !next.IsValid() {
		return false
	}
	if s == next {
		return true // idempotent
	}
	switch s {
	case SessionStatusActive:
		return next == SessionStatusPaused ||
			next == SessionStatusStopped ||
			next == SessionStatusCompleted
	case SessionStatusPaused:
		return next == SessionStatusActive ||
			next == SessionStatusStopped ||
			next == SessionStatusCompleted
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// TaskSession
// ---------------------------------------------------------------------------

// TaskSession represents one contiguous work session on a task, possibly
// interrupted by pauses. Multiple sessions can exist per task (e.g. resumed
// the next day).
type TaskSession struct {
	ID                 uuid.UUID
	TaskID             uuid.UUID
	Status             SessionStatus
	StartedAt          time.Time
	EndedAt            *time.Time
	PausedAt           *time.Time // non-nil iff Status == paused
	TotalPausedSeconds int        // accumulated across all prior pauses
	Interruptions      int
	CompletionQuality  int // 0 = unset, 1..5
	Notes              string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Validate verifies the in-memory invariants of a session.
func (s *TaskSession) Validate() error {
	if !s.Status.IsValid() {
		return ErrInvalidSessionStatus
	}
	if s.Interruptions < 0 {
		return ErrInvalidInterruptions
	}
	if s.CompletionQuality < 0 || s.CompletionQuality > 5 {
		return ErrInvalidQuality
	}
	return nil
}

// EffectiveSeconds returns the actual time worked (in seconds) on this
// session up to `now`. Pauses — both completed pauses (accumulated in
// TotalPausedSeconds) and an in-flight one (PausedAt != nil) — are excluded.
func (s *TaskSession) EffectiveSeconds(now time.Time) int {
	var endRef time.Time
	if s.EndedAt != nil {
		endRef = *s.EndedAt
	} else {
		endRef = now
	}
	total := int(endRef.Sub(s.StartedAt).Seconds()) - s.TotalPausedSeconds
	if s.Status == SessionStatusPaused && s.PausedAt != nil {
		// Still paused: subtract the in-flight pause as well.
		total -= int(endRef.Sub(*s.PausedAt).Seconds())
	}
	if total < 0 {
		return 0
	}
	return total
}

// EffectiveMinutes is EffectiveSeconds rounded to whole minutes (floor).
func (s *TaskSession) EffectiveMinutes(now time.Time) int {
	return s.EffectiveSeconds(now) / 60
}
