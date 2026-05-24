package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Fake session repo
// ---------------------------------------------------------------------------

type fakeSessionRepo struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*model.TaskSession
	now      func() time.Time
}

func newFakeSessionRepo(now func() time.Time) *fakeSessionRepo {
	return &fakeSessionRepo{
		sessions: map[uuid.UUID]*model.TaskSession{},
		now:      now,
	}
}

func (r *fakeSessionRepo) Create(_ context.Context, s *model.TaskSession) error {
	if err := s.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Enforce "at most one running session per task" — matches the DB index.
	for _, ex := range r.sessions {
		if ex.TaskID == s.TaskID && ex.Status.IsRunning() {
			return model.ErrSessionAlreadyRunning
		}
	}
	s.ID = uuid.New()
	now := r.now()
	s.CreatedAt = now
	s.UpdatedAt = now
	cp := *s
	r.sessions[s.ID] = &cp
	return nil
}

func (r *fakeSessionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, model.ErrSessionNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *fakeSessionRepo) Update(_ context.Context, id uuid.UUID, u repository.SessionUpdate) (*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, model.ErrSessionNotFound
	}
	if u.Status != nil {
		s.Status = *u.Status
	}
	switch {
	case u.ClearEndedAt:
		s.EndedAt = nil
	case u.EndedAt != nil:
		v := *u.EndedAt
		s.EndedAt = &v
	}
	switch {
	case u.ClearPausedAt:
		s.PausedAt = nil
	case u.PausedAt != nil:
		v := *u.PausedAt
		s.PausedAt = &v
	}
	if u.TotalPausedSeconds != nil {
		s.TotalPausedSeconds = *u.TotalPausedSeconds
	}
	if u.Interruptions != nil {
		s.Interruptions = *u.Interruptions
	}
	if u.CompletionQuality != nil {
		s.CompletionQuality = *u.CompletionQuality
	}
	if u.Notes != nil {
		s.Notes = *u.Notes
	}
	s.UpdatedAt = r.now()
	cp := *s
	return &cp, nil
}

func (r *fakeSessionRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[id]; !ok {
		return model.ErrSessionNotFound
	}
	delete(r.sessions, id)
	return nil
}

func (r *fakeSessionRepo) List(_ context.Context, f repository.SessionFilter) ([]*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.TaskSession, 0)
	for _, s := range r.sessions {
		if f.TaskID != nil && s.TaskID != *f.TaskID {
			continue
		}
		if len(f.Statuses) > 0 && !containsSessionStatus(f.Statuses, s.Status) {
			continue
		}
		cp := *s
		out = append(out, &cp)
	}
	return out, nil
}

func containsSessionStatus(s []model.SessionStatus, v model.SessionStatus) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (r *fakeSessionRepo) FindRunningByTask(_ context.Context, taskID uuid.UUID) (*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if s.TaskID == taskID && s.Status.IsRunning() {
			cp := *s
			return &cp, nil
		}
	}
	return nil, model.ErrSessionNotFound
}

func (r *fakeSessionRepo) SumEffectiveMinutesByTask(_ context.Context, taskID uuid.UUID, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	total := 0
	for _, s := range r.sessions {
		if s.TaskID == taskID {
			total += s.EffectiveMinutes(now)
		}
	}
	return total, nil
}

func (r *fakeSessionRepo) SumEffectiveMinutesInRange(_ context.Context, from, to, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	total := 0
	for _, s := range r.sessions {
		if !s.StartedAt.Before(from) && s.StartedAt.Before(to) {
			total += s.EffectiveMinutes(now)
		}
	}
	return total, nil
}

// ---------------------------------------------------------------------------
// Fake task completer
// ---------------------------------------------------------------------------

type fakeTaskCompleter struct {
	mu        sync.Mutex
	tasks     map[uuid.UUID]*model.Task
	completed []completeCall
}

type completeCall struct {
	ID            uuid.UUID
	ActualMinutes int
}

func newFakeTaskCompleter(seed ...*model.Task) *fakeTaskCompleter {
	tc := &fakeTaskCompleter{tasks: map[uuid.UUID]*model.Task{}}
	for _, t := range seed {
		tc.tasks[t.ID] = t
	}
	return tc
}

func (f *fakeTaskCompleter) Get(_ context.Context, id uuid.UUID) (*model.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}

func (f *fakeTaskCompleter) MarkCompleted(_ context.Context, id uuid.UUID, actualMinutes int) (*model.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	t.Status = model.StatusCompleted
	t.ActualMinutes = actualMinutes
	f.completed = append(f.completed, completeCall{ID: id, ActualMinutes: actualMinutes})
	cp := *t
	return &cp, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func newSessionSvc(t *testing.T) (*service.TaskSessionService, *fakeSessionRepo, *fakeTaskCompleter, *fixedClock, *model.Task) {
	t.Helper()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)}
	task := &model.Task{
		ID:       uuid.New(),
		Title:    "Apply",
		Status:   model.StatusPending,
		Priority: model.PriorityMedium,
		Category: model.CategoryJobApply,
	}
	tc := newFakeTaskCompleter(task)
	repo := newFakeSessionRepo(clk.Now)
	svc := service.NewTaskSessionService(repo, tc, clk)
	return svc, repo, tc, clk, task
}

func TestSessionService_StartRejectsDuplicate(t *testing.T) {
	t.Parallel()
	svc, _, _, _, task := newSessionSvc(t)
	ctx := context.Background()

	if _, err := svc.Start(ctx, task.ID); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if _, err := svc.Start(ctx, task.ID); !errors.Is(err, model.ErrSessionAlreadyRunning) {
		t.Errorf("want ErrSessionAlreadyRunning, got %v", err)
	}
}

func TestSessionService_StartRejectsTerminalTask(t *testing.T) {
	t.Parallel()
	svc, _, tc, _, task := newSessionSvc(t)
	tc.tasks[task.ID].Status = model.StatusCompleted

	if _, err := svc.Start(context.Background(), task.ID); !errors.Is(err, model.ErrInvalidTransition) {
		t.Errorf("want ErrInvalidTransition, got %v", err)
	}
}

func TestSessionService_PauseResumeMath(t *testing.T) {
	t.Parallel()
	svc, _, _, clk, task := newSessionSvc(t)
	ctx := context.Background()

	s, _ := svc.Start(ctx, task.ID) // t=10:00

	clk.t = clk.t.Add(20 * time.Minute) // worked 20 min
	s, err := svc.Pause(ctx, s.ID)
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if s.Status != model.SessionStatusPaused {
		t.Errorf("status = %s", s.Status)
	}

	clk.t = clk.t.Add(10 * time.Minute) // paused 10 min
	s, err = svc.Resume(ctx, s.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if s.Status != model.SessionStatusActive {
		t.Errorf("status = %s", s.Status)
	}
	if s.TotalPausedSeconds != 600 {
		t.Errorf("TotalPausedSeconds = %d, want 600", s.TotalPausedSeconds)
	}

	clk.t = clk.t.Add(15 * time.Minute) // worked another 15 min

	// At "now": started 10:00, now 10:45, paused total 10 min -> effective = 35 min
	if got := s.EffectiveMinutes(clk.Now()); got != 35 {
		t.Errorf("EffectiveMinutes(now) = %d, want 35", got)
	}
}

func TestSessionService_StopAccumulatesInFlightPause(t *testing.T) {
	t.Parallel()
	svc, _, _, clk, task := newSessionSvc(t)
	ctx := context.Background()

	s, _ := svc.Start(ctx, task.ID)
	clk.t = clk.t.Add(20 * time.Minute)
	s, _ = svc.Pause(ctx, s.ID)
	clk.t = clk.t.Add(5 * time.Minute) // still paused

	// Stop while paused: pause window should be accumulated.
	stopped, err := svc.Stop(ctx, s.ID, service.FinishSessionInput{
		Interruptions:     intPtr(2),
		CompletionQuality: intPtr(4),
		Notes:             strPtr("got distracted"),
	})
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stopped.Status != model.SessionStatusStopped {
		t.Errorf("status = %s", stopped.Status)
	}
	if stopped.TotalPausedSeconds != 5*60 {
		t.Errorf("TotalPausedSeconds = %d, want 300", stopped.TotalPausedSeconds)
	}
	if stopped.EffectiveMinutes(clk.Now()) != 20 {
		t.Errorf("effective minutes = %d, want 20", stopped.EffectiveMinutes(clk.Now()))
	}
	if stopped.Interruptions != 2 || stopped.CompletionQuality != 4 {
		t.Errorf("metadata not captured: %+v", stopped)
	}
}

func TestSessionService_CompleteMarksTask(t *testing.T) {
	t.Parallel()
	svc, _, tc, clk, task := newSessionSvc(t)
	ctx := context.Background()

	s, _ := svc.Start(ctx, task.ID)
	clk.t = clk.t.Add(45 * time.Minute)

	completed, err := svc.Complete(ctx, s.ID, service.FinishSessionInput{
		CompletionQuality: intPtr(5),
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if completed.Status != model.SessionStatusCompleted {
		t.Errorf("session status = %s", completed.Status)
	}

	if len(tc.completed) != 1 {
		t.Fatalf("expected 1 MarkCompleted call, got %d", len(tc.completed))
	}
	got := tc.completed[0]
	if got.ID != task.ID {
		t.Errorf("MarkCompleted called with wrong id: %v vs %v", got.ID, task.ID)
	}
	if got.ActualMinutes != 45 {
		t.Errorf("ActualMinutes = %d, want 45", got.ActualMinutes)
	}
	if tc.tasks[task.ID].Status != model.StatusCompleted {
		t.Errorf("task status not flipped: %s", tc.tasks[task.ID].Status)
	}
}

func TestSessionService_CompleteSumsAcrossSessions(t *testing.T) {
	t.Parallel()
	svc, _, tc, clk, task := newSessionSvc(t)
	ctx := context.Background()

	// First session: 30 min, stopped (not completed)
	s1, _ := svc.Start(ctx, task.ID)
	clk.t = clk.t.Add(30 * time.Minute)
	if _, err := svc.Stop(ctx, s1.ID, service.FinishSessionInput{}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Second session: 20 min, completed
	clk.t = clk.t.Add(5 * time.Minute) // gap
	s2, _ := svc.Start(ctx, task.ID)
	clk.t = clk.t.Add(20 * time.Minute)
	if _, err := svc.Complete(ctx, s2.ID, service.FinishSessionInput{}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if len(tc.completed) != 1 {
		t.Fatalf("expected 1 MarkCompleted call, got %d", len(tc.completed))
	}
	if tc.completed[0].ActualMinutes != 50 {
		t.Errorf("ActualMinutes = %d, want 50 (30+20)", tc.completed[0].ActualMinutes)
	}
}

func TestSessionService_InvalidTransitions(t *testing.T) {
	t.Parallel()
	svc, _, _, _, task := newSessionSvc(t)
	ctx := context.Background()

	s, _ := svc.Start(ctx, task.ID)
	if _, err := svc.Stop(ctx, s.ID, service.FinishSessionInput{}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	// stopped -> pause must fail
	if _, err := svc.Pause(ctx, s.ID); !errors.Is(err, model.ErrInvalidSessionTransition) {
		t.Errorf("want ErrInvalidSessionTransition, got %v", err)
	}
	// stopped -> complete must fail
	if _, err := svc.Complete(ctx, s.ID, service.FinishSessionInput{}); !errors.Is(err, model.ErrInvalidSessionTransition) {
		t.Errorf("want ErrInvalidSessionTransition, got %v", err)
	}
}
