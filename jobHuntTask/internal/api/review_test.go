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
// Tiny in-memory review repo
// ---------------------------------------------------------------------------

type memReviewRepo struct {
	mu     sync.Mutex
	byDate map[time.Time]*model.DailyReview
	byID   map[uuid.UUID]*model.DailyReview
}

func newMemReviewRepo() *memReviewRepo {
	return &memReviewRepo{
		byDate: map[time.Time]*model.DailyReview{},
		byID:   map[uuid.UUID]*model.DailyReview{},
	}
}

func (r *memReviewRepo) Upsert(_ context.Context, rv *model.DailyReview) error {
	if err := rv.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rv.ReviewDate = model.NormalizeDate(rv.ReviewDate)
	if existing, ok := r.byDate[rv.ReviewDate]; ok {
		rv.ID = existing.ID
		rv.CreatedAt = existing.CreatedAt
	} else {
		rv.ID = uuid.New()
		rv.CreatedAt = time.Now()
	}
	rv.UpdatedAt = time.Now()
	cp := *rv
	r.byDate[rv.ReviewDate] = &cp
	r.byID[rv.ID] = &cp
	return nil
}
func (r *memReviewRepo) GetByDate(_ context.Context, date time.Time) (*model.DailyReview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rv, ok := r.byDate[model.NormalizeDate(date)]
	if !ok {
		return nil, model.ErrReviewNotFound
	}
	cp := *rv
	return &cp, nil
}
func (r *memReviewRepo) GetByID(_ context.Context, id uuid.UUID) (*model.DailyReview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rv, ok := r.byID[id]
	if !ok {
		return nil, model.ErrReviewNotFound
	}
	cp := *rv
	return &cp, nil
}
func (r *memReviewRepo) List(_ context.Context, _ repository.ReviewFilter) ([]*model.DailyReview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.DailyReview, 0, len(r.byDate))
	for _, rv := range r.byDate {
		cp := *rv
		out = append(out, &cp)
	}
	return out, nil
}
func (r *memReviewRepo) Delete(_ context.Context, date time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := model.NormalizeDate(date)
	rv, ok := r.byDate[d]
	if !ok {
		return model.ErrReviewNotFound
	}
	delete(r.byDate, d)
	delete(r.byID, rv.ID)
	return nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

func newReviewRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newMemReviewRepo()
	svc := service.NewDailyReviewService(repo, service.SystemClock)
	return api.NewRouter(api.Deps{
		Config:        config.Config{},
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		ReviewService: svc,
	})
}

func doReviewReq(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
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

func TestAPI_ReviewUpsertCreatesAndUpdates(t *testing.T) {
	t.Parallel()
	r := newReviewRouter(t)
	path := "/api/v1/reviews/2026-05-24"

	w := doReviewReq(t, r, http.MethodPut, path, map[string]any{
		"reflection":         "focused",
		"blockers":           []string{"recruiter ghosted"},
		"wins":               []string{"3 apps"},
		"energy_level":       7,
		"productivity_score": 8,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created["review_date"] != "2026-05-24" {
		t.Errorf("review_date wrong: %v", created["review_date"])
	}

	// partial update — only energy
	w = doReviewReq(t, r, http.MethodPut, path, map[string]any{"energy_level": 3})
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d", w.Code)
	}
	var updated map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated["energy_level"].(float64) != 3 {
		t.Errorf("energy not updated")
	}
	if updated["reflection"] != "focused" {
		t.Errorf("reflection clobbered: %v", updated["reflection"])
	}
}

func TestAPI_ReviewValidation(t *testing.T) {
	t.Parallel()
	r := newReviewRouter(t)
	w := doReviewReq(t, r, http.MethodPut, "/api/v1/reviews/2026-05-24", map[string]any{
		"energy_level": 99,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAPI_ReviewBadDate(t *testing.T) {
	t.Parallel()
	r := newReviewRouter(t)
	w := doReviewReq(t, r, http.MethodGet, "/api/v1/reviews/not-a-date", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAPI_ReviewGetNotFound(t *testing.T) {
	t.Parallel()
	r := newReviewRouter(t)
	w := doReviewReq(t, r, http.MethodGet, "/api/v1/reviews/2020-01-01", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestAPI_ReviewList(t *testing.T) {
	t.Parallel()
	r := newReviewRouter(t)
	_ = doReviewReq(t, r, http.MethodPut, "/api/v1/reviews/2026-05-22", map[string]any{"energy_level": 6})
	_ = doReviewReq(t, r, http.MethodPut, "/api/v1/reviews/2026-05-23", map[string]any{"energy_level": 7})

	w := doReviewReq(t, r, http.MethodGet, "/api/v1/reviews", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["count"].(float64) != 2 {
		t.Errorf("count = %v", out["count"])
	}
}
