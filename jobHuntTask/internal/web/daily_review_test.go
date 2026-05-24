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

type reviewHarness struct {
	router   *gin.Engine
	reviews  *memReviewRepo
	tasks    *inMemTaskRepo
	sessions *memSessionRepo
	clock    *fixedReviewClock
}

type fixedReviewClock struct{ t time.Time }

func (c *fixedReviewClock) Now() time.Time { return c.t }

type memReviewRepo struct {
	mu     sync.Mutex
	byDate map[time.Time]*model.DailyReview
}

func newMemReviewRepo() *memReviewRepo {
	return &memReviewRepo{byDate: map[time.Time]*model.DailyReview{}}
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
	return nil, model.ErrReviewNotFound
}
func (r *memReviewRepo) List(_ context.Context, _ repository.ReviewFilter) ([]*model.DailyReview, error) {
	return nil, nil
}
func (r *memReviewRepo) Delete(_ context.Context, _ time.Time) error { return nil }

type memSessionRepo struct {
	mu       sync.Mutex
	sessions []*model.TaskSession
}

func (r *memSessionRepo) Create(_ context.Context, s *model.TaskSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s.ID = uuid.New()
	r.sessions = append(r.sessions, s)
	return nil
}
func (r *memSessionRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.TaskSession, error) {
	return nil, model.ErrSessionNotFound
}
func (r *memSessionRepo) Update(_ context.Context, _ uuid.UUID, _ repository.SessionUpdate) (*model.TaskSession, error) {
	return nil, model.ErrSessionNotFound
}
func (r *memSessionRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (r *memSessionRepo) List(_ context.Context, _ repository.SessionFilter) ([]*model.TaskSession, error) {
	return nil, nil
}
func (r *memSessionRepo) FindRunningByTask(_ context.Context, _ uuid.UUID) (*model.TaskSession, error) {
	return nil, model.ErrSessionNotFound
}
func (r *memSessionRepo) SumEffectiveMinutesByTask(_ context.Context, _ uuid.UUID, _ time.Time) (int, error) {
	return 0, nil
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

func newReviewHarness(t *testing.T) *reviewHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 5, 24, 20, 0, 0, 0, time.UTC)
	clk := &fixedReviewClock{t: now}

	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	reviews := newMemReviewRepo()
	tasks := newInMemTaskRepo()
	sessions := &memSessionRepo{}

	reviewSvc := service.NewDailyReviewService(reviews, clk)
	taskSvc := service.NewTaskService(tasks, clk)
	sessionSvc := service.NewTaskSessionService(sessions, taskSvc, clk)

	h := web.NewDailyReviewHandler(rd, reviewSvc, taskSvc, sessionSvc, clk,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := gin.New()
	if err := web.MountStatic(r); err != nil {
		t.Fatalf("MountStatic: %v", err)
	}
	web.RegisterRoutes(r, rd)
	h.Register(r)
	return &reviewHarness{router: r, reviews: reviews, tasks: tasks, sessions: sessions, clock: clk}
}

func TestDailyReviewPage_RendersJournalAndSnapshot(t *testing.T) {
	t.Parallel()
	h := newReviewHarness(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reviews/daily", nil)
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%.300s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"review-journal", "review-snapshot", "Reflections", "Wins", "Blockers",
		"Distractions", "Notes", "Energy level", "execution minutes",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestDailyReview_AutosaveReflection(t *testing.T) {
	t.Parallel()
	h := newReviewHarness(t)
	form := url.Values{}
	form.Set("reflection", "Good focus day overall.")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/reviews/daily/autosave?section=reflection", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Draft saved") {
		t.Errorf("expected save status, got: %s", w.Body.String())
	}
	rv, _ := h.reviews.GetByDate(context.Background(), h.clock.Now())
	if rv.Reflection != "Good focus day overall." {
		t.Errorf("reflection=%q", rv.Reflection)
	}
}

func TestDailyReview_AutosaveWinsLines(t *testing.T) {
	t.Parallel()
	h := newReviewHarness(t)
	form := url.Values{}
	form.Set("wins", "Applied to 3 roles\nFinished portfolio")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/reviews/daily/autosave?section=wins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	rv, _ := h.reviews.GetByDate(context.Background(), h.clock.Now())
	if len(rv.Wins) != 2 {
		t.Fatalf("wins=%v", rv.Wins)
	}
}

func TestDailyReview_SubmitAllFields(t *testing.T) {
	t.Parallel()
	h := newReviewHarness(t)
	form := url.Values{}
	form.Set("reflection", "Reflect")
	form.Set("wins", "Win one")
	form.Set("blockers", "Block one")
	form.Set("distractions", "Twitter")
	form.Set("notes", "Note here")
	form.Set("energy_level", "7")
	form.Set("productivity_score", "8")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/reviews/daily/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	rv, _ := h.reviews.GetByDate(context.Background(), h.clock.Now())
	if rv.Notes != "Note here" || rv.EnergyLevel != 7 || len(rv.Distractions) != 1 {
		t.Errorf("review=%+v", rv)
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "toast") {
		t.Error("expected toast trigger on submit")
	}
}

func TestDailyReview_SnapshotShowsTasksAndMinutes(t *testing.T) {
	t.Parallel()
	h := newReviewHarness(t)
	now := h.clock.Now()
	completedAt := now.Add(-2 * time.Hour)
	duePast := now.Add(-24 * time.Hour)
	dueFuture := now.Add(4 * time.Hour)

	_ = h.tasks.Create(context.Background(), &model.Task{
		Title: "Done task", Status: model.StatusCompleted, Category: model.CategoryGithub,
		CompletedAt: &completedAt, UpdatedAt: completedAt, CreatedAt: completedAt,
	})
	_ = h.tasks.Create(context.Background(), &model.Task{
		Title: "Open task", Status: model.StatusPending, Category: model.CategoryLearning,
		DueDate: &dueFuture, CreatedAt: now,
	})
	_ = h.tasks.Create(context.Background(), &model.Task{
		Title: "Late task", Status: model.StatusInProgress, Category: model.CategoryJobApply,
		DueDate: &duePast, CreatedAt: now,
	})
	_ = h.sessions.Create(context.Background(), &model.TaskSession{
		TaskID: uuid.New(), Status: model.SessionStatusCompleted,
		StartedAt: now.Add(-90 * time.Minute), EndedAt: ptrTime(now.Add(-30 * time.Minute)),
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reviews/daily/snapshot", nil)
	req.Header.Set("HX-Request", "true")
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Done task") {
		t.Error("missing completed task")
	}
	if !strings.Contains(body, "Open task") {
		t.Error("missing unfinished task")
	}
	if !strings.Contains(body, "Late task") {
		t.Error("missing overdue task")
	}
	if !strings.Contains(body, "execution minutes") {
		t.Error("missing execution minutes label")
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
