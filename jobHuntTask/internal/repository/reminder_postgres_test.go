//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

func TestPostgres_Reminder_ScheduleDedup(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	})
	repo := repository.NewPostgresReminderRepository(pool)
	ctx := context.Background()

	key := "morning:2026-05-24"
	r1 := &model.Reminder{
		Kind: model.ReminderKindMorning, Status: model.ReminderStatusPending,
		DedupKey: &key, ScheduledFor: time.Now().UTC(),
		Payload: map[string]any{"message": "go"},
	}
	created, err := repo.Schedule(ctx, r1)
	if err != nil || !created {
		t.Fatalf("first: %v created=%v", err, created)
	}

	r2 := &model.Reminder{
		Kind: model.ReminderKindMorning, Status: model.ReminderStatusPending,
		DedupKey: &key, ScheduledFor: time.Now().UTC(),
	}
	created, err = repo.Schedule(ctx, r2)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if created {
		t.Error("second should have been deduped")
	}
	if r2.ID != r1.ID {
		t.Errorf("ID mismatch: %v vs %v", r2.ID, r1.ID)
	}
}

func TestPostgres_Reminder_ListDue(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	})
	repo := repository.NewPostgresReminderRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	mk := func(when time.Time, key string) {
		t.Helper()
		k := key
		r := &model.Reminder{
			Kind: model.ReminderKindMorning, Status: model.ReminderStatusPending,
			DedupKey: &k, ScheduledFor: when,
		}
		if _, err := repo.Schedule(ctx, r); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	mk(now.Add(-time.Hour), "past1")
	mk(now.Add(-time.Minute), "past2")
	mk(now.Add(time.Hour), "future")

	got, err := repo.ListDue(ctx, now, 10)
	if err != nil {
		t.Fatalf("ListDue: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 due, got %d", len(got))
	}
}

func TestPostgres_Reminder_MarkSentAndFailed(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	})
	repo := repository.NewPostgresReminderRepository(pool)
	ctx := context.Background()
	k := "k"
	r := &model.Reminder{
		Kind: model.ReminderKindMorning, Status: model.ReminderStatusPending,
		DedupKey: &k, ScheduledFor: time.Now().UTC(),
	}
	if _, err := repo.Schedule(ctx, r); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	failed, err := repo.MarkFailed(ctx, r.ID, time.Now().UTC(), 1, "boom")
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if failed.Status != model.ReminderStatusFailed || failed.LastError == nil {
		t.Errorf("MarkFailed shape: %+v", failed)
	}

	sent, err := repo.MarkSent(ctx, r.ID, time.Now().UTC(), 2)
	if err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if sent.Status != model.ReminderStatusSent || sent.SentAt == nil || sent.LastError != nil {
		t.Errorf("MarkSent shape: %+v", sent)
	}
}

func TestPostgres_Reminder_RequeueAndDelete(t *testing.T) {
	pool := newTestPool(t)
	_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE TABLE reminders`)
	})
	repo := repository.NewPostgresReminderRepository(pool)
	ctx := context.Background()
	k := "k2"
	r := &model.Reminder{
		Kind: model.ReminderKindMorning, Status: model.ReminderStatusPending,
		DedupKey: &k, ScheduledFor: time.Now().UTC(),
	}
	if _, err := repo.Schedule(ctx, r); err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	_, _ = repo.MarkFailed(ctx, r.ID, time.Now().UTC(), 1, "boom")
	requeued, err := repo.Requeue(ctx, r.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	if requeued.Status != model.ReminderStatusPending {
		t.Errorf("status after requeue: %s", requeued.Status)
	}
	if requeued.LastError != nil {
		t.Errorf("last_error not cleared: %v", *requeued.LastError)
	}

	if err := repo.Delete(ctx, r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, r.ID); !errors.Is(err, model.ErrReminderNotFound) {
		t.Errorf("after delete: want ErrReminderNotFound, got %v", err)
	}
}
