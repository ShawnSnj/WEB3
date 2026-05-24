package model_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

func TestReminderKind_IsValid(t *testing.T) {
	t.Parallel()
	for _, k := range []model.ReminderKind{
		model.ReminderKindMorning,
		model.ReminderKindEveningReview,
		model.ReminderKindWeeklyReview,
		model.ReminderKindOverdue,
		model.ReminderKindCustom,
	} {
		if !k.IsValid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if model.ReminderKind("bogus").IsValid() {
		t.Error("bogus kind should be invalid")
	}
}

func TestReminderStatus_CanTransitionTo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to model.ReminderStatus
		want     bool
	}{
		{model.ReminderStatusPending, model.ReminderStatusSent, true},
		{model.ReminderStatusPending, model.ReminderStatusFailed, true},
		{model.ReminderStatusPending, model.ReminderStatusCancelled, true},
		{model.ReminderStatusFailed, model.ReminderStatusPending, true},
		{model.ReminderStatusFailed, model.ReminderStatusSent, true},
		{model.ReminderStatusFailed, model.ReminderStatusCancelled, true},
		{model.ReminderStatusSent, model.ReminderStatusPending, false},
		{model.ReminderStatusSent, model.ReminderStatusFailed, false},
		{model.ReminderStatusCancelled, model.ReminderStatusPending, false},
		{model.ReminderStatusPending, model.ReminderStatusPending, true},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.want {
			t.Errorf("%s -> %s: got %v want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestDedupKeys(t *testing.T) {
	t.Parallel()
	d := time.Date(2026, 5, 24, 14, 15, 0, 0, time.UTC)

	if got := model.DedupKeyDaily(model.ReminderKindMorning, d); got != "morning:2026-05-24" {
		t.Errorf("daily key: %q", got)
	}
	if got := model.DedupKeyWeekly(model.ReminderKindWeeklyReview, d); !strings.HasPrefix(got, "weekly_review:2026-W") {
		t.Errorf("weekly key: %q", got)
	}
	tid := uuid.New()
	want := "overdue:" + tid.String() + ":2026-05-24"
	if got := model.DedupKeyOverdueTask(tid, d); got != want {
		t.Errorf("overdue key: %q want %q", got, want)
	}
}

func TestReminder_Validate(t *testing.T) {
	t.Parallel()
	base := func() *model.Reminder {
		return &model.Reminder{
			Kind:   model.ReminderKindMorning,
			Status: model.ReminderStatusPending,
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("base should be valid: %v", err)
	}

	r := base()
	r.Kind = "nope"
	if err := r.Validate(); err != model.ErrInvalidReminderKind {
		t.Errorf("kind: %v", err)
	}
	r = base()
	r.Status = "nope"
	if err := r.Validate(); err != model.ErrInvalidReminderStatus {
		t.Errorf("status: %v", err)
	}
	r = base()
	r.Attempts = -1
	if err := r.Validate(); err == nil {
		t.Error("negative attempts should fail")
	}
}
