package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/service"
)

// RegisterRoutes mounts the page routes on the given engine.
//
// /dashboard and /tasks are owned by dedicated handlers (DashboardHandler
// / TasksHandler) when wired. Pages registered here are placeholder
// shells; they'll be wired to their respective services in later steps.
//
// Conventions:
//   - GET /                  redirects to /dashboard
func RegisterRoutes(r *gin.Engine, rd *Renderer) {
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard")
	})
}

// RegisterTasksFallback wires a static /tasks route — used by the web
// smoke tests where TaskService isn't available. Production code installs
// the real (*TasksHandler).Register instead.
func RegisterTasksFallback(r *gin.Engine, rd *Renderer) {
	r.GET("/tasks", func(c *gin.Context) {
		rd.Render(c, "tasks", PageData{
			Title:  "Tasks",
			Active: "tasks",
			Data:   TasksPageVM{Query: tasksQuery{View: viewToday, Sort: "due_date", SortDir: "asc"}},
		})
	})
}

// RegisterDailyReviewFallback wires a static daily review page for smoke
// tests when services aren't available.
func RegisterDailyReviewFallback(r *gin.Engine, rd *Renderer) {
	r.GET("/reviews/daily", func(c *gin.Context) {
		rd.Render(c, "daily_review", PageData{
			Title:  "Daily Review",
			Active: "daily_review",
			Data: DailyReviewPageVM{
				Date:         time.Now().UTC(),
				DateLabel:    time.Now().UTC().Format("Monday, Jan 2, 2006"),
				DateInput:    time.Now().UTC().Format("2006-01-02"),
				IsToday:      true,
				SnapshotPath: "/reviews/daily/snapshot",
				SaveStatus:   ReviewSaveStatusVM{State: "idle", Message: "Edits autosave as you type"},
				Snapshot: ReviewSnapshotVM{
					Completed: []ReviewTaskItemVM{},
					Unfinished: []ReviewTaskItemVM{},
					Overdue:   []ReviewTaskItemVM{},
				},
			},
		})
	})
}

// RegisterWeeklyReviewFallback wires a static weekly review for smoke tests.
func RegisterWeeklyReviewFallback(r *gin.Engine, rd *Renderer) {
	r.GET("/reviews/weekly", func(c *gin.Context) {
		ws := time.Now().UTC().AddDate(0, 0, -6)
		ws = time.Date(ws.Year(), ws.Month(), ws.Day(), 0, 0, 0, 0, time.UTC)
		end := ws.AddDate(0, 0, 6)
		rd.Render(c, "weekly_review", PageData{
			Title:  "Weekly Review",
			Active: "weekly_review",
			Data: WeeklyReviewPageVM{
				WeekLabel: ws.Format("Jan 2") + " – " + end.Format("Jan 2, 2006"),
				WeekInput: ws.Format("2006-01-02"),
				IsCurrent: true,
				Stats:     WeeklyStatsVM{WeekInput: ws.Format("2006-01-02")},
				Streak:    WeeklyStreakVM{WeekInput: ws.Format("2006-01-02")},
				Categories: WeeklyCategoriesVM{WeekInput: ws.Format("2006-01-02")},
				Charts:    WeeklyChartsVM{WeekInput: ws.Format("2006-01-02")},
				Suggestions: WeeklySuggestionsVM{Items: []WeeklySuggestionItemVM{}},
				Notes:     WeeklyNotesVM{WeekInput: ws.Format("2006-01-02")},
				SaveStatus: ReviewSaveStatusVM{State: "idle", Message: "Notes autosave as you type"},
			},
		})
	})
}

// RegisterAnalyticsFallback wires a static analytics page for smoke tests.
func RegisterAnalyticsFallback(r *gin.Engine, rd *Renderer) {
	r.GET("/analytics", func(c *gin.Context) {
		rd.Render(c, "analytics", PageData{
			Title:  "Analytics",
			Active: "analytics",
			Data: AnalyticsPageVM{
				Range:      service.AnalyticsRange7,
				RangeLabel: service.AnalyticsRange7.Label(),
				KPIs:       AnalyticsKPIsVM{Range: "7"},
				Comparison: AnalyticsComparisonVM{Range: "7", PeriodLabel: "Last 7 days"},
				Completion: AnalyticsChartVM{Range: "7", ChartID: "chart-completion", Empty: true, EmptyTitle: "No data", EmptyMsg: "Complete tasks first."},
				Carry:      AnalyticsChartVM{Range: "7", ChartID: "chart-carry", Empty: true, EmptyTitle: "No data", EmptyMsg: ""},
				Category:   AnalyticsChartVM{Range: "7", ChartID: "chart-category", Empty: true, EmptyTitle: "No data", EmptyMsg: ""},
				Productivity: AnalyticsChartVM{Range: "7", ChartID: "chart-productivity", Empty: true, EmptyTitle: "No data", EmptyMsg: ""},
				Overdue:    AnalyticsChartVM{Range: "7", ChartID: "chart-overdue", Empty: true, EmptyTitle: "No data", EmptyMsg: ""},
				Execution:  AnalyticsChartVM{Range: "7", ChartID: "chart-execution", Empty: true, EmptyTitle: "No data", EmptyMsg: ""},
				Streak:     AnalyticsChartVM{Range: "7", ChartID: "chart-streak", Empty: true, EmptyTitle: "No data", EmptyMsg: ""},
			},
		})
	})
}

// RegisterDashboardFallback registers a no-data dashboard route (full-page
// only, no card endpoints) when the caller doesn't have services wired
// yet. The smoke tests use this; production wiring uses DashboardHandler.
func RegisterDashboardFallback(r *gin.Engine, rd *Renderer) {
	r.GET("/dashboard", func(c *gin.Context) {
		rd.Render(c, "dashboard", PageData{
			Title:  "Dashboard",
			Active: "dashboard",
			Data:   DashboardViewModel{},
		})
	})
}
