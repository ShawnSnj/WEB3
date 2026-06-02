package service

import (
	"context"
	"strings"
	"time"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// planKey identifies a daily plan slot: same title on the same calendar due day.
type planKey struct {
	title  string
	dueDay string // YYYY-MM-DD in app timezone
}

func (s *TaskService) planKeyFor(title string, due *time.Time) (planKey, bool) {
	title = strings.TrimSpace(title)
	if title == "" || due == nil {
		return planKey{}, false
	}
	return planKey{
		title:  strings.ToLower(title),
		dueDay: s.cal.FormatDate(*due),
	}, true
}

func (s *TaskService) planKeyForTask(t *model.Task) (planKey, bool) {
	return s.planKeyFor(t.Title, t.DueDate)
}

// planExists reports whether any task already occupies the same title + due day.
func (s *TaskService) planExists(ctx context.Context, title string, due time.Time) (bool, error) {
	dayStart := s.cal.StartOfDay(due)
	dayEnd := dayStart.Add(24 * time.Hour)
	title = strings.TrimSpace(title)
	existing, err := s.repo.List(ctx, repository.TaskFilter{
		Title:     &title,
		DueAfter:  &dayStart,
		DueBefore: &dayEnd,
		Limit:     1,
	})
	if err != nil {
		return false, err
	}
	return len(existing) > 0, nil
}

// selectPrimaryOverdue picks one overdue task per plan slot (oldest created_at wins).
// Duplicates in the same slot are returned separately so callers can mark them missed
// without spawning another carry-over successor.
func selectPrimaryOverdue(tasks []*model.Task, cal *calendar.Calendar) (primary []*model.Task, duplicates []*model.Task) {
	seen := make(map[planKey]*model.Task)
	dupes := make([]*model.Task, 0)

	for _, t := range tasks {
		key, ok := planKeyForTaskWithCal(t, cal)
		if !ok {
			primary = append(primary, t)
			continue
		}
		prev, exists := seen[key]
		if !exists {
			seen[key] = t
			continue
		}
		if t.CreatedAt.Before(prev.CreatedAt) {
			dupes = append(dupes, prev)
			seen[key] = t
		} else {
			dupes = append(dupes, t)
		}
	}
	for _, t := range seen {
		primary = append(primary, t)
	}
	return primary, dupes
}

func planKeyForTaskWithCal(t *model.Task, cal *calendar.Calendar) (planKey, bool) {
	title := strings.TrimSpace(t.Title)
	if title == "" || t.DueDate == nil {
		return planKey{}, false
	}
	return planKey{
		title:  strings.ToLower(title),
		dueDay: cal.FormatDate(*t.DueDate),
	}, true
}
