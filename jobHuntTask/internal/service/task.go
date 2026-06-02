// Package service holds the business-logic layer. Services are the
// single place that may:
//   - validate cross-field invariants
//   - orchestrate multiple repository calls
//   - apply domain rules (state machines, carry-over, etc.)
//
// Services depend on repository INTERFACES, not concrete implementations,
// which keeps unit tests fast and database-free.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// Clock abstracts time.Now so business rules that depend on "now" (carry
// over, overdue checks) can be deterministically tested.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// SystemClock is the default real-time clock implementation.
var SystemClock Clock = realClock{}

// ---------------------------------------------------------------------------
// Request value types
// ---------------------------------------------------------------------------

// CreateTaskInput is what the HTTP handler hands to the service after
// decoding a request. It is intentionally separate from any DTO so the
// service stays HTTP-agnostic.
type CreateTaskInput struct {
	Title            string
	Description      string
	Priority         model.Priority
	Category         model.Category
	EstimatedMinutes int
	DueDate          *time.Time
}

// UpdateTaskInput is the partial update analog. Nil pointer fields mean
// "don't touch this column". DueDate has an explicit Clear flag because
// nil already means "no change".
type UpdateTaskInput struct {
	Title            *string
	Description      *string
	Priority         *model.Priority
	Category         *model.Category
	EstimatedMinutes *int
	DueDate          *time.Time
	ClearDueDate     bool
}

// ListTasksInput mirrors repository.TaskFilter but stays in service-land.
type ListTasksInput = repository.TaskFilter

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// SessionGuard lets the task service respect in-flight work timers.
type SessionGuard interface {
	HasRunningSession(ctx context.Context, taskID uuid.UUID) (bool, error)
}

// TaskService implements all business rules for tasks.
type TaskService struct {
	repo     repository.TaskRepository
	clock    Clock
	cal      *calendar.Calendar
	sessions SessionGuard // optional: skips carry-over while a timer runs
}

// NewTaskService constructs a service. Pass SystemClock for production.
// When cal is nil, UTC calendar-day boundaries are used.
func NewTaskService(repo repository.TaskRepository, clock Clock, cal *calendar.Calendar) *TaskService {
	if clock == nil {
		clock = SystemClock
	}
	if cal == nil {
		cal = calendar.UTC()
	}
	return &TaskService{repo: repo, clock: clock, cal: cal}
}

// SetSessionGuard attaches session awareness (e.g. auto carry-over skips timed tasks).
func (s *TaskService) SetSessionGuard(g SessionGuard) {
	s.sessions = g
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create builds and persists a new task. Defaults are applied for any
// fields the caller left blank.
func (s *TaskService) Create(ctx context.Context, in CreateTaskInput) (*model.Task, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, model.ErrTitleRequired
	}
	if in.Priority == "" {
		in.Priority = model.PriorityMedium
	}
	if in.Category == "" {
		in.Category = model.CategoryMisc
	}
	if !in.Priority.IsValid() {
		return nil, model.ErrInvalidPriority
	}
	if !in.Category.IsValid() {
		return nil, model.ErrInvalidCategory
	}
	if in.EstimatedMinutes < 0 {
		return nil, model.ErrEstimatedNegative
	}

	t := &model.Task{
		Title:            strings.TrimSpace(in.Title),
		Description:      strings.TrimSpace(in.Description),
		Priority:         in.Priority,
		Category:         in.Category,
		Status:           model.StatusPending,
		EstimatedMinutes: in.EstimatedMinutes,
		DueDate:          in.DueDate,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// Get returns the task with the given ID.
func (s *TaskService) Get(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	return s.repo.GetByID(ctx, id)
}

// ---------------------------------------------------------------------------
// Update (partial)
// ---------------------------------------------------------------------------

// Update applies a partial update. It does NOT allow status changes —
// callers must go through MarkInProgress / MarkCompleted / MarkMissed
// so the state machine and side effects are enforced in one place.
func (s *TaskService) Update(ctx context.Context, id uuid.UUID, in UpdateTaskInput) (*model.Task, error) {
	if in.Priority != nil && !in.Priority.IsValid() {
		return nil, model.ErrInvalidPriority
	}
	if in.Category != nil && !in.Category.IsValid() {
		return nil, model.ErrInvalidCategory
	}
	if in.EstimatedMinutes != nil && *in.EstimatedMinutes < 0 {
		return nil, model.ErrEstimatedNegative
	}
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return nil, model.ErrTitleRequired
	}

	u := repository.TaskUpdate{
		Title:            trimmedPtr(in.Title),
		Description:      trimmedPtr(in.Description),
		Priority:         in.Priority,
		Category:         in.Category,
		EstimatedMinutes: in.EstimatedMinutes,
		DueDate:          in.DueDate,
		ClearDueDate:     in.ClearDueDate,
	}
	return s.repo.Update(ctx, id, u)
}

// SetStatus assigns status directly (edit form). Any valid status may be
// selected, including reopening a completed or missed task. Side effects:
// moving to completed sets completed_at; leaving completed clears it.
func (s *TaskService) SetStatus(ctx context.Context, id uuid.UUID, to model.Status) (*model.Task, error) {
	if !to.IsValid() {
		return nil, model.ErrInvalidStatus
	}
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cur.Status == to {
		return cur, nil
	}

	u := repository.TaskUpdate{Status: &to}
	switch to {
	case model.StatusCompleted:
		now := s.clock.Now()
		u.CompletedAt = &now
	default:
		if cur.Status == model.StatusCompleted {
			u.ClearCompletedAt = true
		}
	}
	return s.repo.Update(ctx, id, u)
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// Delete removes a task by ID.
func (s *TaskService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ---------------------------------------------------------------------------
// State transitions
// ---------------------------------------------------------------------------

// MarkInProgress moves a task from pending → in_progress.
func (s *TaskService) MarkInProgress(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	return s.transition(ctx, id, model.StatusInProgress, nil)
}

// MarkPending moves a task from in_progress → pending (cancel in-progress).
func (s *TaskService) MarkPending(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	return s.transition(ctx, id, model.StatusPending, nil)
}

// MarkCompleted moves a task to completed. The caller may provide the
// actual time spent; if 0, the existing actual_minutes is left untouched.
func (s *TaskService) MarkCompleted(ctx context.Context, id uuid.UUID, actualMinutes int) (*model.Task, error) {
	if actualMinutes < 0 {
		return nil, model.ErrActualNegative
	}
	now := s.clock.Now()
	apply := func(u *repository.TaskUpdate) {
		u.CompletedAt = &now
		if actualMinutes > 0 {
			u.ActualMinutes = &actualMinutes
		}
	}
	return s.transition(ctx, id, model.StatusCompleted, apply)
}

// MarkMissed flags a task as missed. The carry-over flow uses this
// internally as well.
func (s *TaskService) MarkMissed(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	return s.transition(ctx, id, model.StatusMissed, nil)
}

// transition is the shared state-machine guard for MarkInProgress /
// MarkCompleted / MarkMissed. The apply callback lets each caller stage
// additional column updates inside the same UPDATE round-trip.
func (s *TaskService) transition(
	ctx context.Context,
	id uuid.UUID,
	to model.Status,
	apply func(*repository.TaskUpdate),
) (*model.Task, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !current.Status.CanTransitionTo(to) {
		return nil, fmt.Errorf("%w: %s -> %s", model.ErrInvalidTransition, current.Status, to)
	}

	u := repository.TaskUpdate{Status: &to}
	if apply != nil {
		apply(&u)
	}
	return s.repo.Update(ctx, id, u)
}

// ---------------------------------------------------------------------------
// Listing / dashboard
// ---------------------------------------------------------------------------

// List returns tasks matching the filter.
func (s *TaskService) List(ctx context.Context, f ListTasksInput) ([]*model.Task, error) {
	return s.repo.List(ctx, f)
}

// ListOverdue returns non-terminal tasks whose due date is before today's
// calendar day. Used by the auto carry-over cron (active work only).
func (s *TaskService) ListOverdue(ctx context.Context) ([]*model.Task, error) {
	before := s.cal.StartOfDay(s.clock.Now())
	return s.repo.List(ctx, repository.TaskFilter{
		Statuses:  []model.Status{model.StatusPending, model.StatusInProgress},
		DueBefore: &before,
		Limit:     500,
		OrderBy:   "due_date",
	})
}

// ---------------------------------------------------------------------------
// Carry-over
// ---------------------------------------------------------------------------

// CarryOverTask rolls an unfinished task into a fresh, higher-priority
// task scheduled one day later. The original task is marked missed.
//
// Rules:
//   - source task must NOT be terminal (completed/missed)
//   - new task inherits title/description/category/estimate
//   - priority is bumped one level (capped at urgent)
//   - carry_over_count = source.carry_over_count + 1
//   - due_date is the next calendar day after now when the source is overdue
//     (prevents the auto carry-over cron from spawning duplicate copies while
//     "catching up" an old due date); otherwise one day after the source due
func (s *TaskService) CarryOverTask(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	src, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()

	// Already missed: roll the due date forward and reopen for another attempt.
	if src.Status == model.StatusMissed {
		return s.rescheduleMissed(ctx, src, now)
	}

	// Completed tasks cannot be carried over. Because CarryOverTask flips the
	// source to `missed` as its last step, this also enforces the "no duplicate
	// carry-over" rule on pending/in_progress sources.
	if src.Status.IsTerminal() {
		return nil, model.ErrTaskNotEligibleCarry
	}

	newDue := s.carryOverDueDate(src, now)

	newTask := &model.Task{
		Title:            src.Title,
		Description:      src.Description,
		Priority:         src.Priority.Bump(),
		Category:         src.Category,
		Status:           model.StatusPending,
		EstimatedMinutes: src.EstimatedMinutes,
		DueDate:          &newDue,
		CarryOverCount:   src.CarryOverCount + 1,
	}
	if err := s.repo.Create(ctx, newTask); err != nil {
		return nil, err
	}

	// Mark the source as missed AFTER the new task is safely persisted.
	// If this fails we have a duplicate; that's safer than losing the carry.
	if _, err := s.repo.Update(ctx, src.ID, repository.TaskUpdate{
		Status: statusPtr(model.StatusMissed),
	}); err != nil {
		return nil, fmt.Errorf("mark source missed: %w", err)
	}
	return newTask, nil
}

// CarryOverAllOverdue carries over every overdue, non-terminal task and
// returns the resulting NEW tasks. Designed for the daily scheduler.
//
// The function is best-effort: a failure on one task does not stop the
// loop; the per-task error is returned alongside the successes.
func (s *TaskService) CarryOverAllOverdue(ctx context.Context) (created []*model.Task, errs []error) {
	overdue, err := s.ListOverdue(ctx)
	if err != nil {
		return nil, []error{err}
	}
	toCarry, dupes := selectPrimaryOverdue(overdue, s.cal)
	for _, t := range dupes {
		if _, err := s.repo.Update(ctx, t.ID, repository.TaskUpdate{
			Status: statusPtr(model.StatusMissed),
		}); err != nil {
			errs = append(errs, fmt.Errorf("dedupe missed %s: %w", t.ID, err))
		}
	}
	for _, t := range toCarry {
		if s.sessions != nil {
			running, err := s.sessions.HasRunningSession(ctx, t.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("task %s: session check: %w", t.ID, err))
				continue
			}
			if running {
				continue
			}
		}
		nt, err := s.CarryOverTask(ctx, t.ID)
		if err != nil {
			// Skip already-handled tasks silently — they're not real failures.
			if errors.Is(err, model.ErrTaskNotEligibleCarry) {
				continue
			}
			errs = append(errs, fmt.Errorf("task %s: %w", t.ID, err))
			continue
		}
		created = append(created, nt)
	}
	return created, errs
}

// CollapseDuplicatePlans removes extra tasks that share the same title and
// calendar due day, keeping the oldest row in each group. Returns the number
// of tasks deleted.
func (s *TaskService) CollapseDuplicatePlans(ctx context.Context) (int, error) {
	tasks, err := s.repo.List(ctx, repository.TaskFilter{Limit: 500, OrderBy: "created_at"})
	if err != nil {
		return 0, err
	}
	type group struct {
		keep *model.Task
		rest []*model.Task
	}
	groups := make(map[planKey]*group)
	for _, t := range tasks {
		key, ok := s.planKeyForTask(t)
		if !ok {
			continue
		}
		g, exists := groups[key]
		if !exists {
			groups[key] = &group{keep: t}
			continue
		}
		if t.CreatedAt.Before(g.keep.CreatedAt) {
			g.rest = append(g.rest, g.keep)
			g.keep = t
		} else {
			g.rest = append(g.rest, t)
		}
	}
	removed := 0
	for _, g := range groups {
		if len(g.rest) == 0 {
			continue
		}
		for _, t := range g.rest {
			if err := s.repo.Delete(ctx, t.ID); err != nil {
				return removed, fmt.Errorf("delete %s: %w", t.ID, err)
			}
			removed++
		}
	}
	return removed, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// carryOverDueDate picks the successor task's due date. Overdue sources land
// on today's calendar day so they show up on the Today tab, not Upcoming.
// rescheduleMissed moves a missed task back to pending on the next due day.
func (s *TaskService) rescheduleMissed(ctx context.Context, src *model.Task, now time.Time) (*model.Task, error) {
	newDue := s.carryOverDueDate(src, now)
	pending := model.StatusPending
	bumped := src.Priority.Bump()
	return s.repo.Update(ctx, src.ID, repository.TaskUpdate{
		Status:   &pending,
		DueDate:  &newDue,
		Priority: &bumped,
	})
}

func (s *TaskService) carryOverDueDate(src *model.Task, now time.Time) time.Time {
	today := s.cal.StartOfDay(now)
	tomorrow := today.Add(24 * time.Hour)
	if src.DueDate == nil {
		return today
	}
	if s.cal.IsOverdue(*src.DueDate, now) {
		return today
	}
	next := s.cal.StartOfDay(*src.DueDate).Add(24 * time.Hour)
	if next.Before(tomorrow) {
		return next
	}
	return tomorrow
}

func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}

func statusPtr(s model.Status) *model.Status { return &s }
