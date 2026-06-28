package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

func TestRescheduleTodayAndOverdue_SpreadsTwoPerDay(t *testing.T) {
	cal, _ := calendar.Load("UTC")
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	clk := &fixedClock{t: now}
	today := cal.StartOfDay(now)
	yesterday := today.Add(-24 * time.Hour)

	repo := newFakeRepo(clk.Now)
	svc := service.NewTaskService(repo, clk, cal)

	mustCreate := func(title string, due time.Time, status model.Status) {
		t.Helper()
		d := due
		if err := repo.Create(context.Background(), &model.Task{
			Title: title, Status: status, DueDate: &d, Priority: model.PriorityMedium,
			Category: model.CategoryJobApply,
		}); err != nil {
			t.Fatal(err)
		}
	}

	mustCreate("Overdue 1", yesterday, model.StatusPending)
	mustCreate("Overdue 2", yesterday, model.StatusMissed)
	mustCreate("Today 1", today, model.StatusInProgress)
	mustCreate("Today 2", today, model.StatusPending)
	mustCreate("Today 3", today, model.StatusPending)

	tomorrow := today.Add(24 * time.Hour)
	future := today.Add(48 * time.Hour)
	mustCreate("Future", future, model.StatusPending)

	res, err := svc.RescheduleTodayAndOverdue(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Moved != 5 {
		t.Fatalf("moved = %d, want 5", res.Moved)
	}

	tasks, _ := repo.List(context.Background(), repository.TaskFilter{Limit: 20, OrderBy: "due_date"})
	byTitle := map[string]*model.Task{}
	for _, tk := range tasks {
		byTitle[tk.Title] = tk
	}

	expectDue := map[string]time.Time{
		"Overdue 1": tomorrow,
		"Overdue 2": tomorrow,
		"Today 1":   today.Add(48 * time.Hour),
		"Today 2":   today.Add(48 * time.Hour),
		"Today 3":   today.Add(72 * time.Hour),
		"Future":    future,
	}
	for title, want := range expectDue {
		tk := byTitle[title]
		if tk == nil || tk.DueDate == nil || !tk.DueDate.Equal(want) {
			got := "<nil>"
			if tk != nil && tk.DueDate != nil {
				got = tk.DueDate.Format("2006-01-02")
			}
			t.Errorf("%s due = %s, want %s", title, got, want.Format("2006-01-02"))
		}
	}
	if byTitle["Overdue 2"].Status != model.StatusPending {
		t.Errorf("missed task should reopen to pending")
	}
}

func TestShiftOverdueTodayUpcoming_AddsOneDayEach(t *testing.T) {
	cal, _ := calendar.Load("UTC")
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	clk := &fixedClock{t: now}
	today := cal.StartOfDay(now)
	yesterday := today.Add(-24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)
	dayAfter := today.Add(48 * time.Hour)
	farFuture := today.Add(40 * 24 * time.Hour)

	repo := newFakeRepo(clk.Now)
	svc := service.NewTaskService(repo, clk, cal)

	mustCreate := func(title string, due time.Time, status model.Status) {
		t.Helper()
		d := due
		if err := repo.Create(context.Background(), &model.Task{
			Title: title, Status: status, DueDate: &d, Priority: model.PriorityMedium,
			Category: model.CategoryJobApply,
		}); err != nil {
			t.Fatal(err)
		}
	}

	mustCreate("Overdue", yesterday, model.StatusPending)
	mustCreate("Today", today, model.StatusInProgress)
	mustCreate("Upcoming", tomorrow, model.StatusPending)
	mustCreate("Far out", farFuture, model.StatusPending)
	mustCreate("Done today", today, model.StatusCompleted)

	res, err := svc.ShiftOverdueTodayUpcoming(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if res.Moved != 3 {
		t.Fatalf("moved = %d, want 3", res.Moved)
	}

	tasks, _ := repo.List(context.Background(), repository.TaskFilter{Limit: 20, OrderBy: "due_date"})
	byTitle := map[string]*model.Task{}
	for _, tk := range tasks {
		byTitle[tk.Title] = tk
	}

	expectDue := map[string]time.Time{
		"Overdue":   today,
		"Today":     tomorrow,
		"Upcoming":  dayAfter,
		"Far out":   farFuture,
		"Done today": today,
	}
	for title, want := range expectDue {
		tk := byTitle[title]
		if tk == nil || tk.DueDate == nil || !tk.DueDate.Equal(want) {
			got := "<nil>"
			if tk != nil && tk.DueDate != nil {
				got = tk.DueDate.Format("2006-01-02")
			}
			t.Errorf("%s due = %s, want %s", title, got, want.Format("2006-01-02"))
		}
	}
}
