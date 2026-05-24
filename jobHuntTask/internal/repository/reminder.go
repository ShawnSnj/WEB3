package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// ReminderFilter captures optional list-query parameters.
type ReminderFilter struct {
	Kinds    []model.ReminderKind
	Statuses []model.ReminderStatus
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

// ReminderRepository is the storage contract for reminders.
type ReminderRepository interface {
	// Schedule inserts a new reminder. If r.DedupKey is set and a reminder
	// with that key already exists, Schedule returns the existing row and
	// `created=false` (no error). Otherwise it inserts and returns the new
	// row with `created=true`.
	Schedule(ctx context.Context, r *model.Reminder) (created bool, err error)

	// GetByID returns the reminder or model.ErrReminderNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*model.Reminder, error)

	// ListDue returns up to `limit` reminders whose status is pending or
	// failed and whose scheduled_for is <= now, ordered oldest-first.
	ListDue(ctx context.Context, now time.Time, limit int) ([]*model.Reminder, error)

	// List returns reminders matching filter, ordered scheduled_for DESC.
	List(ctx context.Context, f ReminderFilter) ([]*model.Reminder, error)

	// MarkSent flips status to 'sent' and records sent_at / last_attempt_at.
	// `attempts` is the new (post-increment) attempt count.
	MarkSent(ctx context.Context, id uuid.UUID, at time.Time, attempts int) (*model.Reminder, error)

	// MarkFailed flips status to 'failed' and records the error.
	MarkFailed(ctx context.Context, id uuid.UUID, at time.Time, attempts int, errMsg string) (*model.Reminder, error)

	// MarkCancelled flips status to 'cancelled'.
	MarkCancelled(ctx context.Context, id uuid.UUID) (*model.Reminder, error)

	// Requeue resets status to 'pending' and re-arms scheduled_for. Used by
	// the manual retry path.
	Requeue(ctx context.Context, id uuid.UUID, scheduledFor time.Time) (*model.Reminder, error)

	// Delete removes a reminder.
	Delete(ctx context.Context, id uuid.UUID) error
}
