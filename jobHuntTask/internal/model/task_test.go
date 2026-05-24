package model_test

import (
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

func TestStatus_IsValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   model.Status
		want bool
	}{
		{model.StatusPending, true},
		{model.StatusInProgress, true},
		{model.StatusCompleted, true},
		{model.StatusMissed, true},
		{model.Status(""), false},
		{model.Status("bogus"), false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Errorf("IsValid(%q)=%v, want %v", c.in, got, c.want)
		}
	}
}

func TestStatus_CanTransitionTo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to model.Status
		want     bool
	}{
		{model.StatusPending, model.StatusInProgress, true},
		{model.StatusPending, model.StatusCompleted, true},
		{model.StatusPending, model.StatusMissed, true},
		{model.StatusInProgress, model.StatusCompleted, true},
		{model.StatusInProgress, model.StatusMissed, true},
		{model.StatusInProgress, model.StatusPending, true},
		{model.StatusCompleted, model.StatusPending, false},
		{model.StatusCompleted, model.StatusInProgress, false},
		{model.StatusMissed, model.StatusPending, false},
		{model.StatusPending, model.Status("nope"), false},
		{model.StatusPending, model.StatusPending, true}, // idempotent
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.want {
			t.Errorf("%s -> %s: got %v want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestPriority_Bump(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want model.Priority
	}{
		{model.PriorityLow, model.PriorityMedium},
		{model.PriorityMedium, model.PriorityHigh},
		{model.PriorityHigh, model.PriorityUrgent},
		{model.PriorityUrgent, model.PriorityUrgent}, // capped
		{model.Priority("garbage"), model.PriorityMedium},
	}
	for _, c := range cases {
		if got := c.in.Bump(); got != c.want {
			t.Errorf("Bump(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestCategory_IsValid(t *testing.T) {
	t.Parallel()
	for _, c := range model.AllCategories() {
		if !c.IsValid() {
			t.Errorf("expected %q to be valid", c)
		}
	}
	if model.Category("invalid").IsValid() {
		t.Error("unexpected: 'invalid' considered valid")
	}
}

func TestTask_Validate(t *testing.T) {
	t.Parallel()
	base := func() *model.Task {
		return &model.Task{
			Title:    "Do the thing",
			Priority: model.PriorityMedium,
			Category: model.CategoryJobApply,
			Status:   model.StatusPending,
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("expected base to be valid, got %v", err)
	}

	cases := []struct {
		name  string
		mut   func(*model.Task)
		want  error
	}{
		{"blank title", func(t *model.Task) { t.Title = "   " }, model.ErrTitleRequired},
		{"bad priority", func(t *model.Task) { t.Priority = "nope" }, model.ErrInvalidPriority},
		{"bad category", func(t *model.Task) { t.Category = "nope" }, model.ErrInvalidCategory},
		{"bad status", func(t *model.Task) { t.Status = "nope" }, model.ErrInvalidStatus},
		{"negative estimated", func(t *model.Task) { t.EstimatedMinutes = -1 }, model.ErrEstimatedNegative},
		{"negative actual", func(t *model.Task) { t.ActualMinutes = -1 }, model.ErrActualNegative},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			task := base()
			c.mut(task)
			if err := task.Validate(); err != c.want {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestTask_IsOverdue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name   string
		status model.Status
		due    *time.Time
		want   bool
	}{
		{"pending past due", model.StatusPending, &past, true},
		{"in-progress past due", model.StatusInProgress, &past, true},
		{"completed past due", model.StatusCompleted, &past, false},
		{"missed past due", model.StatusMissed, &past, false},
		{"pending future", model.StatusPending, &future, false},
		{"pending no due", model.StatusPending, nil, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			task := &model.Task{Status: c.status, DueDate: c.due}
			if got := task.IsOverdue(now); got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}
