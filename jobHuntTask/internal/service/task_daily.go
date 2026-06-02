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

// SessionStopper stops running timers before a status rollover.
type SessionStopper interface {
	CurrentForTask(ctx context.Context, taskID uuid.UUID) (*model.TaskSession, error)
	Stop(ctx context.Context, sessionID uuid.UUID, in FinishSessionInput) (*model.TaskSession, error)
}

// DailyRolloverResult counts tasks updated by RollDailyPlan.
type DailyRolloverResult struct {
	MarkedMissed int
	SetPending   int
	Skipped      int
	Errors       []error
}

// RollDailyPlan applies the daily task plan for the current calendar day
// (in the app timezone):
//
//   - Tasks due before today still pending or in progress → missed
//   - Tasks due today that are not completed → pending
//
// Completed tasks are left unchanged. The operation is idempotent and safe
// to run on service startup and at midnight.
func (s *TaskService) RollDailyPlan(ctx context.Context, sessions SessionStopper) (DailyRolloverResult, error) {
	now := s.clock.Now()
	todayStart := s.cal.StartOfDay(now)
	todayEnd := todayStart.Add(24 * time.Hour)

	var out DailyRolloverResult

	overdue, err := s.repo.List(ctx, repository.TaskFilter{
		Statuses:  []model.Status{model.StatusPending, model.StatusInProgress},
		DueBefore: &todayStart,
		Limit:     500,
		OrderBy:   "due_date",
	})
	if err != nil {
		return out, fmt.Errorf("list overdue tasks: %w", err)
	}
	for _, t := range overdue {
		stopTaskSession(ctx, sessions, t.ID)
		if _, err := s.MarkMissed(ctx, t.ID); err != nil {
			if errors.Is(err, model.ErrInvalidTransition) {
				out.Skipped++
				continue
			}
			out.Errors = append(out.Errors, fmt.Errorf("missed %s: %w", t.ID, err))
			continue
		}
		out.MarkedMissed++
	}

	today, err := s.repo.List(ctx, repository.TaskFilter{
		DueAfter:  &todayStart,
		DueBefore: &todayEnd,
		Limit:     500,
		OrderBy:   "due_date",
	})
	if err != nil {
		return out, fmt.Errorf("list today tasks: %w", err)
	}
	for _, t := range today {
		if t.Status == model.StatusCompleted || t.Status == model.StatusPending {
			continue
		}
		stopTaskSession(ctx, sessions, t.ID)
		if _, err := s.SetStatus(ctx, t.ID, model.StatusPending); err != nil {
			if errors.Is(err, model.ErrInvalidTransition) {
				out.Skipped++
				continue
			}
			out.Errors = append(out.Errors, fmt.Errorf("pending %s: %w", t.ID, err))
			continue
		}
		out.SetPending++
	}

	return out, nil
}

func stopTaskSession(ctx context.Context, sessions SessionStopper, taskID uuid.UUID) {
	if sessions == nil {
		return
	}
	sess, err := sessions.CurrentForTask(ctx, taskID)
	if err != nil || sess == nil {
		return
	}
	if _, err := sessions.Stop(ctx, sess.ID, FinishSessionInput{}); err != nil {
		slog.Warn("daily rollover: stop session",
			slog.String("task_id", taskID.String()),
			slog.String("err", err.Error()),
		)
	}
}
