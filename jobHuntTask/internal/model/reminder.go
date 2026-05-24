package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Reminder-specific errors
// ---------------------------------------------------------------------------

var (
	ErrReminderNotFound          = errors.New("reminder not found")
	ErrInvalidReminderKind       = errors.New("invalid reminder kind")
	ErrInvalidReminderStatus     = errors.New("invalid reminder status")
	ErrInvalidReminderTransition = errors.New("invalid reminder transition")
)

// ---------------------------------------------------------------------------
// ReminderKind
// ---------------------------------------------------------------------------

// ReminderKind identifies what a reminder is about.
type ReminderKind string

const (
	ReminderKindMorning       ReminderKind = "morning"
	ReminderKindEveningReview ReminderKind = "evening_review"
	ReminderKindWeeklyReview  ReminderKind = "weekly_review"
	ReminderKindOverdue       ReminderKind = "overdue"
	ReminderKindCustom        ReminderKind = "custom"
)

func (k ReminderKind) IsValid() bool {
	switch k {
	case ReminderKindMorning, ReminderKindEveningReview,
		ReminderKindWeeklyReview, ReminderKindOverdue, ReminderKindCustom:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// ReminderStatus
// ---------------------------------------------------------------------------

type ReminderStatus string

const (
	ReminderStatusPending   ReminderStatus = "pending"
	ReminderStatusSent      ReminderStatus = "sent"
	ReminderStatusFailed    ReminderStatus = "failed"
	ReminderStatusCancelled ReminderStatus = "cancelled"
)

func (s ReminderStatus) IsValid() bool {
	switch s {
	case ReminderStatusPending, ReminderStatusSent,
		ReminderStatusFailed, ReminderStatusCancelled:
		return true
	}
	return false
}

// IsTerminal reports whether the status is final. `failed` is NOT terminal —
// the retry queue can move it back to `pending`.
func (s ReminderStatus) IsTerminal() bool {
	return s == ReminderStatusSent || s == ReminderStatusCancelled
}

// CanTransitionTo encodes the allowed state machine.
//
//	pending  -> sent | failed | cancelled
//	failed   -> pending | sent | cancelled
//	sent, cancelled: terminal
func (s ReminderStatus) CanTransitionTo(next ReminderStatus) bool {
	if !next.IsValid() {
		return false
	}
	if s == next {
		return true // idempotent
	}
	switch s {
	case ReminderStatusPending:
		return next == ReminderStatusSent ||
			next == ReminderStatusFailed ||
			next == ReminderStatusCancelled
	case ReminderStatusFailed:
		return next == ReminderStatusPending ||
			next == ReminderStatusSent ||
			next == ReminderStatusCancelled
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Reminder
// ---------------------------------------------------------------------------

// Reminder is one queued notification. The scheduler enqueues these; the
// reminder service dispatches them through a Notifier.
type Reminder struct {
	ID            uuid.UUID
	Kind          ReminderKind
	Status        ReminderStatus
	DedupKey      *string        // optional: collapses duplicate slots
	ScheduledFor  time.Time
	Payload       map[string]any // free-form per kind
	Attempts      int
	LastAttemptAt *time.Time
	LastError     *string
	SentAt        *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Validate enforces in-memory invariants before persisting.
func (r *Reminder) Validate() error {
	if !r.Kind.IsValid() {
		return ErrInvalidReminderKind
	}
	if !r.Status.IsValid() {
		return ErrInvalidReminderStatus
	}
	if r.Attempts < 0 {
		return errors.New("attempts must be >= 0")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Dedup key helpers
// ---------------------------------------------------------------------------

// DedupKeyDaily returns the canonical dedup key for a once-per-day reminder
// such as "morning" or "evening_review".
func DedupKeyDaily(kind ReminderKind, date time.Time) string {
	return fmt.Sprintf("%s:%s", string(kind), NormalizeDate(date).Format("2006-01-02"))
}

// DedupKeyWeekly returns the canonical dedup key for weekly slots. The week
// is identified by ISO year+week so Sun-Sun and Mon-Mon installations agree.
func DedupKeyWeekly(kind ReminderKind, date time.Time) string {
	y, w := NormalizeDate(date).ISOWeek()
	return fmt.Sprintf("%s:%d-W%02d", string(kind), y, w)
}

// DedupKeyOverdueTask returns a per-task-per-day dedup key for overdue
// reminders so a task that's been overdue for a week generates only one
// reminder per day, not one per scan tick.
func DedupKeyOverdueTask(taskID uuid.UUID, date time.Time) string {
	return fmt.Sprintf("%s:%s:%s",
		string(ReminderKindOverdue),
		taskID.String(),
		NormalizeDate(date).Format("2006-01-02"),
	)
}
