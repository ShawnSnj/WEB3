package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Fake reminder repo
// ---------------------------------------------------------------------------

type fakeReminderRepo struct {
	mu        sync.Mutex
	byID      map[uuid.UUID]*model.Reminder
	byDedup   map[string]uuid.UUID
	now       func() time.Time
}

func newFakeReminderRepo(now func() time.Time) *fakeReminderRepo {
	return &fakeReminderRepo{
		byID:    map[uuid.UUID]*model.Reminder{},
		byDedup: map[string]uuid.UUID{},
		now:     now,
	}
}

func (r *fakeReminderRepo) Schedule(_ context.Context, rem *model.Reminder) (bool, error) {
	if err := rem.Validate(); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if rem.DedupKey != nil {
		if id, ok := r.byDedup[*rem.DedupKey]; ok {
			existing := r.byID[id]
			*rem = *existing
			return false, nil
		}
	}
	rem.ID = uuid.New()
	now := r.now()
	rem.CreatedAt = now
	rem.UpdatedAt = now
	cp := *rem
	r.byID[rem.ID] = &cp
	if rem.DedupKey != nil {
		r.byDedup[*rem.DedupKey] = rem.ID
	}
	return true, nil
}
func (r *fakeReminderRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rem, ok := r.byID[id]
	if !ok {
		return nil, model.ErrReminderNotFound
	}
	cp := *rem
	return &cp, nil
}
func (r *fakeReminderRepo) ListDue(_ context.Context, now time.Time, limit int) ([]*model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*model.Reminder{}
	for _, rem := range r.byID {
		if (rem.Status == model.ReminderStatusPending || rem.Status == model.ReminderStatusFailed) &&
			!rem.ScheduledFor.After(now) {
			cp := *rem
			out = append(out, &cp)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (r *fakeReminderRepo) List(_ context.Context, _ repository.ReminderFilter) ([]*model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Reminder, 0, len(r.byID))
	for _, rem := range r.byID {
		cp := *rem
		out = append(out, &cp)
	}
	return out, nil
}
func (r *fakeReminderRepo) MarkSent(_ context.Context, id uuid.UUID, at time.Time, attempts int) (*model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rem, ok := r.byID[id]
	if !ok {
		return nil, model.ErrReminderNotFound
	}
	rem.Status = model.ReminderStatusSent
	t := at
	rem.SentAt = &t
	la := at
	rem.LastAttemptAt = &la
	rem.LastError = nil
	rem.Attempts = attempts
	rem.UpdatedAt = at
	cp := *rem
	return &cp, nil
}
func (r *fakeReminderRepo) MarkFailed(_ context.Context, id uuid.UUID, at time.Time, attempts int, msg string) (*model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rem, ok := r.byID[id]
	if !ok {
		return nil, model.ErrReminderNotFound
	}
	rem.Status = model.ReminderStatusFailed
	la := at
	rem.LastAttemptAt = &la
	m := msg
	rem.LastError = &m
	rem.Attempts = attempts
	rem.UpdatedAt = at
	cp := *rem
	return &cp, nil
}
func (r *fakeReminderRepo) MarkCancelled(_ context.Context, id uuid.UUID) (*model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rem, ok := r.byID[id]
	if !ok {
		return nil, model.ErrReminderNotFound
	}
	rem.Status = model.ReminderStatusCancelled
	rem.UpdatedAt = r.now()
	cp := *rem
	return &cp, nil
}
func (r *fakeReminderRepo) Requeue(_ context.Context, id uuid.UUID, sched time.Time) (*model.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rem, ok := r.byID[id]
	if !ok {
		return nil, model.ErrReminderNotFound
	}
	rem.Status = model.ReminderStatusPending
	rem.ScheduledFor = sched
	rem.LastError = nil
	rem.UpdatedAt = r.now()
	cp := *rem
	return &cp, nil
}
func (r *fakeReminderRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rem, ok := r.byID[id]
	if !ok {
		return model.ErrReminderNotFound
	}
	if rem.DedupKey != nil {
		delete(r.byDedup, *rem.DedupKey)
	}
	delete(r.byID, id)
	return nil
}

// ---------------------------------------------------------------------------
// Fake notifier
// ---------------------------------------------------------------------------

type fakeNotifier struct {
	mu     sync.Mutex
	calls  []*model.Reminder
	failOn map[uuid.UUID]int // id -> remaining failures
	err    error
}

func (n *fakeNotifier) Notify(_ context.Context, r *model.Reminder) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, r)
	if left, ok := n.failOn[r.ID]; ok && left > 0 {
		n.failOn[r.ID] = left - 1
		return n.err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

func newReminderSvc(t *testing.T, maxAttempts int) (*service.ReminderService, *fakeReminderRepo, *fakeNotifier, *fixedClock) {
	t.Helper()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	repo := newFakeReminderRepo(clk.Now)
	notif := &fakeNotifier{failOn: map[uuid.UUID]int{}, err: errors.New("notifier boom")}
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewReminderService(repo, notif, clk, silent, service.ReminderServiceConfig{
		MaxAttempts: maxAttempts,
		BatchSize:   100,
	})
	return svc, repo, notif, clk
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestReminderService_Schedule_DedupReturnsExisting(t *testing.T) {
	t.Parallel()
	svc, _, _, clk := newReminderSvc(t, 5)
	ctx := context.Background()

	r1, created1, err := svc.Schedule(ctx, service.ScheduleInput{
		Kind:         model.ReminderKindMorning,
		ScheduledFor: clk.Now(),
		DedupKey:     "morning:2026-05-24",
	})
	if err != nil || !created1 {
		t.Fatalf("first: err=%v created=%v", err, created1)
	}
	r2, created2, err := svc.Schedule(ctx, service.ScheduleInput{
		Kind:         model.ReminderKindMorning,
		ScheduledFor: clk.Now(),
		DedupKey:     "morning:2026-05-24",
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if created2 {
		t.Error("second insert should have been deduped")
	}
	if r1.ID != r2.ID {
		t.Errorf("dedup returned different ID: %v vs %v", r1.ID, r2.ID)
	}
}

func TestReminderService_DispatchDue_HappyPath(t *testing.T) {
	t.Parallel()
	svc, _, notif, clk := newReminderSvc(t, 5)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _, _ = svc.Schedule(ctx, service.ScheduleInput{
			Kind:         model.ReminderKindMorning,
			ScheduledFor: clk.Now(),
			DedupKey:     "k" + string(rune('a'+i)),
		})
	}
	res, err := svc.DispatchDue(ctx)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if res.Dispatched != 3 || res.Failed != 0 {
		t.Errorf("dispatch summary off: %+v", res)
	}
	if len(notif.calls) != 3 {
		t.Errorf("notifier got %d calls", len(notif.calls))
	}
}

func TestReminderService_DispatchDue_FailureGetsRetried(t *testing.T) {
	t.Parallel()
	svc, _, notif, clk := newReminderSvc(t, 5)
	ctx := context.Background()

	r, _, _ := svc.Schedule(ctx, service.ScheduleInput{
		Kind: model.ReminderKindMorning, ScheduledFor: clk.Now(), DedupKey: "k",
	})
	notif.failOn[r.ID] = 2 // fail twice, succeed on third

	// First tick: failure, status -> failed, attempts=1
	res, _ := svc.DispatchDue(ctx)
	if res.Failed != 1 || res.Dispatched != 0 {
		t.Errorf("tick1: %+v", res)
	}

	// Second tick: still pending|failed, so re-attempt. Failure again.
	res, _ = svc.DispatchDue(ctx)
	if res.Failed != 1 {
		t.Errorf("tick2: %+v", res)
	}

	// Third tick: notifier returns OK -> sent.
	res, _ = svc.DispatchDue(ctx)
	if res.Dispatched != 1 || res.Failed != 0 {
		t.Errorf("tick3: %+v", res)
	}

	// Final state should be sent with attempts=3.
	got, _ := svc.Get(ctx, r.ID)
	if got.Status != model.ReminderStatusSent {
		t.Errorf("final status: %s", got.Status)
	}
	if got.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", got.Attempts)
	}
}

func TestReminderService_DispatchDue_CancelsAfterMaxAttempts(t *testing.T) {
	t.Parallel()
	svc, _, notif, clk := newReminderSvc(t, 2)
	ctx := context.Background()

	r, _, _ := svc.Schedule(ctx, service.ScheduleInput{
		Kind: model.ReminderKindMorning, ScheduledFor: clk.Now(), DedupKey: "k",
	})
	notif.failOn[r.ID] = 99 // always fail

	_, _ = svc.DispatchDue(ctx) // attempt 1: failed
	_, _ = svc.DispatchDue(ctx) // attempt 2: failed (attempts=2 now == max)
	res, _ := svc.DispatchDue(ctx) // attempt 3 attempted? No — attempts>=max -> cancelled

	if res.Cancelled != 1 || res.MaxedOut != 1 {
		t.Errorf("expected cancellation, got %+v", res)
	}
	got, _ := svc.Get(ctx, r.ID)
	if got.Status != model.ReminderStatusCancelled {
		t.Errorf("final status: %s", got.Status)
	}
}

func TestReminderService_Retry_RearmsFailed(t *testing.T) {
	t.Parallel()
	svc, _, notif, clk := newReminderSvc(t, 5)
	ctx := context.Background()

	r, _, _ := svc.Schedule(ctx, service.ScheduleInput{
		Kind: model.ReminderKindMorning, ScheduledFor: clk.Now(), DedupKey: "k",
	})
	notif.failOn[r.ID] = 1
	_, _ = svc.DispatchDue(ctx) // -> failed

	clk.t = clk.t.Add(time.Hour)
	if _, err := svc.Retry(ctx, r.ID); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	got, _ := svc.Get(ctx, r.ID)
	if got.Status != model.ReminderStatusPending {
		t.Errorf("status after retry: %s", got.Status)
	}
}

func TestReminderService_Retry_RejectsSent(t *testing.T) {
	t.Parallel()
	svc, _, _, clk := newReminderSvc(t, 5)
	ctx := context.Background()

	r, _, _ := svc.Schedule(ctx, service.ScheduleInput{
		Kind: model.ReminderKindMorning, ScheduledFor: clk.Now(), DedupKey: "k",
	})
	_, _ = svc.DispatchDue(ctx) // ->sent

	if _, err := svc.Retry(ctx, r.ID); !errors.Is(err, model.ErrInvalidReminderTransition) {
		t.Errorf("want ErrInvalidReminderTransition, got %v", err)
	}
}

func TestReminderService_DispatchDue_SkipsFutureScheduled(t *testing.T) {
	t.Parallel()
	svc, _, notif, clk := newReminderSvc(t, 5)
	ctx := context.Background()

	future := clk.Now().Add(time.Hour)
	_, _, _ = svc.Schedule(ctx, service.ScheduleInput{
		Kind: model.ReminderKindMorning, ScheduledFor: future, DedupKey: "k",
	})
	res, err := svc.DispatchDue(ctx)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if res.Considered != 0 || res.Dispatched != 0 {
		t.Errorf("future reminder dispatched: %+v", res)
	}
	if len(notif.calls) != 0 {
		t.Error("notifier called for future reminder")
	}
}
