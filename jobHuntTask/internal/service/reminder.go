package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// ---------------------------------------------------------------------------
// Inputs
// ---------------------------------------------------------------------------

// ScheduleInput is the service-level DTO for queuing a new reminder.
// DedupKey is the standard tool for "at most one per slot" — when set, the
// repository's partial unique index collapses duplicates.
type ScheduleInput struct {
	Kind         model.ReminderKind
	ScheduledFor time.Time
	Payload      map[string]any
	DedupKey     string // empty -> no dedup constraint
}

// DispatchResult summarises one DispatchDue tick. Useful for both logs and
// for the cron job to return an exit code.
type DispatchResult struct {
	Considered int
	Dispatched int
	Failed     int
	Cancelled  int
	MaxedOut   int // attempts >= MaxAttempts -> cancelled
}

// ---------------------------------------------------------------------------
// ReminderService
// ---------------------------------------------------------------------------

// ReminderService orchestrates the scheduling, dispatch, and retry of
// reminders. It depends on a ReminderRepository for persistence and a
// Notifier for delivery.
type ReminderService struct {
	repo        repository.ReminderRepository
	notifier    Notifier
	clock       Clock
	log         *slog.Logger
	maxAttempts int
	batchSize   int
}

// ReminderServiceConfig bundles tunable knobs.
type ReminderServiceConfig struct {
	MaxAttempts int // attempts before a reminder is cancelled (defaults to 5)
	BatchSize   int // DispatchDue page size (defaults to 100)
}

// NewReminderService constructs the service. Pass a nil Clock for SystemClock.
// A nil Logger falls back to slog.Default().
func NewReminderService(
	repo repository.ReminderRepository,
	notifier Notifier,
	clock Clock,
	log *slog.Logger,
	cfg ReminderServiceConfig,
) *ReminderService {
	if clock == nil {
		clock = SystemClock
	}
	if log == nil {
		log = slog.Default()
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	return &ReminderService{
		repo:        repo,
		notifier:    notifier,
		clock:       clock,
		log:         log,
		maxAttempts: cfg.MaxAttempts,
		batchSize:   cfg.BatchSize,
	}
}

// ---------------------------------------------------------------------------
// Schedule
// ---------------------------------------------------------------------------

// Schedule enqueues a reminder. The returned `created` flag tells the
// caller whether a new row was inserted (true) or an existing reminder
// with the same dedup key was found (false). Callers should treat both
// as success — `created=false` simply means "already queued".
func (s *ReminderService) Schedule(ctx context.Context, in ScheduleInput) (*model.Reminder, bool, error) {
	if !in.Kind.IsValid() {
		return nil, false, model.ErrInvalidReminderKind
	}
	when := in.ScheduledFor
	if when.IsZero() {
		when = s.clock.Now()
	}
	r := &model.Reminder{
		Kind:         in.Kind,
		Status:       model.ReminderStatusPending,
		ScheduledFor: when,
		Payload:      in.Payload,
	}
	if r.Payload == nil {
		r.Payload = map[string]any{}
	}
	if in.DedupKey != "" {
		k := in.DedupKey
		r.DedupKey = &k
	}
	created, err := s.repo.Schedule(ctx, r)
	if err != nil {
		return nil, false, err
	}
	if created {
		s.log.Info("reminder scheduled",
			slog.String("id", r.ID.String()),
			slog.String("kind", string(r.Kind)),
			slog.Time("scheduled_for", r.ScheduledFor),
			slog.Any("dedup_key", r.DedupKey),
		)
	} else {
		s.log.Debug("reminder deduplicated",
			slog.String("existing_id", r.ID.String()),
			slog.String("kind", string(r.Kind)),
			slog.Any("dedup_key", r.DedupKey),
		)
	}
	return r, created, nil
}

// ---------------------------------------------------------------------------
// DispatchDue (the retry-aware dispatcher)
// ---------------------------------------------------------------------------

// DispatchDue is the single function the scheduler should call on a tick.
// It pulls all due reminders (status pending|failed AND scheduled_for<=now),
// hands each to the notifier, and writes back the result. Reminders that
// have already failed MaxAttempts times are cancelled instead of retried.
func (s *ReminderService) DispatchDue(ctx context.Context) (DispatchResult, error) {
	now := s.clock.Now()
	due, err := s.repo.ListDue(ctx, now, s.batchSize)
	if err != nil {
		return DispatchResult{}, fmt.Errorf("list due: %w", err)
	}

	res := DispatchResult{Considered: len(due)}

	for _, r := range due {
		// Stop early if the parent context is cancelled (graceful shutdown).
		if ctx.Err() != nil {
			return res, ctx.Err()
		}

		// Out-of-budget reminders are cancelled rather than retried forever.
		if r.Attempts >= s.maxAttempts {
			if _, cerr := s.repo.MarkCancelled(ctx, r.ID); cerr != nil {
				s.log.Error("failed to cancel exhausted reminder",
					slog.String("id", r.ID.String()),
					slog.String("error", cerr.Error()),
				)
				continue
			}
			res.MaxedOut++
			res.Cancelled++
			s.log.Warn("reminder cancelled after max attempts",
				slog.String("id", r.ID.String()),
				slog.String("kind", string(r.Kind)),
				slog.Int("attempts", r.Attempts),
			)
			continue
		}

		attempts := r.Attempts + 1
		err := s.notifier.Notify(ctx, r)
		if err != nil {
			if _, ferr := s.repo.MarkFailed(ctx, r.ID, now, attempts, err.Error()); ferr != nil {
				s.log.Error("failed to mark reminder failed",
					slog.String("id", r.ID.String()),
					slog.String("error", ferr.Error()),
				)
			}
			res.Failed++
			s.log.Warn("reminder dispatch failed",
				slog.String("id", r.ID.String()),
				slog.String("kind", string(r.Kind)),
				slog.Int("attempts", attempts),
				slog.String("error", err.Error()),
			)
			continue
		}

		if _, serr := s.repo.MarkSent(ctx, r.ID, now, attempts); serr != nil {
			// Notification went out but we failed to persist the success. This
			// is the "at-least-once" failure mode — log loudly and move on.
			s.log.Error("notified but failed to persist sent state",
				slog.String("id", r.ID.String()),
				slog.String("error", serr.Error()),
			)
			res.Failed++
			continue
		}
		res.Dispatched++
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// Retry / cancel / queries
// ---------------------------------------------------------------------------

// Retry re-arms a failed or cancelled reminder by flipping it back to
// pending with scheduled_for=now.
func (s *ReminderService) Retry(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.Status == model.ReminderStatusPending {
		return r, nil // already queued
	}
	if r.Status == model.ReminderStatusSent {
		return nil, fmt.Errorf("%w: cannot retry a sent reminder", model.ErrInvalidReminderTransition)
	}
	return s.repo.Requeue(ctx, id, s.clock.Now())
}

// Cancel transitions a reminder to cancelled.
func (s *ReminderService) Cancel(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !r.Status.CanTransitionTo(model.ReminderStatusCancelled) {
		return nil, fmt.Errorf("%w: %s -> cancelled", model.ErrInvalidReminderTransition, r.Status)
	}
	return s.repo.MarkCancelled(ctx, id)
}

// Get returns a reminder by ID.
func (s *ReminderService) Get(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns reminders matching the filter.
func (s *ReminderService) List(ctx context.Context, f repository.ReminderFilter) ([]*model.Reminder, error) {
	return s.repo.List(ctx, f)
}

// Delete removes a reminder.
func (s *ReminderService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// helper kept here so wrapping callers can detect "not found" without
// importing model.
var _ = errors.Is
