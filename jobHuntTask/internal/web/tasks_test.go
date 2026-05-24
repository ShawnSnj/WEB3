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

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type tasksHarness struct {
	router *gin.Engine
	repo   *inMemTaskRepo
	svc    *service.TaskService
}

func newTasksHarness(t *testing.T) *tasksHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	repo := newInMemTaskRepo()
	svc := service.NewTaskService(repo, service.SystemClock)
	h := web.NewTasksHandler(rd, svc, service.SystemClock,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := gin.New()
	if err := web.MountStatic(r); err != nil {
		t.Fatalf("MountStatic: %v", err)
	}
	web.RegisterRoutes(r, rd)
	h.Register(r)
	return &tasksHarness{router: r, repo: repo, svc: svc}
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
		`Today`, `Overdue`, `Completed`, `Carried over`, `All`,
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
	if !strings.Contains(w.Body.String(), "Today is clear") {
		t.Error("expected today empty-state copy")
	}
}

func TestTasksList_FiltersByCategoryAndPriority(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	due := time.Now().Add(2 * time.Hour)
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
	if !strings.Contains(body, `hx-patch="/tasks/`+tk.ID.String()+`"`) {
		t.Error("form should target PATCH on the existing task")
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

	w := doForm(t, h.router, http.MethodPost, "/tasks/"+tk.ID.String()+"/complete",
		url.Values{"actual_minutes": {"25"}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusCompleted {
		t.Errorf("status = %s, want completed", got.Status)
	}
	if got.ActualMinutes != 25 {
		t.Errorf("actual minutes = %d, want 25", got.ActualMinutes)
	}
}

func TestTasks_MarkInProgress(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	tk := h.seedTask(t)

	w := doForm(t, h.router, http.MethodPost, "/tasks/"+tk.ID.String()+"/in_progress", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	got, _ := h.repo.GetByID(context.Background(), tk.ID)
	if got.Status != model.StatusInProgress {
		t.Errorf("status = %s, want in_progress", got.Status)
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

func TestTasks_UnknownIDReturns404(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t)
	w := doGet(t, h.router, "/tasks/"+uuid.New().String()+"/form", true)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
