package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/api"
	"github.com/shawn/jobhunttask/internal/config"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Minimal in-memory repo just for HTTP-level tests.
// ---------------------------------------------------------------------------

type memRepo struct {
	mu    sync.Mutex
	tasks map[uuid.UUID]*model.Task
}

func newMemRepo() *memRepo { return &memRepo{tasks: map[uuid.UUID]*model.Task{}} }

func (r *memRepo) Create(_ context.Context, t *model.Task) error {
	if err := t.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	cp := *t
	r.tasks[t.ID] = &cp
	return nil
}
func (r *memRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}
func (r *memRepo) Update(_ context.Context, id uuid.UUID, u repository.TaskUpdate) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	if u.Title != nil {
		t.Title = *u.Title
	}
	if u.Status != nil {
		t.Status = *u.Status
	}
	if u.Priority != nil {
		t.Priority = *u.Priority
	}
	if u.ActualMinutes != nil {
		t.ActualMinutes = *u.ActualMinutes
	}
	if u.CompletedAt != nil {
		v := *u.CompletedAt
		t.CompletedAt = &v
	}
	t.UpdatedAt = time.Now()
	cp := *t
	return &cp, nil
}
func (r *memRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return model.ErrTaskNotFound
	}
	delete(r.tasks, id)
	return nil
}
func (r *memRepo) List(_ context.Context, _ repository.TaskFilter) ([]*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Task, 0, len(r.tasks))
	for _, t := range r.tasks {
		cp := *t
		out = append(out, &cp)
	}
	return out, nil
}
func (r *memRepo) ListOverdue(_ context.Context, now time.Time) ([]*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*model.Task{}
	for _, t := range r.tasks {
		if t.IsOverdue(now) {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

func newTestRouter(t *testing.T) (*gin.Engine, *memRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newMemRepo()
	svc := service.NewTaskService(repo, service.SystemClock)
	r := api.NewRouter(api.Deps{
		Config:      config.Config{},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		TaskService: svc,
	})
	return r, repo
}

func doRequest(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func TestAPI_CreateTask(t *testing.T) {
	t.Parallel()
	r, _ := newTestRouter(t)

	w := doRequest(t, r, http.MethodPost, "/api/v1/tasks", map[string]any{
		"title":             "Apply to Acme",
		"category":          "job_apply",
		"priority":          "high",
		"estimated_minutes": 45,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	decode(t, w, &got)
	if got["title"] != "Apply to Acme" {
		t.Errorf("title=%v", got["title"])
	}
	if got["status"] != "pending" {
		t.Errorf("status=%v", got["status"])
	}
}

func TestAPI_CreateTask_ValidationError(t *testing.T) {
	t.Parallel()
	r, _ := newTestRouter(t)

	w := doRequest(t, r, http.MethodPost, "/api/v1/tasks", map[string]any{
		"title":    "",
		"priority": "bogus",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAPI_GetTask_NotFound(t *testing.T) {
	t.Parallel()
	r, _ := newTestRouter(t)
	w := doRequest(t, r, http.MethodGet, "/api/v1/tasks/"+uuid.NewString(), nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestAPI_FullLifecycle(t *testing.T) {
	t.Parallel()
	r, _ := newTestRouter(t)

	// Create
	w := doRequest(t, r, http.MethodPost, "/api/v1/tasks", map[string]any{
		"title": "End to end task",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d", w.Code)
	}
	var created map[string]any
	decode(t, w, &created)
	id := created["id"].(string)

	// Start
	w = doRequest(t, r, http.MethodPost, "/api/v1/tasks/"+id+"/start", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("start: %d body=%s", w.Code, w.Body.String())
	}

	// Complete
	w = doRequest(t, r, http.MethodPost, "/api/v1/tasks/"+id+"/complete", map[string]any{
		"actual_minutes": 30,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("complete: %d body=%s", w.Code, w.Body.String())
	}
	var completed map[string]any
	decode(t, w, &completed)
	if completed["status"] != "completed" {
		t.Errorf("status=%v", completed["status"])
	}
	if completed["actual_minutes"].(float64) != 30 {
		t.Errorf("actual_minutes=%v", completed["actual_minutes"])
	}

	// Re-starting a completed task is a conflict.
	w = doRequest(t, r, http.MethodPost, "/api/v1/tasks/"+id+"/start", nil)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAPI_DeleteTask(t *testing.T) {
	t.Parallel()
	r, _ := newTestRouter(t)
	w := doRequest(t, r, http.MethodPost, "/api/v1/tasks", map[string]any{"title": "x"})
	var created map[string]any
	decode(t, w, &created)
	id := created["id"].(string)

	w = doRequest(t, r, http.MethodDelete, "/api/v1/tasks/"+id, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
	w = doRequest(t, r, http.MethodGet, "/api/v1/tasks/"+id, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 after delete, got %d", w.Code)
	}
}

func TestAPI_InvalidUUID(t *testing.T) {
	t.Parallel()
	r, _ := newTestRouter(t)
	w := doRequest(t, r, http.MethodGet, "/api/v1/tasks/not-a-uuid", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAPI_CarryOver(t *testing.T) {
	t.Parallel()
	r, _ := newTestRouter(t)

	// Create overdue task by posting then patching due_date would be complex —
	// easiest path is via the model directly through the repo. But we also
	// want to exercise the HTTP path, so we send an overdue due_date in the
	// create body.
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	w := doRequest(t, r, http.MethodPost, "/api/v1/tasks", map[string]any{
		"title":    "overdue",
		"due_date": past,
		"priority": "medium",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	decode(t, w, &created)
	id := created["id"].(string)

	w = doRequest(t, r, http.MethodPost, "/api/v1/tasks/"+id+"/carry-over", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("carry-over: %d body=%s", w.Code, w.Body.String())
	}
	var newTask map[string]any
	decode(t, w, &newTask)
	if newTask["priority"] != "high" {
		t.Errorf("priority should bump to high, got %v", newTask["priority"])
	}
	if newTask["carry_over_count"].(float64) != 1 {
		t.Errorf("carry_over_count=%v", newTask["carry_over_count"])
	}
}
