package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/web"
)

// All page names registered in routes.go. Update this list when adding a
// new page so the smoke tests cover it.
var pageRoutes = []struct {
	name string
	path string
	mustContain string
}{
	{"dashboard",    "/dashboard",      "Dashboard"},
	{"tasks",        "/tasks",          "Tasks"},
	{"daily_review", "/reviews/daily",  "Daily Review"},
	{"weekly_review","/reviews/weekly", "Weekly Review"},
	{"analytics",    "/analytics",      "Analytics"},
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rd, err := web.New()
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	r := gin.New()
	if err := web.MountStatic(r); err != nil {
		t.Fatalf("MountStatic: %v", err)
	}
	web.RegisterRoutes(r, rd)
	web.RegisterDashboardFallback(r, rd)
	web.RegisterTasksFallback(r, rd)
	web.RegisterDailyReviewFallback(r, rd)
	web.RegisterWeeklyReviewFallback(r, rd)
	web.RegisterAnalyticsFallback(r, rd)
	return r
}

func TestPagesRenderFullLayout(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t)

	for _, tc := range pageRoutes {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			if !strings.Contains(body, "<!DOCTYPE html>") {
				t.Error("full page is missing <!DOCTYPE html>")
			}
			if !strings.Contains(body, `id="sidebar"`) {
				t.Error("full page is missing sidebar element")
			}
			if !strings.Contains(body, `id="main-content"`) {
				t.Error("full page is missing main-content landmark")
			}
			if !strings.Contains(body, tc.mustContain) {
				t.Errorf("expected page to contain %q, got body fragment: %.300s", tc.mustContain, body)
			}
		})
	}
}

func TestPagesRenderHTMXFragment(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t)

	for _, tc := range pageRoutes {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			if strings.Contains(body, "<!DOCTYPE html>") {
				t.Error("fragment should NOT include doctype")
			}
			if strings.Contains(body, `id="sidebar"`) {
				t.Error("fragment should NOT include sidebar")
			}
			if !strings.Contains(body, tc.mustContain) {
				t.Errorf("expected fragment to contain %q, got body fragment: %.300s", tc.mustContain, body)
			}
			if w.Header().Get("HX-Push-Url") != tc.path {
				t.Errorf("HX-Push-Url = %q, want %q", w.Header().Get("HX-Push-Url"), tc.path)
			}
		})
	}
}

func TestRootRedirectsToDashboard(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", loc)
	}
}

func TestSidebarActiveHighlightServerSide(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, `data-nav="/tasks"`) {
		t.Fatal("sidebar missing data-nav for tasks")
	}
	// The active link must carry aria-current="page".
	// Look for the segment of HTML around /tasks containing aria-current.
	tasksIdx := strings.Index(body, `data-nav="/tasks"`)
	if tasksIdx == -1 {
		t.Fatal("could not find tasks nav link")
	}
	chunk := body[tasksIdx : tasksIdx+400]
	if !strings.Contains(chunk, `aria-current="page"`) {
		t.Errorf("tasks link lacks aria-current=page; chunk=%s", chunk)
	}
}

func TestStaticAssetsServed(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t)

	cases := []struct {
		path string
		needle string
	}{
		{"/static/css/app.css", "--accent"},
		{"/static/js/app.js",   "JobHuntToast"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d", w.Code)
			}
			if !strings.Contains(w.Body.String(), tc.needle) {
				t.Errorf("expected asset to contain %q", tc.needle)
			}
		})
	}
}
