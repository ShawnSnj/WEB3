package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// WeeklyReviewHandler renders the report-style weekly review and exposes
// HTMX fragment endpoints for refreshing individual sections.
type WeeklyReviewHandler struct {
	rd        *Renderer
	reviews   *service.WeeklyReviewService
	metrics   *service.MetricsService
	sessions  *service.TaskSessionService
	suggests  *service.SuggestionService
	clock     service.Clock
	log       *slog.Logger
}

func NewWeeklyReviewHandler(
	rd *Renderer,
	reviews *service.WeeklyReviewService,
	metrics *service.MetricsService,
	sessions *service.TaskSessionService,
	suggests *service.SuggestionService,
	clock service.Clock,
	log *slog.Logger,
) *WeeklyReviewHandler {
	if clock == nil {
		clock = service.SystemClock
	}
	if log == nil {
		log = slog.Default()
	}
	return &WeeklyReviewHandler{
		rd: rd, reviews: reviews, metrics: metrics, sessions: sessions,
		suggests: suggests, clock: clock, log: log,
	}
}

func (h *WeeklyReviewHandler) Register(r *gin.Engine) {
	r.GET("/reviews/weekly", h.page)
	g := r.Group("/reviews/weekly")
	g.GET("/cards/stats", h.statsCard)
	g.GET("/cards/streak", h.streakCard)
	g.GET("/cards/categories", h.categoriesCard)
	g.GET("/cards/charts", h.chartsCard)
	g.GET("/cards/suggestions", h.suggestionsCard)
	g.GET("/sections/notes", h.notesSection)
	g.PATCH("/autosave", h.autosave)
}

// ---------------------------------------------------------------------------
// View models
// ---------------------------------------------------------------------------

type WeeklyReviewPageVM struct {
	WeekStart    time.Time
	WeekEnd      time.Time
	WeekLabel    string
	WeekInput    string
	PrevPath     string
	NextPath     string
	IsCurrent    bool
	Stats        WeeklyStatsVM
	Streak       WeeklyStreakVM
	Categories   WeeklyCategoriesVM
	Charts       WeeklyChartsVM
	Suggestions  WeeklySuggestionsVM
	Notes        WeeklyNotesVM
	SaveStatus   ReviewSaveStatusVM
}

type WeeklyStatsVM struct {
	WeekInput       string
	CompletionPct   int
	CompletedCount  int
	TotalTasks      int
	MissedCount     int
	CarryOverPct    int
	CarryOverCount  int
	CarryOverDelta  int // vs previous week, percentage points
	FocusMinutes    int
	FocusAvgPerDay  int
}

type WeeklyStreakVM struct {
	WeekInput  string
	Current    int
	Longest    int
	MissedDays int
}

type WeeklyCategoriesVM struct {
	WeekInput      string
	StrongestName  string
	StrongestPct   int
	WeakestName    string
	WeakestPct     int
	HasData        bool
}

type WeeklyChartDayVM struct {
	Label     string
	Count     int
	HeightCls string
}

type WeeklyChartsVM struct {
	WeekInput       string
	CompletionDays  []WeeklyChartDayVM
	CompletionTotal int
	CarryThisPct   int
	CarryPrevPct   int
	HasCompletion  bool
}

type WeeklySuggestionItemVM struct {
	Title    string
	Message  string
	Severity string
}

type WeeklySuggestionsVM struct {
	Items []WeeklySuggestionItemVM
}

type WeeklyNotesVM struct {
	WeekInput          string
	Wins               string
	Bottlenecks        string
	ImprovementNotes   string
	NextWeekPriorities string
}

type weeklySection string

const (
	wSectionWins        weeklySection = "wins"
	wSectionBottlenecks weeklySection = "bottlenecks"
	wSectionImprovement weeklySection = "improvement"
	wSectionPriorities  weeklySection = "priorities"
)

func (s weeklySection) valid() bool {
	switch s {
	case wSectionWins, wSectionBottlenecks, wSectionImprovement, wSectionPriorities:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *WeeklyReviewHandler) page(c *gin.Context) {
	ws := h.parseWeekStart(c)
	vm := h.buildPage(c.Request.Context(), ws)
	h.rd.Render(c, "weekly_review", PageData{
		Title:  "Weekly Review",
		Active: "weekly_review",
		Data:   vm,
	})
}

func (h *WeeklyReviewHandler) statsCard(c *gin.Context) {
	h.rd.RenderPartial(c, "weekly_stats_card", h.buildStats(c.Request.Context(), h.parseWeekStart(c)))
}

func (h *WeeklyReviewHandler) streakCard(c *gin.Context) {
	ws := h.parseWeekStart(c)
	h.rd.RenderPartial(c, "weekly_streak_card", h.buildStreak(c.Request.Context(), ws))
}

func (h *WeeklyReviewHandler) categoriesCard(c *gin.Context) {
	h.rd.RenderPartial(c, "weekly_categories_card", h.buildCategories(c.Request.Context(), h.parseWeekStart(c)))
}

func (h *WeeklyReviewHandler) chartsCard(c *gin.Context) {
	h.rd.RenderPartial(c, "weekly_charts_card", h.buildCharts(c.Request.Context(), h.parseWeekStart(c)))
}

func (h *WeeklyReviewHandler) suggestionsCard(c *gin.Context) {
	h.rd.RenderPartial(c, "weekly_suggestions_card", h.buildSuggestions(c.Request.Context()))
}

func (h *WeeklyReviewHandler) notesSection(c *gin.Context) {
	ws := h.parseWeekStart(c)
	h.rd.RenderPartial(c, "weekly_notes_section", h.buildNotes(c.Request.Context(), ws))
}

func (h *WeeklyReviewHandler) autosave(c *gin.Context) {
	ctx := c.Request.Context()
	ws := h.parseWeekStart(c)
	section := weeklySection(strings.ToLower(c.Query("section")))
	if !section.valid() {
		c.String(http.StatusBadRequest, "invalid section")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}

	in := service.UpsertWeeklyReviewInput{WeekStart: ws}
	switch section {
	case wSectionWins:
		v := c.PostForm("wins")
		in.Wins = &v
	case wSectionBottlenecks:
		v := c.PostForm("bottlenecks")
		in.Bottlenecks = &v
	case wSectionImprovement:
		v := c.PostForm("improvement_notes")
		in.ImprovementNotes = &v
	case wSectionPriorities:
		v := c.PostForm("next_week_priorities")
		in.NextWeekPriorities = &v
	}

	if _, err := h.reviews.Upsert(ctx, in); err != nil {
		h.log.Warn("weekly.autosave", slog.String("section", string(section)), slog.String("err", err.Error()))
		h.rd.RenderPartial(c, "review_save_status", ReviewSaveStatusVM{
			State: "error", Message: "Could not save notes",
		})
		return
	}
	h.rd.RenderPartial(c, "review_save_status", ReviewSaveStatusVM{
		State: "saved", Message: "Draft saved", Time: h.clock.Now().Format("15:04"),
	})
}

// ---------------------------------------------------------------------------
// Builders
// ---------------------------------------------------------------------------

func (h *WeeklyReviewHandler) buildPage(ctx context.Context, ws time.Time) WeeklyReviewPageVM {
	current := h.currentWeekStart()
	end := ws.AddDate(0, 0, 6)
	prev := ws.AddDate(0, 0, -7)
	next := ws.AddDate(0, 0, 7)

	return WeeklyReviewPageVM{
		WeekStart:   ws,
		WeekEnd:     end,
		WeekLabel:   ws.Format("Jan 2") + " – " + end.Format("Jan 2, 2006"),
		WeekInput:   ws.Format("2006-01-02"),
		PrevPath:    h.weeklyPath(prev),
		NextPath:    h.weeklyPath(next),
		IsCurrent:   ws.Equal(current),
		Stats:       h.buildStats(ctx, ws),
		Streak:      h.buildStreak(ctx, ws),
		Categories:  h.buildCategories(ctx, ws),
		Charts:      h.buildCharts(ctx, ws),
		Suggestions: h.buildSuggestions(ctx),
		Notes:       h.buildNotes(ctx, ws),
		SaveStatus:  ReviewSaveStatusVM{State: "idle", Message: "Notes autosave as you type"},
	}
}

func (h *WeeklyReviewHandler) buildStats(ctx context.Context, ws time.Time) WeeklyStatsVM {
	vm := WeeklyStatsVM{WeekInput: ws.Format("2006-01-02")}
	if h.metrics == nil {
		return vm
	}
	weekly, err := h.metrics.WeeklyFor(ctx, ws)
	if err != nil {
		h.log.Warn("weekly.stats", slog.String("err", err.Error()))
		return vm
	}
	vm.CompletionPct = int(weekly.CompletionRate*100 + 0.5)
	vm.CompletedCount = weekly.Breakdown.Completed
	vm.TotalTasks = weekly.Breakdown.Total()
	vm.MissedCount = weekly.Breakdown.Missed

	carryRate, carryN, err := h.metrics.CarryOverRateFor(ctx, ws)
	if err != nil {
		h.log.Warn("weekly.carry", slog.String("err", err.Error()))
	} else {
		vm.CarryOverPct = int(carryRate*100 + 0.5)
		vm.CarryOverCount = carryN
	}
	prevRate, _, err := h.metrics.CarryOverRateFor(ctx, ws.AddDate(0, 0, -7))
	if err == nil {
		vm.CarryOverDelta = vm.CarryOverPct - int(prevRate*100+0.5)
	}

	if h.sessions != nil {
		from := model.NormalizeDate(ws)
		to := from.AddDate(0, 0, 7)
		mins, err := h.sessions.TotalEffectiveMinutesInRange(ctx, from, to)
		if err != nil {
			h.log.Warn("weekly.focus", slog.String("err", err.Error()))
		} else {
			vm.FocusMinutes = mins
			vm.FocusAvgPerDay = mins / 7
		}
	}
	return vm
}

func (h *WeeklyReviewHandler) buildStreak(ctx context.Context, ws time.Time) WeeklyStreakVM {
	vm := WeeklyStreakVM{WeekInput: ws.Format("2006-01-02")}
	if h.metrics == nil {
		return vm
	}
	s, err := h.metrics.Streak(ctx)
	if err != nil {
		h.log.Warn("weekly.streak", slog.String("err", err.Error()))
		return vm
	}
	vm.Current = s.CurrentStreak
	vm.Longest = s.LongestStreak
	vm.MissedDays = s.MissedDayCount
	return vm
}

func (h *WeeklyReviewHandler) buildCategories(ctx context.Context, ws time.Time) WeeklyCategoriesVM {
	vm := WeeklyCategoriesVM{WeekInput: ws.Format("2006-01-02")}
	if h.metrics == nil {
		return vm
	}
	from := model.NormalizeDate(ws)
	to := from.AddDate(0, 0, 7)
	cats, err := h.metrics.Categories(ctx, from, to)
	if err != nil {
		h.log.Warn("weekly.categories", slog.String("err", err.Error()))
		return vm
	}
	var ranked []model.CategoryStats
	for _, c := range cats {
		if c.Total > 0 {
			ranked = append(ranked, c)
		}
	}
	if len(ranked) == 0 {
		return vm
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].CompletionRate > ranked[j].CompletionRate
	})
	vm.HasData = true
	vm.StrongestName = humanCategory(ranked[0].Category)
	vm.StrongestPct = int(ranked[0].CompletionRate*100 + 0.5)
	weakest := ranked[len(ranked)-1]
	vm.WeakestName = humanCategory(weakest.Category)
	vm.WeakestPct = int(weakest.CompletionRate*100 + 0.5)
	return vm
}

func (h *WeeklyReviewHandler) buildCharts(ctx context.Context, ws time.Time) WeeklyChartsVM {
	vm := WeeklyChartsVM{WeekInput: ws.Format("2006-01-02")}
	if h.metrics == nil {
		return vm
	}
	weekly, err := h.metrics.WeeklyFor(ctx, ws)
	if err != nil {
		h.log.Warn("weekly.charts", slog.String("err", err.Error()))
		return vm
	}
	max := 0
	for _, d := range weekly.DailyCompletions {
		if d.Count > max {
			max = d.Count
		}
		vm.CompletionTotal += d.Count
	}
	for _, d := range weekly.DailyCompletions {
		bucket := 0
		if max > 0 {
			bucket = int(float64(d.Count) / float64(max) * 10)
			if bucket > 10 {
				bucket = 10
			}
		}
		vm.CompletionDays = append(vm.CompletionDays, WeeklyChartDayVM{
			Label:     d.Date.Format("Mon"),
			Count:     d.Count,
			HeightCls: "trend-bar--h" + itoa(bucket),
		})
	}
	vm.HasCompletion = vm.CompletionTotal > 0

	carryThis, _, _ := h.metrics.CarryOverRateFor(ctx, ws)
	carryPrev, _, _ := h.metrics.CarryOverRateFor(ctx, ws.AddDate(0, 0, -7))
	vm.CarryThisPct = int(carryThis*100 + 0.5)
	vm.CarryPrevPct = int(carryPrev*100 + 0.5)
	return vm
}

func (h *WeeklyReviewHandler) buildSuggestions(ctx context.Context) WeeklySuggestionsVM {
	vm := WeeklySuggestionsVM{Items: []WeeklySuggestionItemVM{}}
	if h.suggests == nil {
		return vm
	}
	active, err := h.suggests.ListActive(ctx)
	if err != nil {
		h.log.Warn("weekly.suggestions", slog.String("err", err.Error()))
		return vm
	}
	for _, s := range active {
		vm.Items = append(vm.Items, WeeklySuggestionItemVM{
			Title:    s.Title,
			Message:  s.Message,
			Severity: string(s.Severity),
		})
	}
	return vm
}

func (h *WeeklyReviewHandler) buildNotes(ctx context.Context, ws time.Time) WeeklyNotesVM {
	vm := WeeklyNotesVM{WeekInput: ws.Format("2006-01-02")}
	if h.reviews == nil {
		return vm
	}
	rv, err := h.reviews.GetByWeekStart(ctx, ws)
	if err != nil && !errors.Is(err, model.ErrWeeklyReviewNotFound) {
		h.log.Warn("weekly.notes", slog.String("err", err.Error()))
		return vm
	}
	if rv == nil {
		return vm
	}
	vm.Wins = rv.Wins
	vm.Bottlenecks = rv.Bottlenecks
	vm.ImprovementNotes = rv.ImprovementNotes
	vm.NextWeekPriorities = rv.NextWeekPriorities
	return vm
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *WeeklyReviewHandler) currentWeekStart() time.Time {
	today := model.NormalizeDate(h.clock.Now())
	return today.AddDate(0, 0, -6)
}

func (h *WeeklyReviewHandler) parseWeekStart(c *gin.Context) time.Time {
	raw := strings.TrimSpace(c.Query("week"))
	if raw == "" {
		raw = strings.TrimSpace(c.PostForm("week"))
	}
	if raw == "" {
		return h.currentWeekStart()
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return h.currentWeekStart()
	}
	return model.NormalizeWeekStart(t)
}

func (h *WeeklyReviewHandler) weeklyPath(weekStart time.Time) string {
	if weekStart.Equal(h.currentWeekStart()) {
		return "/reviews/weekly"
	}
	return "/reviews/weekly?week=" + weekStart.Format("2006-01-02")
}
