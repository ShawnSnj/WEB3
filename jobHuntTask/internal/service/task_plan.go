package service

import (
	"context"
	"fmt"
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

// DuplicatePendingGroup is a set of active tasks sharing the same title on
// different due days — usually from carry-over plus bulk reschedule.
type DuplicatePendingGroup struct {
	Title string
	Tasks []*model.Task
}

// FindDuplicatePendingByTitle returns groups of pending/in-progress tasks that
// share a title but occupy different calendar days.
func (s *TaskService) FindDuplicatePendingByTitle(ctx context.Context) ([]DuplicatePendingGroup, error) {
	tasks, err := s.repo.List(ctx, repository.TaskFilter{
		Statuses: []model.Status{model.StatusPending, model.StatusInProgress},
		Limit:    500,
		OrderBy:  "created_at",
	})
	if err != nil {
		return nil, err
	}
	byTitle := make(map[string][]*model.Task)
	for _, t := range tasks {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			continue
		}
		key := strings.ToLower(title)
		byTitle[key] = append(byTitle[key], t)
	}
	out := make([]DuplicatePendingGroup, 0)
	for _, group := range byTitle {
		if len(group) < 2 {
			continue
		}
		out = append(out, DuplicatePendingGroup{
			Title: group[0].Title,
			Tasks: append([]*model.Task(nil), group...),
		})
	}
	return out, nil
}

// CollapseDuplicatePendingByTitle removes extra pending/in-progress rows that
// share a title on different due days. The keeper prefers carry_over_count=0,
// then the earliest due date, then the oldest row.
func (s *TaskService) CollapseDuplicatePendingByTitle(ctx context.Context) (int, error) {
	groups, err := s.FindDuplicatePendingByTitle(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, g := range groups {
		keep := pickDuplicateKeeper(g.Tasks)
		for _, t := range g.Tasks {
			if t.ID == keep.ID {
				continue
			}
			if err := s.repo.Delete(ctx, t.ID); err != nil {
				return removed, fmt.Errorf("delete %s (%q): %w", t.ID, t.Title, err)
			}
			removed++
		}
	}
	return removed, nil
}

// Keeper returns the task row that should survive deduplication.
func (g DuplicatePendingGroup) Keeper() *model.Task {
	return pickDuplicateKeeper(g.Tasks)
}

func pickDuplicateKeeper(tasks []*model.Task) *model.Task {
	best := tasks[0]
	for _, t := range tasks[1:] {
		if duplicateKeeperBetter(t, best) {
			best = t
		}
	}
	return best
}

func duplicateKeeperBetter(a, b *model.Task) bool {
	if a.CarryOverCount != b.CarryOverCount {
		return a.CarryOverCount < b.CarryOverCount
	}
	switch {
	case a.DueDate != nil && b.DueDate != nil && !a.DueDate.Equal(*b.DueDate):
		return a.DueDate.Before(*b.DueDate)
	case a.DueDate != nil && b.DueDate == nil:
		return true
	case a.DueDate == nil && b.DueDate != nil:
		return false
	}
	return a.CreatedAt.Before(b.CreatedAt)
}
