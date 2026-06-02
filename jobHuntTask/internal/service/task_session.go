package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// TaskCompleter is the minimal subset of TaskService that the session
// service needs. Declaring it here lets the session service stay decoupled
// from the concrete TaskService struct, which is a big win for testability.
type TaskCompleter interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Task, error)
	MarkCompleted(ctx context.Context, id uuid.UUID, actualMinutes int) (*model.Task, error)
}

// FinishSessionInput carries the optional fields a caller may attach when
// stopping or completing a session.
type FinishSessionInput struct {
	Interruptions     *int    // increment vs. replace? Replace; clients send the running total.
	CompletionQuality *int    // 0..5
	Notes             *string // free-form
}

// ListSessionsInput aliases the repository filter.
type ListSessionsInput = repository.SessionFilter

// TaskSessionService owns the state machine for task execution sessions.
type TaskSessionService struct {
	sessions repository.TaskSessionRepository
	tasks    TaskCompleter
	clock    Clock
}

// NewTaskSessionService constructs the service. `tasks` is used only on
// Start (to verify the task is not terminal) and on Complete (to mark the
// task itself completed with cumulative actual_minutes).
func NewTaskSessionService(
	sessions repository.TaskSessionRepository,
	tasks TaskCompleter,
	clock Clock,
) *TaskSessionService {
	if clock == nil {
		clock = SystemClock
	}
	return &TaskSessionService{sessions: sessions, tasks: tasks, clock: clock}
}

// ---------------------------------------------------------------------------
// Start
// ---------------------------------------------------------------------------

// Start creates a new active session for the task. Fails if:
//   - the task does not exist
//   - the task is in a terminal state
//   - another session is already active or paused for that task
func (s *TaskSessionService) Start(ctx context.Context, taskID uuid.UUID) (*model.TaskSession, error) {
	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status.IsTerminal() {
		return nil, fmt.Errorf("%w: task is %s", model.ErrInvalidTransition, task.Status)
	}

	// Pre-check so we can return a clean error before hitting the DB.
	if running, err := s.sessions.FindRunningByTask(ctx, taskID); err == nil && running != nil {
		return nil, model.ErrSessionAlreadyRunning
	} else if err != nil && !errors.Is(err, model.ErrSessionNotFound) {
		return nil, err
	}

	now := s.clock.Now()
	sess := &model.TaskSession{
		TaskID:    taskID,
		Status:    model.SessionStatusActive,
		StartedAt: now,
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// ---------------------------------------------------------------------------
// Pause
// ---------------------------------------------------------------------------

// Pause moves an active session to paused. Idempotent: pausing an already-
// paused session is a no-op success.
func (s *TaskSessionService) Pause(ctx context.Context, id uuid.UUID) (*model.TaskSession, error) {
	sess, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.Status == model.SessionStatusPaused {
		return sess, nil
	}
	if sess.Status != model.SessionStatusActive {
		return nil, fmt.Errorf("%w: %s -> paused", model.ErrInvalidSessionTransition, sess.Status)
	}
	now := s.clock.Now()
	pausedStatus := model.SessionStatusPaused
	return s.sessions.Update(ctx, id, repository.SessionUpdate{
		Status:   &pausedStatus,
		PausedAt: &now,
	})
}

// ---------------------------------------------------------------------------
// Resume
// ---------------------------------------------------------------------------

// Resume moves a paused session back to active and accumulates the elapsed
// pause window into TotalPausedSeconds.
func (s *TaskSessionService) Resume(ctx context.Context, id uuid.UUID) (*model.TaskSession, error) {
	sess, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.Status == model.SessionStatusActive {
		return sess, nil // idempotent
	}
	if sess.Status != model.SessionStatusPaused || sess.PausedAt == nil {
		return nil, fmt.Errorf("%w: cannot resume from %s", model.ErrInvalidSessionTransition, sess.Status)
	}
	now := s.clock.Now()
	addPaused := int(now.Sub(*sess.PausedAt).Seconds())
	newTotal := sess.TotalPausedSeconds + addPaused
	activeStatus := model.SessionStatusActive
	return s.sessions.Update(ctx, id, repository.SessionUpdate{
		Status:             &activeStatus,
		ClearPausedAt:      true,
		TotalPausedSeconds: &newTotal,
	})
}

// ---------------------------------------------------------------------------
// Stop / Complete
// ---------------------------------------------------------------------------

// Stop ends the session without marking the task complete. Useful when you
// were working but didn't finish.
func (s *TaskSessionService) Stop(ctx context.Context, id uuid.UUID, in FinishSessionInput) (*model.TaskSession, error) {
	return s.endSession(ctx, id, model.SessionStatusStopped, in, false)
}

// Complete ends the session AND marks the underlying task completed,
// setting actual_minutes to the cumulative effective minutes across every
// session for that task.
func (s *TaskSessionService) Complete(ctx context.Context, id uuid.UUID, in FinishSessionInput) (*model.TaskSession, error) {
	return s.endSession(ctx, id, model.SessionStatusCompleted, in, true)
}

// endSession is the shared engine for Stop and Complete.
func (s *TaskSessionService) endSession(
	ctx context.Context,
	id uuid.UUID,
	target model.SessionStatus,
	in FinishSessionInput,
	markTaskCompleted bool,
) (*model.TaskSession, error) {
	if err := validateFinishInput(in); err != nil {
		return nil, err
	}

	sess, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sess.Status.CanTransitionTo(target) {
		return nil, fmt.Errorf("%w: %s -> %s", model.ErrInvalidSessionTransition, sess.Status, target)
	}

	now := s.clock.Now()
	upd := repository.SessionUpdate{
		Status:        &target,
		EndedAt:       &now,
		ClearPausedAt: true,
	}

	// If currently paused, accumulate the in-flight pause before ending so
	// effective-minutes math stays correct.
	if sess.Status == model.SessionStatusPaused && sess.PausedAt != nil {
		addPaused := int(now.Sub(*sess.PausedAt).Seconds())
		newTotal := sess.TotalPausedSeconds + addPaused
		upd.TotalPausedSeconds = &newTotal
	}

	if in.Notes != nil {
		v := strings.TrimSpace(*in.Notes)
		upd.Notes = &v
	}
	if in.Interruptions != nil {
		upd.Interruptions = in.Interruptions
	}
	if in.CompletionQuality != nil {
		upd.CompletionQuality = in.CompletionQuality
	}

	updated, err := s.sessions.Update(ctx, id, upd)
	if err != nil {
		return nil, err
	}

	if markTaskCompleted {
		total, err := s.sessions.SumEffectiveMinutesByTask(ctx, updated.TaskID, now)
		if err != nil {
			return nil, fmt.Errorf("sum minutes: %w", err)
		}
		if _, err := s.tasks.MarkCompleted(ctx, updated.TaskID, total); err != nil {
			// Soft-fail: the session is already ended, but we couldn't
			// transition the task. Return the error so the caller can
			// retry / inspect — the session itself is intact.
			return updated, fmt.Errorf("mark task completed: %w", err)
		}
	}

	return updated, nil
}

// ---------------------------------------------------------------------------
// Read paths
// ---------------------------------------------------------------------------

func (s *TaskSessionService) GetByID(ctx context.Context, id uuid.UUID) (*model.TaskSession, error) {
	return s.sessions.GetByID(ctx, id)
}

func (s *TaskSessionService) List(ctx context.Context, f ListSessionsInput) ([]*model.TaskSession, error) {
	return s.sessions.List(ctx, f)
}

// CurrentForTask returns the currently-running session for a task, if any.
func (s *TaskSessionService) CurrentForTask(ctx context.Context, taskID uuid.UUID) (*model.TaskSession, error) {
	return s.sessions.FindRunningByTask(ctx, taskID)
}

// HasRunningSession reports whether the task has an active or paused session.
func (s *TaskSessionService) HasRunningSession(ctx context.Context, taskID uuid.UUID) (bool, error) {
	_, err := s.CurrentForTask(ctx, taskID)
	if errors.Is(err, model.ErrSessionNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Delete removes a session entry.
func (s *TaskSessionService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.sessions.Delete(ctx, id)
}

// TotalEffectiveMinutesForDay returns effective execution minutes for
// sessions started on the given calendar day (UTC midnight window).
func (s *TaskSessionService) TotalEffectiveMinutesForDay(ctx context.Context, day time.Time) (int, error) {
	from := model.NormalizeDate(day)
	to := from.Add(24 * time.Hour)
	return s.sessions.SumEffectiveMinutesInRange(ctx, from, to, s.clock.Now())
}

// TotalEffectiveMinutesInRange returns effective minutes for sessions
// started in [from, to).
func (s *TaskSessionService) TotalEffectiveMinutesInRange(ctx context.Context, from, to time.Time) (int, error) {
	return s.sessions.SumEffectiveMinutesInRange(ctx, from, to, s.clock.Now())
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func validateFinishInput(in FinishSessionInput) error {
	if in.Interruptions != nil && *in.Interruptions < 0 {
		return model.ErrInvalidInterruptions
	}
	if in.CompletionQuality != nil {
		if *in.CompletionQuality < 0 || *in.CompletionQuality > 5 {
			return model.ErrInvalidQuality
		}
	}
	return nil
}
