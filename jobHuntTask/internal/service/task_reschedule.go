package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// RescheduleSpreadResult summarizes a bulk reschedule operation.
type RescheduleSpreadResult struct {
	Moved  int
	Errors []error
	Plan   []RescheduleAssignment
}

// RescheduleAssignment records where a task was moved.
type RescheduleAssignment struct {
	TaskID   string
	Title    string
	OldDue   string
	NewDue   string
	OldStatus model.Status
	NewStatus model.Status
}

// RescheduleTodayAndOverdue moves every non-completed task due today or earlier
// onto future calendar days starting tomorrow, spreading perDay tasks per day
// while preserving due_date then created_at order.
func (s *TaskService) RescheduleTodayAndOverdue(ctx context.Context, perDay int) (RescheduleSpreadResult, error) {
	if perDay <= 0 {
		perDay = 2
	}

	var out RescheduleSpreadResult
	now := s.clock.Now()
	todayStart := s.cal.StartOfDay(now)
	todayEnd := todayStart.Add(24 * time.Hour)

	statuses := []model.Status{
		model.StatusPending,
		model.StatusInProgress,
		model.StatusMissed,
	}

	overdue, err := s.repo.List(ctx, repository.TaskFilter{
		Statuses:  statuses,
		DueBefore: &todayStart,
		Limit:     500,
		OrderBy:   "due_date",
	})
	if err != nil {
		return out, fmt.Errorf("list overdue tasks: %w", err)
	}

	todayTasks, err := s.repo.List(ctx, repository.TaskFilter{
		Statuses:  statuses,
		DueAfter:  &todayStart,
		DueBefore: &todayEnd,
		Limit:     500,
		OrderBy:   "due_date",
	})
	if err != nil {
		return out, fmt.Errorf("list today tasks: %w", err)
	}

	tasks := append(overdue, todayTasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.DueDate == nil && b.DueDate == nil {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		if a.DueDate == nil {
			return false
		}
		if b.DueDate == nil {
			return true
		}
		if !a.DueDate.Equal(*b.DueDate) {
			return a.DueDate.Before(*b.DueDate)
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})

	startDay := todayStart.Add(24 * time.Hour) // tomorrow
	pending := model.StatusPending

	for i, t := range tasks {
		dayOffset := i / perDay
		newDue := startDay.Add(time.Duration(dayOffset) * 24 * time.Hour)
		newDue = s.cal.StartOfDay(newDue)

		oldDue := "—"
		if t.DueDate != nil {
			oldDue = s.cal.FormatDate(*t.DueDate)
		}

		u := repository.TaskUpdate{DueDate: &newDue}
		if t.Status != model.StatusPending {
			u.Status = &pending
		}

		if _, err := s.repo.Update(ctx, t.ID, u); err != nil {
			out.Errors = append(out.Errors, fmt.Errorf("%s (%s): %w", t.Title, t.ID, err))
			continue
		}

		out.Moved++
		newStatus := t.Status
		if u.Status != nil {
			newStatus = *u.Status
		}
		out.Plan = append(out.Plan, RescheduleAssignment{
			TaskID:    t.ID.String(),
			Title:     t.Title,
			OldDue:    oldDue,
			NewDue:    s.cal.FormatDate(newDue),
			OldStatus: t.Status,
			NewStatus: newStatus,
		})
	}

	return out, nil
}
