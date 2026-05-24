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
// In-memory session repo for the API test
// ---------------------------------------------------------------------------

type memSessionRepo struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*model.TaskSession
}

func newMemSessionRepo() *memSessionRepo {
	return &memSessionRepo{sessions: map[uuid.UUID]*model.TaskSession{}}
}

func (r *memSessionRepo) Create(_ context.Context, s *model.TaskSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ex := range r.sessions {
		if ex.TaskID == s.TaskID && ex.Status.IsRunning() {
			return model.ErrSessionAlreadyRunning
		}
	}
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	s.UpdatedAt = s.CreatedAt
	cp := *s
	r.sessions[s.ID] = &cp
	return nil
}
func (r *memSessionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, model.ErrSessionNotFound
	}
	cp := *s
	return &cp, nil
}
func (r *memSessionRepo) Update(_ context.Context, id uuid.UUID, u repository.SessionUpdate) (*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, model.ErrSessionNotFound
	}
	if u.Status != nil {
		s.Status = *u.Status
	}
	if u.ClearEndedAt {
		s.EndedAt = nil
	} else if u.EndedAt != nil {
		v := *u.EndedAt
		s.EndedAt = &v
	}
	if u.ClearPausedAt {
		s.PausedAt = nil
	} else if u.PausedAt != nil {
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
	s.UpdatedAt = time.Now()
	cp := *s
	return &cp, nil
}
func (r *memSessionRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[id]; !ok {
		return model.ErrSessionNotFound
	}
	delete(r.sessions, id)
	return nil
}
func (r *memSessionRepo) List(_ context.Context, f repository.SessionFilter) ([]*model.TaskSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.TaskSession, 0)
	for _, s := range r.sessions {
		if f.TaskID != nil && s.TaskID != *f.TaskID {
			continue
		}
		cp := *s
		out = append(out, &cp)
	}
	return out, nil
}
func (r *memSessionRepo) FindRunningByTask(_ context.Context, taskID uuid.UUID) (*model.TaskSession, error) {
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
func (r *memSessionRepo) SumEffectiveMinutesByTask(_ context.Context, taskID uuid.UUID, now time.Time) (int, error) {
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
func (r *memSessionRepo) SumEffectiveMinutesInRange(_ context.Context, from, to, now time.Time) (int, error) {
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
// Stub task completer for the API harness
// ---------------------------------------------------------------------------

type stubCompleter struct {
	mu    sync.Mutex
	tasks map[uuid.UUID]*model.Task
}

func newStubCompleter(t *model.Task) *stubCompleter {
	return &stubCompleter{tasks: map[uuid.UUID]*model.Task{t.ID: t}}
}
func (s *stubCompleter) Get(_ context.Context, id uuid.UUID) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}
func (s *stubCompleter) MarkCompleted(_ context.Context, id uuid.UUID, mins int) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, model.ErrTaskNotFound
	}
	t.Status = model.StatusCompleted
	t.ActualMinutes = mins
	cp := *t
	return &cp, nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

func newSessionRouter(t *testing.T) (*gin.Engine, *model.Task) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	task := &model.Task{
		ID:       uuid.New(),
		Title:    "Apply",
		Status:   model.StatusPending,
		Priority: model.PriorityMedium,
		Category: model.CategoryJobApply,
	}
	completer := newStubCompleter(task)
	repo := newMemSessionRepo()
	svc := service.NewTaskSessionService(repo, completer, service.SystemClock)
	r := api.NewRouter(api.Deps{
		Config:             config.Config{},
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		TaskSessionService: svc,
	})
	return r, task
}

func doSessReq(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAPI_SessionFullLifecycle(t *testing.T) {
	t.Parallel()
	r, task := newSessionRouter(t)
	startPath := "/api/v1/tasks/" + task.ID.String() + "/sessions/start"

	w := doSessReq(t, r, http.MethodPost, startPath, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("start: %d body=%s", w.Code, w.Body.String())
	}
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	sid := s["id"].(string)
	if s["status"] != "active" {
		t.Errorf("status=%v", s["status"])
	}

	// Second start -> 409
	w = doSessReq(t, r, http.MethodPost, startPath, nil)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate start: want 409, got %d", w.Code)
	}

	// Pause -> 200
	w = doSessReq(t, r, http.MethodPost, "/api/v1/sessions/"+sid+"/pause", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: %d body=%s", w.Code, w.Body.String())
	}
	// Resume -> 200
	w = doSessReq(t, r, http.MethodPost, "/api/v1/sessions/"+sid+"/resume", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: %d body=%s", w.Code, w.Body.String())
	}
	// Complete -> 200
	w = doSessReq(t, r, http.MethodPost, "/api/v1/sessions/"+sid+"/complete", map[string]any{
		"completion_quality": 4,
		"notes":              "good session",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("complete: %d body=%s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	if s["status"] != "completed" {
		t.Errorf("final status=%v", s["status"])
	}

	// Pause after complete -> 409
	w = doSessReq(t, r, http.MethodPost, "/api/v1/sessions/"+sid+"/pause", nil)
	if w.Code != http.StatusConflict {
		t.Errorf("pause-after-complete: want 409, got %d", w.Code)
	}
}

func TestAPI_SessionCurrentAndDelete(t *testing.T) {
	t.Parallel()
	r, task := newSessionRouter(t)

	// no current yet -> 404
	w := doSessReq(t, r, http.MethodGet,
		"/api/v1/tasks/"+task.ID.String()+"/sessions/current", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}

	// start, then current -> 200
	w = doSessReq(t, r, http.MethodPost,
		"/api/v1/tasks/"+task.ID.String()+"/sessions/start", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("start: %d", w.Code)
	}
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	sid := s["id"].(string)

	w = doSessReq(t, r, http.MethodGet,
		"/api/v1/tasks/"+task.ID.String()+"/sessions/current", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("current: %d", w.Code)
	}

	// Delete -> 204, then get -> 404
	w = doSessReq(t, r, http.MethodDelete, "/api/v1/sessions/"+sid, nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("delete: want 204, got %d", w.Code)
	}
	w = doSessReq(t, r, http.MethodGet, "/api/v1/sessions/"+sid, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("get after delete: want 404, got %d", w.Code)
	}
}

func TestAPI_SessionInvalidBody(t *testing.T) {
	t.Parallel()
	r, task := newSessionRouter(t)
	w := doSessReq(t, r, http.MethodPost,
		"/api/v1/tasks/"+task.ID.String()+"/sessions/start", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("start: %d", w.Code)
	}
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	sid := s["id"].(string)

	// quality out of range
	w = doSessReq(t, r, http.MethodPost, "/api/v1/sessions/"+sid+"/complete",
		map[string]any{"completion_quality": 99})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}
