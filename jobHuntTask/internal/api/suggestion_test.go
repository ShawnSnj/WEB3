package api_test

import (
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
// Fake suggestion repo (api-test variant — pre-seeded, no rule wiring)
// ---------------------------------------------------------------------------

type apiFakeSuggestionRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]*model.Suggestion
}

func newAPIFakeSuggestionRepo() *apiFakeSuggestionRepo {
	return &apiFakeSuggestionRepo{byID: map[uuid.UUID]*model.Suggestion{}}
}

func (r *apiFakeSuggestionRepo) Upsert(_ context.Context, s *model.Suggestion) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	cp := *s
	r.byID[s.ID] = &cp
	return true, nil
}
func (r *apiFakeSuggestionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Suggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.byID[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, model.ErrSuggestionNotFound
}
func (r *apiFakeSuggestionRepo) List(_ context.Context, f repository.SuggestionFilter) ([]*model.Suggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Suggestion, 0, len(r.byID))
	for _, s := range r.byID {
		match := true
		if len(f.Statuses) > 0 {
			match = false
			for _, st := range f.Statuses {
				if s.Status == st {
					match = true
					break
				}
			}
		}
		if match {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *apiFakeSuggestionRepo) Dismiss(_ context.Context, id uuid.UUID, at time.Time) (*model.Suggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok || s.Status != model.SuggestionStatusActive {
		return nil, model.ErrSuggestionNotFound
	}
	s.Status = model.SuggestionStatusDismissed
	s.DismissedAt = &at
	cp := *s
	return &cp, nil
}
func (r *apiFakeSuggestionRepo) ExpireActiveExcept(_ context.Context, _ []model.SuggestionKind, _ time.Time) (int, error) {
	return 0, nil
}
func (r *apiFakeSuggestionRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return model.ErrSuggestionNotFound
	}
	delete(r.byID, id)
	return nil
}

// ---------------------------------------------------------------------------
// Stub metrics repo (no rules will fire by default)
// ---------------------------------------------------------------------------

type apiStubMetricsRepo struct{}

func (apiStubMetricsRepo) StatusBreakdown(_ context.Context, _, _ time.Time) (model.StatusBreakdown, error) {
	return model.StatusBreakdown{}, nil
}
func (apiStubMetricsRepo) CompletionCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return model.Counts{}, nil
}
func (apiStubMetricsRepo) CarryOverCounts(_ context.Context, _, _ time.Time) (model.Counts, error) {
	return model.Counts{}, nil
}
func (apiStubMetricsRepo) OverdueLive(_ context.Context, _ time.Time) (int, error) { return 0, nil }
func (apiStubMetricsRepo) AvgActualMinutes(_ context.Context, _, _ time.Time) (float64, error) {
	return 0, nil
}
func (apiStubMetricsRepo) CategoryStats(_ context.Context, _, _ time.Time) ([]model.CategoryStats, error) {
	return nil, nil
}
func (apiStubMetricsRepo) MostMissedCategory(_ context.Context, _, _ time.Time) (*model.CategoryMissed, error) {
	return nil, nil
}
func (apiStubMetricsRepo) DailyCompletionCounts(_ context.Context, _, _ time.Time) ([]model.DailyCompletion, error) {
	return nil, nil
}
func (apiStubMetricsRepo) EffortDistribution(_ context.Context, _, _ time.Time, _ int) (float64, int, int, error) {
	return 0, 0, 0, nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

func newSuggestionRouter(t *testing.T) (*gin.Engine, *apiFakeSuggestionRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newAPIFakeSuggestionRepo()
	metricsRepo := apiStubMetricsRepo{}
	metricsSvc := service.NewMetricsService(metricsRepo, service.SystemClock)
	svc := service.NewSuggestionService(repo, metricsRepo, metricsSvc, nil, service.SystemClock, service.SuggestionServiceConfig{})
	r := api.NewRouter(api.Deps{
		Config:            config.Config{},
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		SuggestionService: svc,
	})
	return r, repo
}

func seedActiveSuggestion(repo *apiFakeSuggestionRepo, kind model.SuggestionKind) *model.Suggestion {
	s := &model.Suggestion{
		ID:          uuid.New(),
		Kind:        kind,
		Severity:    model.SeverityWarning,
		Status:      model.SuggestionStatusActive,
		Title:       "title",
		Message:     "msg",
		DedupKey:    string(kind) + ":2026-W21",
		GeneratedAt: time.Now(),
		Payload:     map[string]any{},
	}
	repo.byID[s.ID] = s
	return s
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAPI_Suggestion_ListDefaultsToActive(t *testing.T) {
	t.Parallel()
	r, repo := newSuggestionRouter(t)
	a := seedActiveSuggestion(repo, model.SuggestionReduceWorkload)
	b := seedActiveSuggestion(repo, model.SuggestionEasierWins)
	b.Status = model.SuggestionStatusDismissed
	_ = a

	req := httptest.NewRequest(http.MethodGet, "/api/v1/suggestions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["count"].(float64) != 1 {
		t.Fatalf("count = %v, want 1", resp["count"])
	}
}

func TestAPI_Suggestion_ListInvalidStatus(t *testing.T) {
	t.Parallel()
	r, _ := newSuggestionRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/suggestions?status=bogus", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAPI_Suggestion_Dismiss(t *testing.T) {
	t.Parallel()
	r, repo := newSuggestionRouter(t)
	s := seedActiveSuggestion(repo, model.SuggestionReduceWorkload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/suggestions/"+s.ID.String()+"/dismiss", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["status"] != "dismissed" {
		t.Errorf("status field = %v, want dismissed", got["status"])
	}
}

func TestAPI_Suggestion_DismissAlreadyDismissed(t *testing.T) {
	t.Parallel()
	r, repo := newSuggestionRouter(t)
	s := seedActiveSuggestion(repo, model.SuggestionReduceWorkload)
	s.Status = model.SuggestionStatusDismissed

	req := httptest.NewRequest(http.MethodPost, "/api/v1/suggestions/"+s.ID.String()+"/dismiss", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", w.Code, w.Body.String())
	}
}

func TestAPI_Suggestion_Refresh(t *testing.T) {
	t.Parallel()
	r, _ := newSuggestionRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/suggestions/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["created_count"]; !ok {
		t.Error("response missing created_count")
	}
	if _, ok := resp["expired_count"]; !ok {
		t.Error("response missing expired_count")
	}
	// With an empty stub the streak is 0, so EasierWinsRule fires.
	if resp["created_count"].(float64) < 1 {
		t.Errorf("expected at least 1 firing rule on empty stub, got %v", resp["created_count"])
	}
}

func TestAPI_Suggestion_GetNotFound(t *testing.T) {
	t.Parallel()
	r, _ := newSuggestionRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/suggestions/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestAPI_Suggestion_InvalidID(t *testing.T) {
	t.Parallel()
	r, _ := newSuggestionRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/suggestions/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
