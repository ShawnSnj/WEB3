package service_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// In-memory fake repository
// ---------------------------------------------------------------------------

type fakeRepo struct {
	mu    sync.Mutex
	tasks map[uuid.UUID]*model.Task
	now   func() time.Time
}

func newFakeRepo(now func() time.Time) *fakeRepo {
	return &fakeRepo{tasks: map[uuid.UUID]*model.Task{}, now: now}
}

func (r *fakeRepo) Create(_ context.Context, t *model.Task) error {
	if err := t.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.ID = uuid.New()
	now := r.now()
	t.CreatedAt = now
	t.UpdatedAt = now
	cp := *t
	r.tasks[t.ID] = &cp
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}

func (r *fakeRepo) Update(_ context.Context, id uuid.UUID, u repository.TaskUpdate) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	if u.Title != nil {
		t.Title = *u.Title
	}
	if u.Description != nil {
		t.Description = *u.Description
	}
	if u.Priority != nil {
		t.Priority = *u.Priority
	}
	if u.Category != nil {
		t.Category = *u.Category
	}
	if u.Status != nil {
		t.Status = *u.Status
	}
	if u.EstimatedMinutes != nil {
		t.EstimatedMinutes = *u.EstimatedMinutes
	}
	if u.ActualMinutes != nil {
		t.ActualMinutes = *u.ActualMinutes
	}
	switch {
	case u.ClearDueDate:
		t.DueDate = nil
	case u.DueDate != nil:
		v := *u.DueDate
		t.DueDate = &v
	}
	if u.CarryOverCount != nil {
		t.CarryOverCount = *u.CarryOverCount
	}
	switch {
	case u.ClearCompletedAt:
		t.CompletedAt = nil
	case u.CompletedAt != nil:
		v := *u.CompletedAt
		t.CompletedAt = &v
	}
	t.UpdatedAt = r.now()
	cp := *t
	return &cp, nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return model.ErrTaskNotFound
	}
	delete(r.tasks, id)
	return nil
}

func (r *fakeRepo) List(_ context.Context, f repository.TaskFilter) ([]*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Task, 0, len(r.tasks))
	for _, t := range r.tasks {
		if !matchFilter(t, f, r.now()) {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *fakeRepo) ListOverdue(_ context.Context, before time.Time) ([]*model.Task, error) {
	return r.List(context.Background(), repository.TaskFilter{
		Statuses:  []model.Status{model.StatusPending, model.StatusInProgress},
		DueBefore: &before,
	})
}

func matchFilter(t *model.Task, f repository.TaskFilter, now time.Time) bool {
	if len(f.Statuses) > 0 && !containsStatus(f.Statuses, t.Status) {
		return false
	}
	if len(f.Categories) > 0 && !containsCategory(f.Categories, t.Category) {
		return false
	}
	if f.DueBefore != nil && (t.DueDate == nil || !t.DueDate.Before(*f.DueBefore)) {
		return false
	}
	if f.DueAfter != nil && (t.DueDate == nil || t.DueDate.Before(*f.DueAfter)) {
		return false
	}
	if f.Title != nil && !strings.EqualFold(strings.TrimSpace(t.Title), strings.TrimSpace(*f.Title)) {
		return false
	}
	if f.OnlyOverdue && !t.IsOverdue(now) {
		return false
	}
	if f.CarriedOver != nil {
		if *f.CarriedOver && t.CarryOverCount == 0 {
			return false
		}
		if !*f.CarriedOver && t.CarryOverCount > 0 {
			return false
		}
	}
	return true
}

func containsStatus(s []model.Status, v model.Status) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
func containsCategory(s []model.Category, v model.Category) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Frozen clock
// ---------------------------------------------------------------------------

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func newSvc(t *testing.T) (*service.TaskService, *fakeRepo, *fixedClock) {
	t.Helper()
	clk := &fixedClock{t: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)}
	repo := newFakeRepo(clk.Now)
	return service.NewTaskService(repo, clk, nil), repo, clk
}

func TestService_Create_DefaultsAndValidation(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)
	ctx := context.Background()

	got, err := svc.Create(ctx, service.CreateTaskInput{Title: "  Apply to Acme  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Apply to Acme" {
		t.Errorf("title not trimmed: %q", got.Title)
	}
	if got.Priority != model.PriorityMedium {
		t.Errorf("default priority should be medium, got %q", got.Priority)
	}
	if got.Category != model.CategoryMisc {
		t.Errorf("default category should be misc, got %q", got.Category)
	}
	if got.Status != model.StatusPending {
		t.Errorf("default status should be pending, got %q", got.Status)
	}

	if _, err := svc.Create(ctx, service.CreateTaskInput{Title: ""}); !errors.Is(err, model.ErrTitleRequired) {
		t.Errorf("want ErrTitleRequired, got %v", err)
	}
	if _, err := svc.Create(ctx, service.CreateTaskInput{Title: "x", Priority: "bogus"}); !errors.Is(err, model.ErrInvalidPriority) {
		t.Errorf("want ErrInvalidPriority, got %v", err)
	}
}

func TestService_StateMachine(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	t1, _ := svc.Create(ctx, service.CreateTaskInput{Title: "Read paper"})

	// pending -> in_progress
	t2, err := svc.MarkInProgress(ctx, t1.ID)
	if err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if t2.Status != model.StatusInProgress {
		t.Errorf("status got %q want in_progress", t2.Status)
	}

	// in_progress -> pending (cancel)
	t2b, err := svc.MarkPending(ctx, t1.ID)
	if err != nil {
		t.Fatalf("MarkPending: %v", err)
	}
	if t2b.Status != model.StatusPending {
		t.Errorf("status got %q want pending", t2b.Status)
	}

	// pending -> in_progress again
	t2, err = svc.MarkInProgress(ctx, t1.ID)
	if err != nil {
		t.Fatalf("MarkInProgress again: %v", err)
	}
	if t2.Status != model.StatusInProgress {
		t.Errorf("status got %q want in_progress", t2.Status)
	}

	// in_progress -> completed with actual_minutes
	clk.t = clk.t.Add(30 * time.Minute)
	t3, err := svc.MarkCompleted(ctx, t1.ID, 25)
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if t3.Status != model.StatusCompleted {
		t.Errorf("status got %q want completed", t3.Status)
	}
	if t3.ActualMinutes != 25 {
		t.Errorf("actual_minutes got %d want 25", t3.ActualMinutes)
	}
	if t3.CompletedAt == nil || !t3.CompletedAt.Equal(clk.Now()) {
		t.Errorf("CompletedAt = %v, want %v", t3.CompletedAt, clk.Now())
	}

	// completed -> anywhere is rejected
	if _, err := svc.MarkInProgress(ctx, t1.ID); !errors.Is(err, model.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestService_SetStatus_AllowsAnyValidChange(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	t1, _ := svc.Create(ctx, service.CreateTaskInput{Title: "Reopen me"})
	_, _ = svc.MarkCompleted(ctx, t1.ID, 10)

	reopened, err := svc.SetStatus(ctx, t1.ID, model.StatusPending)
	if err != nil {
		t.Fatalf("SetStatus pending: %v", err)
	}
	if reopened.Status != model.StatusPending {
		t.Errorf("status = %q, want pending", reopened.Status)
	}
	if reopened.CompletedAt != nil {
		t.Errorf("CompletedAt should be cleared, got %v", reopened.CompletedAt)
	}

	missed, err := svc.SetStatus(ctx, t1.ID, model.StatusMissed)
	if err != nil {
		t.Fatalf("SetStatus missed: %v", err)
	}
	if missed.Status != model.StatusMissed {
		t.Errorf("status = %q, want missed", missed.Status)
	}

	done, err := svc.SetStatus(ctx, t1.ID, model.StatusCompleted)
	if err != nil {
		t.Fatalf("SetStatus completed: %v", err)
	}
	if done.Status != model.StatusCompleted {
		t.Errorf("status = %q, want completed", done.Status)
	}
	if done.CompletedAt == nil || !done.CompletedAt.Equal(clk.Now()) {
		t.Errorf("CompletedAt = %v, want %v", done.CompletedAt, clk.Now())
	}
}

func TestService_Update_RejectsBadValuesAndSkipsNil(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)
	ctx := context.Background()

	t1, _ := svc.Create(ctx, service.CreateTaskInput{Title: "Initial"})

	bad := "garbage"
	badPriority := model.Priority(bad)
	if _, err := svc.Update(ctx, t1.ID, service.UpdateTaskInput{Priority: &badPriority}); !errors.Is(err, model.ErrInvalidPriority) {
		t.Errorf("want ErrInvalidPriority, got %v", err)
	}

	blank := "   "
	if _, err := svc.Update(ctx, t1.ID, service.UpdateTaskInput{Title: &blank}); !errors.Is(err, model.ErrTitleRequired) {
		t.Errorf("want ErrTitleRequired, got %v", err)
	}

	newTitle := "Updated title"
	updated, err := svc.Update(ctx, t1.ID, service.UpdateTaskInput{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Updated title" {
		t.Errorf("got %q", updated.Title)
	}
}

func TestService_MarkCompleted_FromMissed(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	t1, err := svc.Create(ctx, service.CreateTaskInput{Title: "Catch up"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkMissed(ctx, t1.ID); err != nil {
		t.Fatal(err)
	}
	done, err := svc.MarkCompleted(ctx, t1.ID, 15)
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if done.Status != model.StatusCompleted {
		t.Errorf("status = %q, want completed", done.Status)
	}
	if done.ActualMinutes != 15 {
		t.Errorf("actual = %d, want 15", done.ActualMinutes)
	}
	if done.CompletedAt == nil || !done.CompletedAt.Equal(clk.Now()) {
		t.Errorf("CompletedAt = %v", done.CompletedAt)
	}
}

func TestService_CarryOver_ReschedulesMissed(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	past := clk.Now().Add(-48 * time.Hour)
	t1, err := svc.Create(ctx, service.CreateTaskInput{Title: "Retry me", DueDate: &past})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkMissed(ctx, t1.ID); err != nil {
		t.Fatal(err)
	}

	out, err := svc.CarryOverTask(ctx, t1.ID)
	if err != nil {
		t.Fatalf("CarryOverTask: %v", err)
	}
	if out.ID != t1.ID {
		t.Errorf("want same task id, got new %s", out.ID)
	}
	if out.Status != model.StatusPending {
		t.Errorf("status = %q, want pending", out.Status)
	}
	if out.Priority != model.PriorityHigh {
		t.Errorf("priority = %q, want bumped high", out.Priority)
	}
}

func TestService_CarryOver_BumpsPriorityAndCount(t *testing.T) {
	t.Parallel()
	svc, repo, clk := newSvc(t)
	ctx := context.Background()

	due := clk.Now().Add(-25 * time.Hour) // calendar day before today
	t1, err := svc.Create(ctx, service.CreateTaskInput{
		Title:    "Apply to BigCo",
		Priority: model.PriorityMedium,
		DueDate:  &due,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	nt, err := svc.CarryOverTask(ctx, t1.ID)
	if err != nil {
		t.Fatalf("CarryOverTask: %v", err)
	}
	if nt.Priority != model.PriorityHigh {
		t.Errorf("priority bump failed: %q", nt.Priority)
	}
	if nt.CarryOverCount != 1 {
		t.Errorf("carry_over_count want 1 got %d", nt.CarryOverCount)
	}
	if nt.Status != model.StatusPending {
		t.Errorf("new task should be pending, got %q", nt.Status)
	}
	wantDue := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC) // today (catch up overdue)
	if nt.DueDate == nil || !nt.DueDate.Equal(wantDue) {
		t.Errorf("due_date want %v, got %v", wantDue, nt.DueDate)
	}

	// Original is now missed.
	src, _ := repo.GetByID(ctx, t1.ID)
	if src.Status != model.StatusMissed {
		t.Errorf("source status want missed, got %q", src.Status)
	}

	// A missed source can be rescheduled again (reopen + roll due date).
	rescheduled, err := svc.CarryOverTask(ctx, t1.ID)
	if err != nil {
		t.Fatalf("reschedule missed: %v", err)
	}
	if rescheduled.Status != model.StatusPending {
		t.Errorf("rescheduled status = %q, want pending", rescheduled.Status)
	}

	// And a completed task cannot be carried over.
	t2, _ := svc.Create(ctx, service.CreateTaskInput{Title: "done"})
	_, _ = svc.MarkCompleted(ctx, t2.ID, 0)
	if _, err := svc.CarryOverTask(ctx, t2.ID); !errors.Is(err, model.ErrTaskNotEligibleCarry) {
		t.Errorf("want ErrTaskNotEligibleCarry, got %v", err)
	}
}

func TestService_CarryOver_CapsAtUrgent(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()
	due := clk.Now().Add(-time.Hour)

	t1, _ := svc.Create(ctx, service.CreateTaskInput{
		Title:    "Important",
		Priority: model.PriorityUrgent,
		DueDate:  &due,
	})
	nt, err := svc.CarryOverTask(ctx, t1.ID)
	if err != nil {
		t.Fatalf("CarryOverTask: %v", err)
	}
	if nt.Priority != model.PriorityUrgent {
		t.Errorf("urgent should cap, got %q", nt.Priority)
	}
}

func TestService_CarryOver_DeeplyOverdueDueIsToday(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	due := clk.Now().Add(-5 * 24 * time.Hour)
	t1, err := svc.Create(ctx, service.CreateTaskInput{
		Title:    "Stale apply",
		Priority: model.PriorityMedium,
		DueDate:  &due,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	nt, err := svc.CarryOverTask(ctx, t1.ID)
	if err != nil {
		t.Fatalf("CarryOverTask: %v", err)
	}
	wantDue := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	if nt.DueDate == nil || !nt.DueDate.Equal(wantDue) {
		t.Errorf("deeply overdue carry should land today, want %v got %v", wantDue, nt.DueDate)
	}
}

func TestService_CarryOverAllOverdue_DedupesSamePlanSlot(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	due := clk.Now().Add(-48 * time.Hour)
	_, err := svc.Create(ctx, service.CreateTaskInput{Title: "Dup plan", DueDate: &due})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	_, err = svc.Create(ctx, service.CreateTaskInput{Title: "Dup plan", DueDate: &due})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	created, errs := svc.CarryOverAllOverdue(ctx)
	if len(errs) != 0 {
		t.Fatalf("carry errors: %v", errs)
	}
	if len(created) != 1 {
		t.Fatalf("want 1 successor, got %d", len(created))
	}

	all, err := svc.List(ctx, service.ListTasksInput{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var pending, missed int
	for _, t := range all {
		if t.Title != "Dup plan" {
			continue
		}
		switch t.Status {
		case model.StatusPending:
			pending++
		case model.StatusMissed:
			missed++
		}
	}
	if pending != 1 {
		t.Errorf("pending successors = %d, want 1", pending)
	}
	if missed != 2 {
		t.Errorf("missed sources = %d, want 2", missed)
	}
}

func TestService_CollapseDuplicatePendingByTitle(t *testing.T) {
	t.Parallel()
	svc, repo, clk := newSvc(t)
	ctx := context.Background()

	jun20 := clk.Now().Add(6 * 24 * time.Hour)
	jun24 := clk.Now().Add(10 * 24 * time.Hour)

	_, _ = svc.Create(ctx, service.CreateTaskInput{Title: "Weekly review", DueDate: &jun20})
	time.Sleep(2 * time.Millisecond)
	dup, _ := svc.Create(ctx, service.CreateTaskInput{Title: "Weekly review", DueDate: &jun24})
	dup.CarryOverCount = 1
	_, _ = repo.Update(ctx, dup.ID, repository.TaskUpdate{CarryOverCount: &dup.CarryOverCount})

	removed, err := svc.CollapseDuplicatePendingByTitle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if len(repo.tasks) != 1 {
		t.Fatalf("tasks left = %d, want 1", len(repo.tasks))
	}
	for _, tk := range repo.tasks {
		if tk.CarryOverCount != 0 {
			t.Fatalf("keeper should have carry_over_count 0, got %d", tk.CarryOverCount)
		}
	}
}

func TestService_CollapseDuplicatePlans(t *testing.T) {
	t.Parallel()
	svc, repo, clk := newSvc(t)
	ctx := context.Background()
	due := clk.Now()

	_, _ = svc.Create(ctx, service.CreateTaskInput{Title: "Collapse me", DueDate: &due})
	time.Sleep(2 * time.Millisecond)
	_, _ = svc.Create(ctx, service.CreateTaskInput{Title: "Collapse me", DueDate: &due})

	removed, err := svc.CollapseDuplicatePlans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if len(repo.tasks) != 1 {
		t.Fatalf("tasks left = %d, want 1", len(repo.tasks))
	}
}

func TestService_CarryOverAllOverdue_SkipsRunningSession(t *testing.T) {
	t.Parallel()
	svc, repo, clk := newSvc(t)
	ctx := context.Background()

	due := clk.Now().Add(-48 * time.Hour)
	_, err := svc.Create(ctx, service.CreateTaskInput{Title: "Timed", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	var taskID uuid.UUID
	for id := range repo.tasks {
		taskID = id
	}
	svc.SetSessionGuard(sessionGuardStub{running: map[uuid.UUID]bool{taskID: true}})

	created, errs := svc.CarryOverAllOverdue(ctx)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(created) != 0 {
		t.Fatalf("want 0 created while session runs, got %d", len(created))
	}
	got, _ := svc.Get(ctx, taskID)
	if got.Status != model.StatusPending {
		t.Errorf("status = %s, want pending (unchanged)", got.Status)
	}
}

type sessionGuardStub struct {
	running map[uuid.UUID]bool
}

func (s sessionGuardStub) HasRunningSession(_ context.Context, id uuid.UUID) (bool, error) {
	return s.running[id], nil
}

func TestService_CarryOverAllOverdue_NoDuplicateChain(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	due := clk.Now().Add(-5 * 24 * time.Hour)
	_, err := svc.Create(ctx, service.CreateTaskInput{Title: "Chain me", DueDate: &due})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	first, errs := svc.CarryOverAllOverdue(ctx)
	if len(errs) != 0 {
		t.Fatalf("first sweep errors: %v", errs)
	}
	if len(first) != 1 {
		t.Fatalf("first sweep want 1 created, got %d", len(first))
	}

	second, errs := svc.CarryOverAllOverdue(ctx)
	if len(errs) != 0 {
		t.Fatalf("second sweep errors: %v", errs)
	}
	if len(second) != 0 {
		t.Errorf("second sweep should not re-carry the successor, got %d new tasks", len(second))
	}
}

func TestService_CarryOverAllOverdue(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	overdue := clk.Now().Add(-48 * time.Hour)
	future := clk.Now().Add(48 * time.Hour)

	// 2 overdue (due before today), 1 not-yet-due, 1 completed past-due (skipped).
	_, _ = svc.Create(ctx, service.CreateTaskInput{Title: "A", DueDate: &overdue})
	_, _ = svc.Create(ctx, service.CreateTaskInput{Title: "B", DueDate: &overdue})
	_, _ = svc.Create(ctx, service.CreateTaskInput{Title: "C", DueDate: &future})
	doneTask, _ := svc.Create(ctx, service.CreateTaskInput{Title: "D", DueDate: &overdue})
	_, _ = svc.MarkCompleted(ctx, doneTask.ID, 0)

	created, errs := svc.CarryOverAllOverdue(ctx)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(created) != 2 {
		t.Errorf("want 2 new tasks, got %d", len(created))
	}
	for _, nt := range created {
		if nt.CarryOverCount != 1 {
			t.Errorf("carry_over_count want 1, got %d", nt.CarryOverCount)
		}
	}
}

func TestService_ListOverdue_FiltersTerminal(t *testing.T) {
	t.Parallel()
	svc, _, clk := newSvc(t)
	ctx := context.Background()

	past := clk.Now().Add(-48 * time.Hour)
	a, _ := svc.Create(ctx, service.CreateTaskInput{Title: "live", DueDate: &past})
	b, _ := svc.Create(ctx, service.CreateTaskInput{Title: "done", DueDate: &past})
	_, _ = svc.MarkCompleted(ctx, b.ID, 0)

	got, err := svc.ListOverdue(ctx)
	if err != nil {
		t.Fatalf("ListOverdue: %v", err)
	}
	if len(got) != 1 || got[0].ID != a.ID {
		t.Errorf("want only live task, got %+v", got)
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)
	if err := svc.Delete(context.Background(), uuid.New()); !errors.Is(err, model.ErrTaskNotFound) {
		t.Errorf("want ErrTaskNotFound, got %v", err)
	}
}
