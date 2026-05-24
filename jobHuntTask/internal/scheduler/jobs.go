package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// Deps bundles the services the scheduler jobs need. All fields are
// required if their corresponding spec is non-empty; nil fields disable
// the matching job.
type Deps struct {
	Tasks     *service.TaskService
	Reminders *service.ReminderService
	Clock     service.Clock
	Logger    *slog.Logger
}

// RegisterJobs registers every scheduler-managed cron job onto s using
// the provided service dependencies. Jobs whose spec is empty in the
// config are skipped automatically by Scheduler.Register.
func RegisterJobs(s *Scheduler, d Deps) error {
	if d.Clock == nil {
		d.Clock = service.SystemClock
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}

	if d.Reminders != nil {
		if err := s.Register("morning_reminder", s.cfg.MorningReminderSpec,
			scheduleDailyReminder(d.Reminders, d.Clock, model.ReminderKindMorning,
				map[string]any{"message": "Plan your day. Pick your top 3 tasks."})); err != nil {
			return err
		}
		if err := s.Register("evening_review_reminder", s.cfg.EveningReviewSpec,
			scheduleDailyReminder(d.Reminders, d.Clock, model.ReminderKindEveningReview,
				map[string]any{"message": "Time for your evening review."})); err != nil {
			return err
		}
		if err := s.Register("weekly_review_reminder", s.cfg.WeeklyReviewSpec,
			scheduleWeeklyReminder(d.Reminders, d.Clock, model.ReminderKindWeeklyReview,
				map[string]any{"message": "Weekly review: wins, blockers, plan next week."})); err != nil {
			return err
		}
		if err := s.Register("reminder_dispatcher", s.cfg.ReminderDispatcherSpec,
			dispatchDueReminders(d.Reminders)); err != nil {
			return err
		}
	}

	if d.Tasks != nil && d.Reminders != nil {
		if err := s.Register("overdue_scanner", s.cfg.OverdueScannerSpec,
			scanOverdueTasks(d.Tasks, d.Reminders, d.Clock)); err != nil {
			return err
		}
	}

	if d.Tasks != nil {
		if err := s.Register("auto_carry_over", s.cfg.AutoCarryOverSpec,
			autoCarryOver(d.Tasks, d.Logger)); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Job factories
// ---------------------------------------------------------------------------

// scheduleDailyReminder enqueues at-most-one reminder per (kind, calendar
// day). Idempotent: dedup_key collapses re-runs on the same day.
func scheduleDailyReminder(svc *service.ReminderService, clk service.Clock, kind model.ReminderKind, payload map[string]any) JobFunc {
	return func(ctx context.Context) error {
		now := clk.Now()
		_, _, err := svc.Schedule(ctx, service.ScheduleInput{
			Kind:         kind,
			ScheduledFor: now,
			Payload:      payload,
			DedupKey:     model.DedupKeyDaily(kind, now),
		})
		return err
	}
}

// scheduleWeeklyReminder enqueues at-most-one reminder per ISO week.
func scheduleWeeklyReminder(svc *service.ReminderService, clk service.Clock, kind model.ReminderKind, payload map[string]any) JobFunc {
	return func(ctx context.Context) error {
		now := clk.Now()
		_, _, err := svc.Schedule(ctx, service.ScheduleInput{
			Kind:         kind,
			ScheduledFor: now,
			Payload:      payload,
			DedupKey:     model.DedupKeyWeekly(kind, now),
		})
		return err
	}
}

// scanOverdueTasks queries currently-overdue tasks and queues one reminder
// per (task, day). Re-runs throughout the day are deduped.
func scanOverdueTasks(tasks *service.TaskService, reminders *service.ReminderService, clk service.Clock) JobFunc {
	return func(ctx context.Context) error {
		overdue, err := tasks.ListOverdue(ctx)
		if err != nil {
			return fmt.Errorf("list overdue tasks: %w", err)
		}
		now := clk.Now()
		var failures int
		for _, t := range overdue {
			payload := map[string]any{
				"task_id":  t.ID.String(),
				"title":    t.Title,
				"priority": string(t.Priority),
				"due_date": t.DueDate,
			}
			if _, _, err := reminders.Schedule(ctx, service.ScheduleInput{
				Kind:         model.ReminderKindOverdue,
				ScheduledFor: now,
				Payload:      payload,
				DedupKey:     model.DedupKeyOverdueTask(t.ID, now),
			}); err != nil {
				failures++
			}
		}
		if failures > 0 {
			return fmt.Errorf("overdue scan: %d of %d failed", failures, len(overdue))
		}
		return nil
	}
}

// autoCarryOver rolls every overdue task into a higher-priority successor.
func autoCarryOver(tasks *service.TaskService, log *slog.Logger) JobFunc {
	return func(ctx context.Context) error {
		created, errs := tasks.CarryOverAllOverdue(ctx)
		log.Info("auto carry-over swept",
			slog.Int("created", len(created)),
			slog.Int("errors", len(errs)),
		)
		if len(errs) > 0 {
			return fmt.Errorf("carry-over: %d errors (first: %v)", len(errs), errs[0])
		}
		return nil
	}
}

// dispatchDueReminders drains the due queue, sending each via the notifier
// and writing back state. This is the "actual delivery" job.
func dispatchDueReminders(svc *service.ReminderService) JobFunc {
	return func(ctx context.Context) error {
		res, err := svc.DispatchDue(ctx)
		if err != nil {
			return err
		}
		_ = res // result is already logged by DispatchDue's structured logs
		return nil
	}
}
