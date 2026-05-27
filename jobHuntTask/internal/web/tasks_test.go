package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
	"github.com/shawn/jobhunttask/internal/web"
)

// inMemTaskRepo is a small but real implementation of repository.TaskRepository
// — supports Create / Get / Update / Delete / List / ListOverdue with the
// filter semantics the handler actually exercises.
type inMemTaskRepo struct {
	mu    sync.Mutex
	items map[uuid.UUID]*model.Task
}

func newInMemTaskRepo() *inMemTaskRepo {
	return &inMemTaskRepo{items: map[uuid.UUID]*model.Task{}}
}

func (r *inMemTaskRepo) Create(_ context.Context, t *model.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t.ID = uuid.New()
	now := time.Now()
	t.CreatedAt, t.UpdatedAt = now, now
	cp := *t
	r.items[t.ID] = &cp
	return nil
}

func (r *inMemTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.items[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}

func (r *inMemTaskRepo) Update(_ context.Context, id uuid.UUID, u repository.TaskUpdate) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.items[id]
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
	if u.DueDate != nil {
		v := *u.DueDate
		t.DueDate = &v
	} else if u.ClearDueDate {
		t.DueDate = nil
	}
	if u.CarryOverCount != nil {
		t.CarryOverCount = *u.CarryOverCount
	}
	if u.CompletedAt != nil {
		v := *u.CompletedAt
		t.CompletedAt = &v
	} else if u.ClearCompletedAt {
		t.CompletedAt = nil
	}
	t.UpdatedAt = time.Now()
	cp := *t
	return &cp, nil
}

func (r *inMemTaskRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return model.ErrTaskNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *inMemTaskRepo) List(_ context.Context, f repository.TaskFilter) ([]*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Task, 0, len(r.items))
	for _, t := range r.items {
		if !matchesFilter(t, f) {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *inMemTaskRepo) ListOverdue(_ context.Context, now time.Time) ([]*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*model.Task{}
	for _, t := range r.items {
		if t.Status.IsTerminal() || t.DueDate == nil {
			continue
		}
		if t.DueDate.Before(now) {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out, nil
}

func matchesFilter(t *model.Task, f repository.TaskFilter) bool {
	if len(f.Statuses) > 0 && !containsStatus(f.Statuses, t.Status) {
		return false
	}
	if len(f.Priorities) > 0 && !containsPriority(f.Priorities, t.Priority) {
		return false
	}
	if len(f.Categories) > 0 && !containsCategory(f.Categories, t.Category) {
		return false
	}
	if f.OnlyOverdue {
		if t.Status.IsTerminal() || t.DueDate == nil || !t.DueDate.Before(time.Now()) {
			return false
		}
	}
	if f.DueBefore != nil && t.DueDate != nil && !t.DueDate.Before(*f.DueBefore) {
		return false
	}
	if f.DueAfter != nil {
		if t.DueDate == nil || t.DueDate.Before(*f.DueAfter) {
			return false
		}
	}
	if f.CompletedAfter != nil {
		if t.CompletedAt == nil || t.CompletedAt.Before(*f.CompletedAfter) {
			return false
		}
	}
	if f.CompletedBefore != nil {
		if t.CompletedAt == nil || !t.CompletedAt.Before(*f.CompletedBefore) {
			return false
		}
	}
	if f.UpdatedAfter != nil {
		if t.UpdatedAt.Before(*f.UpdatedAfter) {
			return false
		}
	}
	if f.UpdatedBefore != nil {
		if !t.UpdatedAt.Before(*f.UpdatedBefore) {
			return false
		}
	}
	if f.CarriedOver != nil {
		want := *f.CarriedOver
		got := t.CarryOverCount > 0
		if want != got {
			return false
		}
	}
	return true
}

func containsStatus(ss []model.Status, s model.Status) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
func containsPriority(ps []model.Priority, p model.Priority) bool {
	for _, x := range ps {
		if x == p {
			return true
		}
	}
	return false
}
func containsCategory(cs []model.Category, c model.Category) bool {
	for _, x := range cs {
		if x == c {
			return true
		}
	}
	return false
}

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

type tasksSessionRepo struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*model.TaskSession
	now      func() time.Time
}

func newTasksSessionRepo(now func() time.Time) *tasksSessionRepo {
	return &tasksSessionRepo{sessions: map[uuid.UUID]*model.TaskSession{}, now: now}
}

func (r *tasksSessionRepo) Create(_ context.Context, s *model.TaskSession) error {
	if err := s.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (r *tasksSessionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, model.ErrSessionNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *tasksSessionRepo) Update(_ context.Context, id uuid.UUID, u repository.SessionUpdate) (*model.TaskSession, error) {
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

func (r *tasksSessionRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[id]; !ok {
		return model.ErrSessionNotFound
	}
	delete(r.sessions, id)
	return nil
}

func (r *tasksSessionRepo) List(_ context.Context, _ repository.SessionFilter) ([]*model.TaskSession, error) {
	return nil, nil
}

func (r *tasksSessionRepo) FindRunningByTask(_ context.Context, taskID uuid.UUID) (*model.TaskSession, error) {
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

func (r *tasksSessionRepo) SumEffectiveMinutesByTask(_ context.Context, taskID uuid.UUID, now time.Time) (int, error) {
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

func (r *tasksSessionRepo) SumEffectiveMinutesInRange(_ context.Context, from, to, now time.Time) (int, error) {
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
// Harness
// ---------------------------------------------------------------------------

type tasksHarness struct {
	router     *gin.Engine
	repo       *inMemTaskRepo
	sessions   *tasksSessionRepo
	svc        *service.TaskService
	sessionSvc *service.TaskSessionService
	clock      *testClock
}

func newTasksHarness(t *testing.T) *tasksHarness {
	return newTasksHarnessAt(t, time.Now().UTC())
}

func newTasksHarnessAt(t *testing.T, now time.Time) *tasksHarness {
	return newTasksHarnessAtWithCal(t, now, calendar.UTC())
}

func newTasksHarnessAtWithCal(t *testing.T, now time.Time, cal *calendar.Calendar) *tasksHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	repo := newInMemTaskRepo()
	clk := &testClock{now: now}
	svc := service.NewTaskService(repo, clk, cal)
	sessions := newTasksSessionRepo(func() time.Time { return clk.now })
	sessionSvc := service.NewTaskSessionService(sessions, svc, clk)
	h := web.NewTasksHandler(rd, svc, sessionSvc, clk, cal,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := gin.New()
	if err := web.MountStatic(r); err != nil {
		t.Fatalf("MountStatic: %v", err)
	}
	web.RegisterRoutes(r, rd)
	h.Register(r)
	return &tasksHarness{router: r, repo: repo, sessions: sessions, svc: svc, sessionSvc: sessionSvc, clock: clk}
}

// seedTask creates a task with sane defaults plus any overrides applied.
func (h *tasksHarness) seedTask(t *testing.T, overrides ...func(*model.Task)) *model.Task {
	t.Helper()
	tk := &model.Task{
		Title:    "Seeded task",
		Priority: model.PriorityMedium,
		Category: model.CategoryMisc,
		Status:   model.StatusPending,
	}
	for _, o := range overrides {
		o(tk)
	}
	if err := h.repo.Create(context.Background(), tk); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return tk
}

func doForm(t *testing.T, r *gin.Engine, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Tests — page + list
// ---------------------------------------------------------------------------

func TestTasksPage_RendersAllTabsAndFilterBar(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%.300s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`class="tasks-tabs"`,
		`class="filter-bar"`,
		`class="bulk-bar"`,
		`id="tasks-list-host"`,
		`id="task-modal"`,
		`Today`, `Upcoming`, `Overdue`, `Completed`, `Carried over`, `All`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in page body", want)
		}
	}
}

func TestTasksPage_EmptyView(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/tasks/list?view=today", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="tasks-list"`) {
		t.Error("list fragment must include #tasks-list for HTMX tab targets")
	}
	if !strings.Contains(body, "Today is clear") {
		t.Error("expected today empty-state copy")
	}
}

func TestTasksTabs_AllViewCount(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)

	todayDue := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	tomorrowDue := time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC)
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Today A"; tk.DueDate = &todayDue })
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Today B"; tk.DueDate = &todayDue })
	h.seedTask(t, func(tk *model.Task) {
		tk.Title = "Done"; tk.DueDate = &todayDue
		tk.Status = model.StatusCompleted
	})
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Future"; tk.DueDate = &tomorrowDue })

	w := doGet(t, h.router, "/tasks", false)
	body := w.Body.String()
	if !strings.Contains(body, `data-view="all"`) {
		t.Fatalf("missing all tab: %s", body)
	}
	// 4 tasks total in repo.
	if !strings.Contains(body, `data-view="all"`) || !strings.Contains(body, ">4<") {
		// tab-count is inside span after All label
		if strings.Count(body, `tab-count">4<`) != 1 {
			t.Errorf("expected All tab count 4, body snippet: %.800s", body)
		}
	}

	w2 := doGet(t, h.router, "/tasks/list?view=all", true)
	if !strings.Contains(w2.Body.String(), "Today A") || !strings.Contains(w2.Body.String(), "Done") {
		t.Fatalf("all view should list every task: %s", w2.Body.String())
	}
}

func TestTasksTabsFragment_RefreshesCounts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)
	due := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	h.seedTask(t, func(tk *model.Task) { tk.Title = "One"; tk.DueDate = &due })

	w := doGet(t, h.router, "/tasks/tabs?view=today", true)
	body := w.Body.String()
	if !strings.Contains(body, `class="tasks-tabs"`) {
		t.Fatalf("tabs fragment missing nav: %s", body)
	}
	if !strings.Contains(body, `tab-count">1<`) {
		t.Errorf("expected today count 1 in tabs fragment: %s", body)
	}
}

func TestTasksList_TodayViewRespectsAppTimezone(t *testing.T) {
	t.Parallel()
	cal, err := calendar.Load("Asia/Taipei")
	if err != nil {
		t.Fatal(err)
	}
	// Local May 26 06:00 Taipei; UTC still May 25 22:00.
	now := time.Date(2026, 5, 25, 22, 0, 0, 0, time.UTC)
	h := newTasksHarnessAtWithCal(t, now, cal)

	dueToday, ok := cal.ParseDate("2026-05-26")
	if !ok {
		t.Fatal("parse due today")
	}
	dueTomorrow, ok := cal.ParseDate("2026-05-27")
	if !ok {
		t.Fatal("parse due tomorrow")
	}
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Due today Taipei"; tk.DueDate = &dueToday })
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Due tomorrow Taipei"; tk.DueDate = &dueTomorrow })

	w := doGet(t, h.router, "/tasks/list?view=today", true)
	body := w.Body.String()
	if !strings.Contains(body, "Due today Taipei") {
		t.Fatalf("expected today task in today view, got: %s", body)
	}
	if strings.Contains(body, "Due tomorrow Taipei") {
		t.Fatalf("tomorrow task should not appear in today view: %s", body)
	}

	w2 := doGet(t, h.router, "/tasks/list?view=upcoming", true)
	body2 := w2.Body.String()
	if !strings.Contains(body2, "Due tomorrow Taipei") {
		t.Fatalf("expected tomorrow task in upcoming view, got: %s", body2)
	}
	if strings.Contains(body2, "Due today Taipei") {
		t.Fatalf("today task should not appear in upcoming view: %s", body2)
	}
}

func TestTasksList_TodayViewKeepsCompletedDueToday(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 26, 15, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)

	todayDue := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	completedAt := now.Add(-1 * time.Hour)
	h.seedTask(t, func(tk *model.Task) {
		tk.Title = "Still visible"
		tk.DueDate = &todayDue
		tk.Status = model.StatusCompleted
		tk.CompletedAt = &completedAt
	})
	h.seedTask(t, func(tk *model.Task) {
		tk.Title = "Old done"
		oldDue := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		oldDone := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
		tk.DueDate = &oldDue
		tk.Status = model.StatusCompleted
		tk.CompletedAt = &oldDone
	})

	w := doGet(t, h.router, "/tasks/list?view=today", true)
	body := w.Body.String()
	if !strings.Contains(body, "Still visible") {
		t.Fatalf("completed task due today should remain on today view: %s", body)
	}
	if strings.Contains(body, "Old done") {
		t.Fatalf("old completed task should not appear on today view: %s", body)
	}
}

func TestTasksList_UpcomingView(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)

	todayDue := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	tomorrowDue := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	soonDue := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	farDue := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	h.seedTask(t, func(tk *model.Task) { tk.Title = "Due today"; tk.DueDate = &todayDue })
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Due tomorrow"; tk.DueDate = &tomorrowDue })
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Due soon"; tk.DueDate = &soonDue })
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Due later"; tk.DueDate = &farDue })

	w := doGet(t, h.router, "/tasks/list?view=upcoming", true)
	body := w.Body.String()
	if !strings.Contains(body, "Due tomorrow") || !strings.Contains(body, "Due soon") {
		t.Fatalf("expected upcoming tasks in body: %s", body)
	}
	if strings.Contains(body, "Due today") || strings.Contains(body, "Due later") {
		t.Fatalf("today/far tasks should not appear in upcoming view: %s", body)
	}
}

func TestTasksList_TodayEmptyShowsUpcomingHint(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)

	tomorrowDue := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Future task"; tk.DueDate = &tomorrowDue })

	w := doGet(t, h.router, "/tasks/list?view=today", true)
	body := w.Body.String()
	if !strings.Contains(body, "Nothing due today") {
		t.Fatalf("expected contextual empty title, got: %s", body)
	}
	if !strings.Contains(body, "scheduled starting") || !strings.Contains(body, "View upcoming") {
		t.Fatalf("expected upcoming hint and action, got: %s", body)
	}
}

func TestTasksList_FiltersByCategoryAndPriority(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)
	due := time.Date(2026, 5, 24, 14, 0, 0, 0, time.UTC)
	h.seedTask(t, func(tk *model.Task) {
		tk.Title = "Apply to Google"
		tk.Category = model.CategoryJobApply
		tk.Priority = model.PriorityHigh
		tk.DueDate = &due
	})
	h.seedTask(t, func(tk *model.Task) {
		tk.Title = "Tweet thread"
		tk.Category = model.CategoryTwitter
		tk.Priority = model.PriorityLow
		tk.DueDate = &due
	})

	req := httptest.NewRequest(http.MethodGet,
		"/tasks/list?view=today&category=job_apply", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "Apply to Google") {
		t.Error("expected filtered task in body")
	}
	if strings.Contains(body, "Tweet thread") {
		t.Error("non-matching task should be filtered out")
	}
}

func TestTasksList_SearchAndSortFlip(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	now := time.Now()
	due1 := now.Add(time.Hour)
	due2 := now.Add(3 * time.Hour)
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Alpha"; tk.DueDate = &due1 })
	h.seedTask(t, func(tk *model.Task) { tk.Title = "Beta"; tk.DueDate = &due2 })

	// Search narrows to Alpha only.
	w := doGet(t, h.router, "/tasks/list?view=today&q=alpha", true)
	body := w.Body.String()
	if !strings.Contains(body, "Alpha") {
		t.Error("expected Alpha in search result")
	}
	if strings.Contains(body, ">Beta<") {
		t.Error("Beta should not appear in filtered list")
	}

	// Sort link flips between asc/desc when clicked twice on same field.
	w = doGet(t, h.router, "/tasks/list?view=today&sort=due_date&dir=asc", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	w = doGet(t, h.router, "/tasks/list?view=today&sort=due_date&dir=desc", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestTasksList_CompletedShowsDueDateAndSortsByDue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)

	dueEarly := now.Add(-72 * time.Hour)
	dueLate := now.Add(-24 * time.Hour)
	completedAt := now.Add(-2 * time.Hour)

	h.seedTask(t, func(tk *model.Task) {
		tk.Title = "Earlier due"
		tk.Status = model.StatusCompleted
		tk.DueDate = &dueEarly
		tk.CompletedAt = &completedAt
	})
	h.seedTask(t, func(tk *model.Task) {
		tk.Title = "Later due"
		tk.Status = model.StatusCompleted
		tk.DueDate = &dueLate
		tk.CompletedAt = &completedAt
	})

	w := doGet(t, h.router, "/tasks/list?view=completed&sort=due_date&dir=asc", true)
	body := w.Body.String()
	if strings.Contains(body, ">overdue<") || strings.Contains(body, "due-overdue") {
		t.Errorf("completed tab should show actual due dates, not overdue styling: %.500s", body)
	}
	if !strings.Contains(body, "May 23, 2026") || !strings.Contains(body, "May 25, 2026") {
		t.Errorf("expected formatted due dates in completed list: %.500s", body)
	}
	idxEarly := strings.Index(body, "Earlier due")
	idxLate := strings.Index(body, "Later due")
	if idxEarly < 0 || idxLate < 0 {
		t.Fatal("expected both completed tasks in list")
	}
	if idxEarly > idxLate {
		t.Error("asc due sort: Earlier due should appear before Later due")
	}

	w = doGet(t, h.router, "/tasks/list?view=completed&sort=due_date&dir=desc", true)
	body = w.Body.String()
	idxEarly = strings.Index(body, "Earlier due")
	idxLate = strings.Index(body, "Later due")
	if idxEarly < idxLate {
		t.Error("desc due sort: Later due should appear before Earlier due")
	}
	if !strings.Contains(body, `sort-link--active`) || !strings.Contains(body, "Due") {
		t.Error("expected active Due sort link on completed tab")
	}
}

func TestTasksList_SortLinkURLsToggleDueDirection(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	h.seedTask(t, func(tk *model.Task) { tk.Status = model.StatusCompleted })

	w := doGet(t, h.router, "/tasks/list?view=completed&sort=due_date&dir=asc", true)
	body := w.Body.String()
	if !strings.Contains(body, `hx-get="/tasks/list?`) {
		t.Fatal("expected sort link with hx-get list URL")
	}
	if !strings.Contains(body, "sort=due_date") || !strings.Contains(body, "dir=desc") {
		t.Errorf("asc due sort link should flip to desc on click: %.800s", body)
	}

	w = doGet(t, h.router, "/tasks/list?view=completed&sort=due_date&dir=desc", true)
	body = w.Body.String()
	if !strings.Contains(body, "dir=asc") {
		t.Errorf("desc due sort link should flip to asc on click: %.800s", body)
	}
}

// ---------------------------------------------------------------------------
// Tests — create / edit / delete
// ---------------------------------------------------------------------------

func TestTasks_CreateReturnsRowAndPersists(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)

	form := url.Values{}
	form.Set("title", "Write cover letter")
	form.Set("priority", "high")
	form.Set("category", "job_apply")
	form.Set("estimated_minutes", "45")

	w := doForm(t, h.router, http.MethodPost, "/tasks", form)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Write cover letter") {
		t.Error("created row should contain title")
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("expected HX-Trigger to include tasks-changed")
	}
	tasks, _ := h.repo.List(context.Background(), repository.TaskFilter{})
	if len(tasks) != 1 {
		t.Fatalf("repo size = %d, want 1", len(tasks))
	}
}

func TestTasks_CreateValidationError(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)

	form := url.Values{}
	form.Set("title", "") // missing required field

	w := doForm(t, h.router, http.MethodPost, "/tasks", form)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Title is required") {
		t.Error("expected validation message in re-rendered form")
	}
}

func TestTasks_EditFormPrefillsExistingValues(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t, func(x *model.Task) {
		x.Title = "Refresh GitHub profile"
		x.Category = model.CategoryGithub
		x.Priority = model.PriorityHigh
		x.EstimatedMinutes = 30
	})

	w := doGet(t, h.router, "/tasks/"+tk.ID.String()+"/form", true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `value="Refresh GitHub profile"`) {
		t.Error("title not pre-filled")
	}
	if !strings.Contains(body, `data-method="patch"`) || !strings.Contains(body, `data-action="/tasks/`+tk.ID.String()+`"`) {
		t.Error("form should PATCH the existing task")
	}
	if !strings.Contains(body, `name="status"`) {
		t.Error("edit form should include status select")
	}
	for _, label := range []string{"Pending", "In progress", "Completed", "Missed"} {
		if !strings.Contains(body, ">"+label+"<") {
			t.Errorf("edit form missing status option %q", label)
		}
	}
}

func TestTasks_PatchUpdatesStatus(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t, func(x *model.Task) { x.Status = model.StatusPending })

	form := url.Values{}
	form.Set("title", tk.Title)
	form.Set("priority", string(tk.Priority))
	form.Set("category", string(tk.Category))
	form.Set("estimated_minutes", "0")
	form.Set("status", "in_progress")

	w := doForm(t, h.router, http.MethodPatch, "/tasks/"+tk.ID.String(), form)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusInProgress {
		t.Errorf("status = %s, want in_progress", got.Status)
	}
	if !strings.Contains(w.Body.String(), `task-row--status-in_progress`) {
		t.Error("response row should reflect new status")
	}
}

func TestTasks_PatchReopensCompletedTask(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t, func(x *model.Task) { x.Status = model.StatusCompleted })

	form := url.Values{}
	form.Set("title", tk.Title)
	form.Set("priority", string(tk.Priority))
	form.Set("category", string(tk.Category))
	form.Set("estimated_minutes", "0")
	form.Set("status", "pending")

	w := doForm(t, h.router, http.MethodPatch, "/tasks/"+tk.ID.String(), form)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "status change isn't allowed") {
		t.Fatal("edit form should allow reopening completed tasks")
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusPending {
		t.Errorf("status = %s, want pending", got.Status)
	}
}

func TestTasks_PatchUpdatesFields(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t)

	form := url.Values{}
	form.Set("title", "Renamed")
	form.Set("priority", "urgent")
	form.Set("category", "interview")
	form.Set("estimated_minutes", "90")
	form.Set("description", "Phone screen prep")
	form.Set("due_date", "")

	w := doForm(t, h.router, http.MethodPatch, "/tasks/"+tk.ID.String(), form)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Renamed") {
		t.Error("response row should reflect new title")
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Title != "Renamed" || got.Priority != model.PriorityUrgent {
		t.Errorf("repo not updated: %+v", got)
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("patch should trigger list refresh")
	}
}

func TestTasks_Delete(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t)

	req := httptest.NewRequest(http.MethodDelete, "/tasks/"+tk.ID.String(), nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if _, err := h.repo.GetByID(context.Background(), tk.ID); err == nil {
		t.Error("task should be gone after delete")
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("delete should fire tasks-changed")
	}
}

// ---------------------------------------------------------------------------
// Tests — state transitions + carry-over
// ---------------------------------------------------------------------------

func TestTasks_MarkComplete(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t)

	req := httptest.NewRequest(http.MethodPost, "/tasks/"+tk.ID.String()+"/complete", strings.NewReader("actual_minutes=25"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `hx-swap-oob="outerHTML"`) {
		t.Errorf("expected OOB swap bundle, got: %.500s", body)
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("complete should trigger list refresh")
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusCompleted {
		t.Errorf("status = %s, want completed", got.Status)
	}
	if got.ActualMinutes != 25 {
		t.Errorf("actual minutes = %d, want 25", got.ActualMinutes)
	}
}

func TestTasks_MarkPending(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t, func(x *model.Task) { x.Status = model.StatusInProgress })

	req := httptest.NewRequest(http.MethodPost, "/tasks/"+tk.ID.String()+"/pending", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusPending {
		t.Errorf("status = %s, want pending", got.Status)
	}
	if !strings.Contains(w.Body.String(), `task-row--status-pending`) {
		t.Error("expected pending status in swapped row")
	}
}

func TestTasks_MarkInProgress(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t)

	req := httptest.NewRequest(http.MethodPost, "/tasks/"+tk.ID.String()+"/in_progress", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `hx-swap-oob="outerHTML"`) {
		t.Errorf("expected OOB swap bundle, got: %.500s", body)
	}
	if !strings.Contains(body, `id="task-row-`) || !strings.Contains(body, `id="task-card-`) {
		t.Error("expected both row and card in swap response")
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("in progress should trigger list refresh")
	}
	if !strings.Contains(body, `data-task-timer`) {
		t.Error("expected live timer after starting in progress")
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusInProgress {
		t.Errorf("status = %s, want in_progress", got.Status)
	}
}

func TestTasks_CompleteSavesTrackedMinutes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	h := newTasksHarnessAt(t, now)
	tk := h.seedTask(t)

	req := httptest.NewRequest(http.MethodPost, "/tasks/"+tk.ID.String()+"/in_progress", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("start in_progress status = %d", w.Code)
	}

	h.clock.now = now.Add(32 * time.Minute)

	req2 := httptest.NewRequest(http.MethodPost, "/tasks/"+tk.ID.String()+"/complete", nil)
	req2.Header.Set("HX-Request", "true")
	w2 := httptest.NewRecorder()
	h.router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("complete status = %d body=%s", w2.Code, w2.Body.String())
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusCompleted {
		t.Errorf("status = %s, want completed", got.Status)
	}
	if got.ActualMinutes != 32 {
		t.Errorf("actual_minutes = %d, want 32", got.ActualMinutes)
	}
	if !strings.Contains(w2.Header().Get("HX-Trigger"), "Completed in 32 min") {
		t.Errorf("expected minutes in toast trigger, got %q", w2.Header().Get("HX-Trigger"))
	}
}

func TestTasks_MarkMissed(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t)
	w := doForm(t, h.router, http.MethodPost, "/tasks/"+tk.ID.String()+"/missed", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusMissed {
		t.Errorf("status = %s, want missed", got.Status)
	}
}

func TestTasks_CarryOver(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	due := time.Now().Add(-24 * time.Hour) // overdue
	tk := h.seedTask(t, func(x *model.Task) { x.DueDate = &due })

	w := doForm(t, h.router, http.MethodPost, "/tasks/"+tk.ID.String()+"/carry_over", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("carry over should trigger list refresh")
	}
	src, _ := h.repo.GetByID(context.Background(), tk.ID)
	if src.Status != model.StatusMissed {
		t.Errorf("source status = %s, want missed", src.Status)
	}
	tasks, _ := h.repo.List(context.Background(), repository.TaskFilter{})
	if len(tasks) != 2 {
		t.Fatalf("expected source + new task = 2, got %d", len(tasks))
	}
}

func TestTasks_InvalidTransitionReturnsConflict(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t, func(x *model.Task) { x.Status = model.StatusCompleted })

	w := doForm(t, h.router, http.MethodPost, "/tasks/"+tk.ID.String()+"/in_progress", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Tests — bulk
// ---------------------------------------------------------------------------

func TestTasks_BulkComplete(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	t1 := h.seedTask(t)
	t2 := h.seedTask(t)
	t3 := h.seedTask(t)

	form := url.Values{}
	form["ids"] = []string{t1.ID.String(), t2.ID.String(), t3.ID.String()}
	w := doForm(t, h.router, http.MethodPost, "/tasks/bulk/complete", form)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	for _, id := range []uuid.UUID{t1.ID, t2.ID, t3.ID} {
		got, _ := h.repo.GetByID(context.Background(), id)
		if got.Status != model.StatusCompleted {
			t.Errorf("task %s not completed", id)
		}
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("bulk complete should trigger refresh")
	}
}

func TestTasks_BulkDelete(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	t1 := h.seedTask(t)
	t2 := h.seedTask(t)

	form := url.Values{}
	form["ids"] = []string{t1.ID.String(), t2.ID.String()}
	w := doForm(t, h.router, http.MethodPost, "/tasks/bulk/delete", form)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	left, _ := h.repo.List(context.Background(), repository.TaskFilter{})
	if len(left) != 0 {
		t.Errorf("repo should be empty, got %d", len(left))
	}
}

func TestTasks_BulkWithNoIDsIsNoop(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	h.seedTask(t)
	w := doForm(t, h.router, http.MethodPost, "/tasks/bulk/complete", url.Values{})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	// Single seeded task unchanged.
	tasks, _ := h.repo.List(context.Background(), repository.TaskFilter{})
	if tasks[0].Status != model.StatusPending {
		t.Errorf("task should be untouched, got status=%s", tasks[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Tests — fragment-only output
// ---------------------------------------------------------------------------

func TestTasks_AllFragmentEndpointsReturnPartials(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t)
	for _, path := range []string{
		"/tasks/list?view=today",
		"/tasks/form",
		"/tasks/" + tk.ID.String() + "/form",
		"/tasks/" + tk.ID.String() + "/row",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			w := doGet(t, h.router, path, true)
			if w.Code != http.StatusOK {
				t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
			}
			body := w.Body.String()
			if strings.Contains(body, "<!DOCTYPE html>") {
				t.Errorf("%s returned full page instead of fragment", path)
			}
		})
	}
}

func TestTasks_InvalidIDReturns400(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	w := doGet(t, h.router, "/tasks/not-a-uuid/form", true)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTasks_ImportCSV(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	csv := "title,description,category,priority,estimated_minutes,due_date\nImported task,,job_apply,high,30,\n"
	form := url.Values{"csv": {csv}}
	w := doForm(t, h.router, http.MethodPost, "/tasks/import", form)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Imported") {
		t.Fatalf("expected result partial, got %s", w.Body.String())
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "tasks-changed") {
		t.Error("expected tasks-changed trigger")
	}
	tasks, _ := h.repo.List(context.Background(), repository.TaskFilter{})
	if len(tasks) != 1 || tasks[0].Title != "Imported task" {
		t.Fatalf("repo = %+v", tasks)
	}
}

func TestTasks_ImportTemplate(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/tasks/import/template.csv", nil)
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "title,description") {
		t.Error("expected csv header in template")
	}
}

func TestTasks_UnknownIDReturns404(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	w := doGet(t, h.router, "/tasks/"+uuid.New().String()+"/form", true)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
