package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

func TestService_RollDailyPlan(t *testing.T) {
	t.Parallel()
	svc, repo, clk := newSvc(t)
	ctx := context.Background()

	today := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC) // matches fixed clock in newSvc
	yesterday := today.Add(-24 * time.Hour)
	old := today.Add(-48 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)
	_ = clk

	oldPending, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "Old pending", DueDate: &old,
	})
	yPending, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "Y pending", DueDate: &yesterday,
	})
	yInProg, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "Y in prog", DueDate: &yesterday,
	})
	_, _ = svc.MarkInProgress(ctx, yInProg.ID)

	tMissed, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "T missed", DueDate: &today,
	})
	_, _ = svc.MarkMissed(ctx, tMissed.ID)

	tInProg, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "T in prog", DueDate: &today,
	})
	_, _ = svc.MarkInProgress(ctx, tInProg.ID)

	tPending, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "T pending", DueDate: &today,
	})

	tDone, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "T done", DueDate: &today,
	})
	_, _ = svc.MarkCompleted(ctx, tDone.ID, 10)

	future, _ := svc.Create(ctx, service.CreateTaskInput{
		Title: "Future", DueDate: &tomorrow,
	})

	res, err := svc.RollDailyPlan(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.MarkedMissed != 3 {
		t.Errorf("marked missed = %d, want 3", res.MarkedMissed)
	}
	if res.SetPending != 2 {
		t.Errorf("set pending = %d, want 2 (missed + in_progress due today)", res.SetPending)
	}

	check := func(tk *model.Task, want model.Status) {
		t.Helper()
		got, _ := repo.GetByID(ctx, tk.ID)
		if got.Status != want {
			t.Errorf("%q status = %s, want %s", tk.Title, got.Status, want)
		}
	}
	check(oldPending, model.StatusMissed)
	check(yPending, model.StatusMissed)
	check(yInProg, model.StatusMissed)
	check(tMissed, model.StatusPending)
	check(tInProg, model.StatusPending)
	check(tPending, model.StatusPending)
	check(tDone, model.StatusCompleted)
	check(future, model.StatusPending)
}
