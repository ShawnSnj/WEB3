package model_test

import (
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

func TestSessionStatus_IsValid(t *testing.T) {
	t.Parallel()
	for _, s := range []model.SessionStatus{
		model.SessionStatusActive,
		model.SessionStatusPaused,
		model.SessionStatusStopped,
		model.SessionStatusCompleted,
	} {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if model.SessionStatus("garbage").IsValid() {
		t.Error("garbage should be invalid")
	}
}

func TestSessionStatus_CanTransitionTo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to model.SessionStatus
		want     bool
	}{
		{model.SessionStatusActive, model.SessionStatusPaused, true},
		{model.SessionStatusActive, model.SessionStatusStopped, true},
		{model.SessionStatusActive, model.SessionStatusCompleted, true},
		{model.SessionStatusPaused, model.SessionStatusActive, true},
		{model.SessionStatusPaused, model.SessionStatusStopped, true},
		{model.SessionStatusPaused, model.SessionStatusCompleted, true},
		{model.SessionStatusStopped, model.SessionStatusActive, false},
		{model.SessionStatusCompleted, model.SessionStatusActive, false},
		{model.SessionStatusActive, model.SessionStatusActive, true},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.want {
			t.Errorf("%s -> %s: got %v want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestSession_EffectiveSeconds(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	now := start.Add(60 * time.Minute) // 60 min wall-clock

	t.Run("active full window", func(t *testing.T) {
		s := &model.TaskSession{
			Status:    model.SessionStatusActive,
			StartedAt: start,
		}
		if got := s.EffectiveSeconds(now); got != 3600 {
			t.Errorf("got %d, want 3600", got)
		}
	})

	t.Run("subtracts past pauses", func(t *testing.T) {
		s := &model.TaskSession{
			Status:             model.SessionStatusActive,
			StartedAt:          start,
			TotalPausedSeconds: 600, // 10 minutes paused earlier
		}
		if got := s.EffectiveSeconds(now); got != 3000 {
			t.Errorf("got %d, want 3000", got)
		}
	})

	t.Run("currently paused subtracts in-flight pause", func(t *testing.T) {
		pausedAt := start.Add(40 * time.Minute) // last 20 min are paused
		s := &model.TaskSession{
			Status:    model.SessionStatusPaused,
			StartedAt: start,
			PausedAt:  &pausedAt,
		}
		// effective = 60min - 20min (in-flight pause) = 40min
		if got := s.EffectiveSeconds(now); got != 40*60 {
			t.Errorf("got %d, want %d", got, 40*60)
		}
	})

	t.Run("ended session uses ended_at not now", func(t *testing.T) {
		ended := start.Add(30 * time.Minute)
		s := &model.TaskSession{
			Status:    model.SessionStatusStopped,
			StartedAt: start,
			EndedAt:   &ended,
		}
		if got := s.EffectiveSeconds(now.Add(10 * time.Hour)); got != 30*60 {
			t.Errorf("got %d, want %d", got, 30*60)
		}
	})

	t.Run("never goes negative", func(t *testing.T) {
		s := &model.TaskSession{
			Status:             model.SessionStatusActive,
			StartedAt:          start,
			TotalPausedSeconds: 99999, // absurd
		}
		if got := s.EffectiveSeconds(now); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})
}
