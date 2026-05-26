package web

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// DashboardHandler renders the home dashboard. Each card is rendered
// both at first paint (composed into the full page) and via a dedicated
// HTMX fragment endpoint so the UI can refresh individual cards without
// a full reload.
//
// All service dependencies are interfaces / pointers — the handler does
// no business logic itself; it projects service output into view models.
type DashboardHandler struct {
	rd        *Renderer
	tasks     *service.TaskService
	reviews   *service.DailyReviewService
	reminders *service.ReminderService
	metrics   *service.MetricsService
	suggests  *service.SuggestionService
	clock     service.Clock
	cal       *calendar.Calendar
	log       *slog.Logger
}

// NewDashboardHandler constructs the handler. Any nil dependency is
// tolerated — the corresponding card simply renders empty rather than
// crashing the page, which keeps the dashboard usable while other
// subsystems are still being wired.
func NewDashboardHandler(
	rd *Renderer,
	tasks *service.TaskService,
	reviews *service.DailyReviewService,
	reminders *service.ReminderService,
	metrics *service.MetricsService,
	suggests *service.SuggestionService,
	clock service.Clock,
	cal *calendar.Calendar,
	log *slog.Logger,
) *DashboardHandler {
	if clock == nil {
		clock = service.SystemClock
	}
	if cal == nil {
		cal = calendar.UTC()
	}
	if log == nil {
		log = slog.Default()
	}
	return &DashboardHandler{
		rd: rd, tasks: tasks, reviews: reviews, reminders: reminders,
		metrics: metrics, suggests: suggests, clock: clock, cal: cal, log: log,
	}
}

// Register installs the dashboard routes on the engine.
func (h *DashboardHandler) Register(r *gin.Engine) {
	r.GET("/dashboard", h.page)
	cards := r.Group("/dashboard/cards")
	cards.GET("/summary", h.summaryCard)
	cards.GET("/streak", h.streakCard)
	cards.GET("/activity", h.activityCard)
	cards.GET("/trend", h.trendCard)
	cards.GET("/suggestions", h.suggestionsCard)
}

// ---------------------------------------------------------------------------
// View models
// ---------------------------------------------------------------------------

type DashboardViewModel struct {
	Summary     SummaryCardVM
	Streak      StreakCardVM
	Activity    ActivityCardVM
	Trend       TrendCardVM
	Suggestions SuggestionCardVM
}

type SummaryCardVM struct {
	Date            time.Time
	Total           int
	Completed       int
	Overdue         int
	CarryOver       int
	CompletionPct   int // 0-100 (exact, for aria + label)
	CompletionStep  int // 0,5,10,...,100 — matches the CSS bucket
	HasData         bool
}

type StreakCardVM struct {
	Current        int
	Longest        int
	MissedDays     int
	TodayCount     int
	LastCompletion *time.Time
}

type ActivityItemVM struct {
	Title    string
	Subtitle string
	When     string // already formatted ("2h ago", "Yesterday", "Mon")
	Tone     string // "success", "info", "muted"
}

type ActivityCardVM struct {
	RecentlyCompleted []ActivityItemVM
	RecentReminders   []ActivityItemVM
	LatestReviews     []ActivityItemVM
}

type TrendDayVM struct {
	Date       time.Time
	Label      string // "Mon"
	Count      int
	HeightCls  string // CSS bucket class: trend-bar--h0..h10
	IsToday    bool
}

type TrendCardVM struct {
	Days     []TrendDayVM
	Total    int
	MaxDay   int
}

type SuggestionItemVM struct {
	ID       string
	Kind     string
	Severity string
	Title    string
	Message  string
}

type SuggestionCardVM struct {
	Items []SuggestionItemVM
}

// ---------------------------------------------------------------------------
// Page handler — composes every card at first paint
// ---------------------------------------------------------------------------

func (h *DashboardHandler) page(c *gin.Context) {
	ctx := c.Request.Context()
	vm := DashboardViewModel{
		Summary:     h.buildSummary(ctx),
		Streak:      h.buildStreak(ctx),
		Activity:    h.buildActivity(ctx),
		Trend:       h.buildTrend(ctx),
		Suggestions: h.buildSuggestions(ctx),
	}
	h.rd.Render(c, "dashboard", PageData{
		Title:  "Dashboard",
		Active: "dashboard",
		Data:   vm,
	})
}

// ---------------------------------------------------------------------------
// Card fragment handlers (HTMX endpoints)
// ---------------------------------------------------------------------------

func (h *DashboardHandler) summaryCard(c *gin.Context) {
	h.rd.RenderPartial(c, "dashboard_summary_card", h.buildSummary(c.Request.Context()))
}

func (h *DashboardHandler) streakCard(c *gin.Context) {
	h.rd.RenderPartial(c, "dashboard_streak_card", h.buildStreak(c.Request.Context()))
}

func (h *DashboardHandler) activityCard(c *gin.Context) {
	h.rd.RenderPartial(c, "dashboard_activity_card", h.buildActivity(c.Request.Context()))
}

func (h *DashboardHandler) trendCard(c *gin.Context) {
	h.rd.RenderPartial(c, "dashboard_trend_card", h.buildTrend(c.Request.Context()))
}

func (h *DashboardHandler) suggestionsCard(c *gin.Context) {
	h.rd.RenderPartial(c, "dashboard_suggestions_card", h.buildSuggestions(c.Request.Context()))
}

// ---------------------------------------------------------------------------
// View-model builders
// ---------------------------------------------------------------------------

func (h *DashboardHandler) buildSummary(ctx context.Context) SummaryCardVM {
	now := h.clock.Now()
	vm := SummaryCardVM{Date: now}
	if h.metrics == nil {
		return vm
	}
	today, err := h.metrics.Today(ctx)
	if err != nil {
		h.logErr("dashboard.summary.today", err)
		return vm
	}
	carry, err := h.metrics.TodayCarryOver(ctx)
	if err != nil {
		h.logErr("dashboard.summary.carry", err)
	}
	vm.Total = today.Breakdown.Total()
	vm.Completed = today.Breakdown.Completed
	vm.Overdue = today.OverdueCount
	vm.CarryOver = carry
	vm.CompletionPct = int(today.CompletionRate*100 + 0.5)
	vm.CompletionStep = bucketTo5(vm.CompletionPct)
	vm.HasData = vm.Total > 0
	return vm
}

// bucketTo5 snaps n in [0,100] to the nearest multiple of 5 in [0,100].
func bucketTo5(n int) int {
	if n < 0 {
		n = 0
	}
	if n > 100 {
		n = 100
	}
	b := (n + 2) / 5 * 5
	if b > 100 {
		b = 100
	}
	return b
}

func (h *DashboardHandler) buildStreak(ctx context.Context) StreakCardVM {
	vm := StreakCardVM{}
	if h.metrics == nil {
		return vm
	}
	s, err := h.metrics.Streak(ctx)
	if err != nil {
		h.logErr("dashboard.streak", err)
		return vm
	}
	vm.Current = s.CurrentStreak
	vm.Longest = s.LongestStreak
	vm.MissedDays = s.MissedDayCount
	vm.TodayCount = s.TodayCompletedCount
	if !s.LastCompletionDate.IsZero() {
		t := s.LastCompletionDate
		vm.LastCompletion = &t
	}
	return vm
}

func (h *DashboardHandler) buildActivity(ctx context.Context) ActivityCardVM {
	now := h.clock.Now()
	vm := ActivityCardVM{
		RecentlyCompleted: []ActivityItemVM{},
		RecentReminders:   []ActivityItemVM{},
		LatestReviews:     []ActivityItemVM{},
	}

	if h.tasks != nil {
		tasks, err := h.tasks.List(ctx, repository.TaskFilter{
			Statuses: []model.Status{model.StatusCompleted},
			Limit:    5,
		})
		if err != nil {
			h.logErr("dashboard.activity.tasks", err)
		}
		for _, t := range tasks {
			when := t.CreatedAt
			if t.CompletedAt != nil {
				when = *t.CompletedAt
			}
			vm.RecentlyCompleted = append(vm.RecentlyCompleted, ActivityItemVM{
				Title:    t.Title,
				Subtitle: string(t.Category),
				When:     relativeTime(when, now),
				Tone:     "success",
			})
		}
	}

	if h.reminders != nil {
		rs, err := h.reminders.List(ctx, repository.ReminderFilter{Limit: 5})
		if err != nil {
			h.logErr("dashboard.activity.reminders", err)
		}
		for _, r := range rs {
			vm.RecentReminders = append(vm.RecentReminders, ActivityItemVM{
				Title:    humanReminderKind(r.Kind),
				Subtitle: string(r.Status),
				When:     relativeTime(r.ScheduledFor, now),
				Tone:     reminderTone(r.Status),
			})
		}
	}

	if h.reviews != nil {
		rvs, err := h.reviews.List(ctx, repository.ReviewFilter{Limit: 3})
		if err != nil {
			h.logErr("dashboard.activity.reviews", err)
		}
		for _, r := range rvs {
			snippet := r.Reflection
			if snippet == "" {
				snippet = "(no reflection)"
			}
			vm.LatestReviews = append(vm.LatestReviews, ActivityItemVM{
				Title:    r.ReviewDate.Format("Mon, Jan 2"),
				Subtitle: truncate(snippet, 80),
				When:     relativeTime(r.ReviewDate, now),
				Tone:     "info",
			})
		}
	}

	return vm
}

func (h *DashboardHandler) buildTrend(ctx context.Context) TrendCardVM {
	vm := TrendCardVM{Days: []TrendDayVM{}}
	if h.metrics == nil {
		return vm
	}
	weekly, err := h.metrics.Weekly(ctx)
	if err != nil {
		h.logErr("dashboard.trend", err)
		return vm
	}
	max := 0
	for _, d := range weekly.DailyCompletions {
		if d.Count > max {
			max = d.Count
		}
		vm.Total += d.Count
	}
	vm.MaxDay = max
	today := h.cal.StartOfDay(h.clock.Now())
	for _, d := range weekly.DailyCompletions {
		bucket := 0
		if max > 0 {
			// 10 buckets, integer division on percentage
			pct := int(float64(d.Count) / float64(max) * 10)
			if pct > 10 {
				pct = 10
			}
			bucket = pct
		}
		vm.Days = append(vm.Days, TrendDayVM{
			Date:      d.Date,
			Label:     d.Date.Format("Mon"),
			Count:     d.Count,
			HeightCls: "trend-bar--h" + itoa(bucket),
			IsToday:   d.Date.Equal(today),
		})
	}
	return vm
}

func (h *DashboardHandler) buildSuggestions(ctx context.Context) SuggestionCardVM {
	vm := SuggestionCardVM{Items: []SuggestionItemVM{}}
	if h.suggests == nil {
		return vm
	}
	active, err := h.suggests.ListActive(ctx)
	if err != nil {
		h.logErr("dashboard.suggestions", err)
		return vm
	}
	for _, s := range active {
		vm.Items = append(vm.Items, SuggestionItemVM{
			ID:       s.ID.String(),
			Kind:     string(s.Kind),
			Severity: string(s.Severity),
			Title:    s.Title,
			Message:  s.Message,
		})
	}
	return vm
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

func (h *DashboardHandler) logErr(op string, err error) {
	if err == nil || h.log == nil {
		return
	}
	h.log.Warn("dashboard render", slog.String("op", op), slog.String("error", err.Error()))
}

func relativeTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < 0:
		// future: render the actual time-of-day
		return t.Format("Mon 15:04")
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + "h ago"
	case d < 7*24*time.Hour:
		return itoa(int(d.Hours()/24)) + "d ago"
	default:
		return t.Format("Jan 2")
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func humanReminderKind(k model.ReminderKind) string {
	switch k {
	case model.ReminderKindMorning:
		return "Morning reminder"
	case model.ReminderKindEveningReview:
		return "Evening review"
	case model.ReminderKindWeeklyReview:
		return "Weekly review"
	case model.ReminderKindOverdue:
		return "Overdue task"
	}
	return string(k)
}

func reminderTone(s model.ReminderStatus) string {
	switch s {
	case model.ReminderStatusSent:
		return "success"
	case model.ReminderStatusFailed:
		return "warning"
	case model.ReminderStatusCancelled:
		return "muted"
	}
	return "info"
}

// itoa avoids strconv to keep this file dep-free for the small ints we
// handle. It's only used for non-negative ints up to a few thousand.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

var _ = http.MethodGet
