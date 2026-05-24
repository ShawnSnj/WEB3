package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// AnalyticsHandler renders the analytics dashboard with Chart.js charts
// and HTMX-refreshable fragments.
type AnalyticsHandler struct {
	rd      *Renderer
	metrics *service.MetricsService
	clock   service.Clock
	log     *slog.Logger
}

func NewAnalyticsHandler(
	rd *Renderer,
	metrics *service.MetricsService,
	clock service.Clock,
	log *slog.Logger,
) *AnalyticsHandler {
	if clock == nil {
		clock = service.SystemClock
	}
	if log == nil {
		log = slog.Default()
	}
	return &AnalyticsHandler{rd: rd, metrics: metrics, clock: clock, log: log}
}

func (h *AnalyticsHandler) Register(r *gin.Engine) {
	r.GET("/analytics", h.page)
	g := r.Group("/analytics")
	g.GET("/kpis", h.kpis)
	g.GET("/comparison", h.comparison)
	g.GET("/charts/completion", h.chartCompletion)
	g.GET("/charts/carry", h.chartCarry)
	g.GET("/charts/category", h.chartCategory)
	g.GET("/charts/productivity", h.chartProductivity)
	g.GET("/charts/overdue", h.chartOverdue)
	g.GET("/charts/execution", h.chartExecution)
	g.GET("/charts/streak", h.chartStreak)
	g.GET("/refresh", h.refreshPanels)
}

// ---------------------------------------------------------------------------
// View models
// ---------------------------------------------------------------------------

type AnalyticsPageVM struct {
	Range      service.AnalyticsRange
	RangeLabel string
	KPIs       AnalyticsKPIsVM
	Comparison AnalyticsComparisonVM
	Completion AnalyticsChartVM
	Carry      AnalyticsChartVM
	Category   AnalyticsChartVM
	Productivity AnalyticsChartVM
	Overdue    AnalyticsChartVM
	Execution  AnalyticsChartVM
	Streak     AnalyticsChartVM
}

type AnalyticsKPIsVM struct {
	Range           string
	Completed       int
	CompletedDelta  int
	CompletionPct   int
	OverduePct      int
	CarryOverPct    int
	AvgExecMinutes  int
	CurrentStreak   int
	LongestStreak   int
}

type AnalyticsComparisonVM struct {
	Range              string
	CompletionPct      int
	CompletionPPDelta  int // percentage points, signed
	Completed          int
	CompletedDelta     int
	PeriodLabel        string
	PrevPeriodLabel    string
}

type AnalyticsChartVM struct {
	Range      string
	ChartID    string
	ChartType  string
	Title      string
	Subtitle   string
	ConfigJSON string
	Empty      bool
	EmptyTitle string
	EmptyMsg   string
}

type chartDataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BackgroundColor any       `json:"backgroundColor,omitempty"`
	BorderColor     string    `json:"borderColor,omitempty"`
	BorderWidth     int       `json:"borderWidth,omitempty"`
	Fill            bool      `json:"fill,omitempty"`
	Tension         float64   `json:"tension,omitempty"`
}

type chartPayload struct {
	Labels   []string       `json:"labels"`
	Datasets []chartDataset `json:"datasets"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *AnalyticsHandler) page(c *gin.Context) {
	rng := h.parseRange(c)
	vm := h.buildPage(c.Request.Context(), rng)
	h.rd.Render(c, "analytics", PageData{Title: "Analytics", Active: "analytics", Data: vm})
}

func (h *AnalyticsHandler) kpis(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_kpis", h.buildKPIs(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) comparison(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_comparison", h.buildComparison(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) chartCompletion(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_chart_card", h.buildCompletionChart(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) chartCarry(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_chart_card", h.buildCarryChart(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) chartCategory(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_chart_card", h.buildCategoryChart(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) chartProductivity(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_chart_card", h.buildProductivityChart(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) chartOverdue(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_chart_card", h.buildOverdueChart(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) chartExecution(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_chart_card", h.buildExecutionChart(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) chartStreak(c *gin.Context) {
	h.rd.RenderPartial(c, "analytics_chart_card", h.buildStreakChart(c.Request.Context(), h.parseRange(c)))
}

func (h *AnalyticsHandler) refreshPanels(c *gin.Context) {
	rng := h.parseRange(c)
	c.Header("HX-Push-Url", "/analytics?range="+string(rng))
	vm := h.buildPage(c.Request.Context(), rng)
	h.rd.RenderPartial(c, "analytics_panels", vm)
}

// ---------------------------------------------------------------------------
// Builders
// ---------------------------------------------------------------------------

func (h *AnalyticsHandler) buildPage(ctx context.Context, rng service.AnalyticsRange) AnalyticsPageVM {
	return AnalyticsPageVM{
		Range:        rng,
		RangeLabel:   rng.Label(),
		KPIs:         h.buildKPIs(ctx, rng),
		Comparison:   h.buildComparison(ctx, rng),
		Completion:   h.buildCompletionChart(ctx, rng),
		Carry:        h.buildCarryChart(ctx, rng),
		Category:     h.buildCategoryChart(ctx, rng),
		Productivity: h.buildProductivityChart(ctx, rng),
		Overdue:      h.buildOverdueChart(ctx, rng),
		Execution:    h.buildExecutionChart(ctx, rng),
		Streak:       h.buildStreakChart(ctx, rng),
	}
}

func (h *AnalyticsHandler) buildKPIs(ctx context.Context, rng service.AnalyticsRange) AnalyticsKPIsVM {
	vm := AnalyticsKPIsVM{Range: string(rng)}
	if h.metrics == nil {
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	stats, err := h.metrics.PeriodStats(ctx, from, to)
	if err != nil {
		h.log.Warn("analytics.kpis", slog.String("err", err.Error()))
		return vm
	}
	trend, _ := h.metrics.TrendComparisonFor(ctx, from, to)
	streak, _ := h.metrics.Streak(ctx)

	vm.Completed = stats.Breakdown.Completed
	vm.CompletedDelta = trend.CompletedDelta
	vm.CompletionPct = int(stats.CompletionRate*100 + 0.5)
	vm.OverduePct = int(stats.OverdueRate*100 + 0.5)
	vm.CarryOverPct = int(stats.CarryOverRate*100 + 0.5)
	vm.AvgExecMinutes = int(stats.AvgActualMinutes + 0.5)
	vm.CurrentStreak = streak.CurrentStreak
	vm.LongestStreak = streak.LongestStreak
	return vm
}

func (h *AnalyticsHandler) buildComparison(ctx context.Context, rng service.AnalyticsRange) AnalyticsComparisonVM {
	vm := AnalyticsComparisonVM{Range: string(rng), PeriodLabel: rng.Label()}
	if h.metrics == nil {
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	trend, err := h.metrics.TrendComparisonFor(ctx, from, to)
	if err != nil {
		h.log.Warn("analytics.comparison", slog.String("err", err.Error()))
		return vm
	}
	vm.CompletionPct = int(trend.CompletionRateNow*100 + 0.5)
	dpp := trend.CompletionRateDelta * 100
	if dpp >= 0 {
		vm.CompletionPPDelta = int(dpp + 0.5)
	} else {
		vm.CompletionPPDelta = int(dpp - 0.5)
	}
	vm.Completed = trend.CompletedNow
	vm.CompletedDelta = trend.CompletedDelta
	vm.PrevPeriodLabel = "prior " + rng.Label()
	return vm
}

func (h *AnalyticsHandler) buildCompletionChart(ctx context.Context, rng service.AnalyticsRange) AnalyticsChartVM {
	vm := AnalyticsChartVM{
		Range: string(rng), ChartID: "chart-completion", ChartType: "line",
		Title: "Task completion trend", Subtitle: "Daily completions",
	}
	if h.metrics == nil {
		vm.Empty = true
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	stats, err := h.metrics.PeriodStats(ctx, from, to)
	if err != nil {
		h.log.Warn("analytics.completion", slog.String("err", err.Error()))
		vm.Empty = true
		return vm
	}
	if stats.Breakdown.Completed == 0 {
		vm.Empty = true
		vm.EmptyTitle = "No completions yet"
		vm.EmptyMsg = "Complete tasks to see the trend."
		return vm
	}
	labels := make([]string, len(stats.DailyCompletions))
	data := make([]float64, len(stats.DailyCompletions))
	for i, d := range stats.DailyCompletions {
		labels[i] = d.Date.Format("Jan 2")
		data[i] = float64(d.Count)
	}
	vm.ConfigJSON = mustChartJSON(chartPayload{
		Labels: labels,
		Datasets: []chartDataset{{
			Label: "Completed", Data: data, BorderColor: "#6ea8fe", Fill: true, Tension: 0.3,
		}},
	})
	return vm
}

func (h *AnalyticsHandler) buildCarryChart(ctx context.Context, rng service.AnalyticsRange) AnalyticsChartVM {
	vm := AnalyticsChartVM{
		Range: string(rng), ChartID: "chart-carry", ChartType: "line",
		Title: "Carry-over trend", Subtitle: "Weekly carry-over rate %",
	}
	if h.metrics == nil {
		vm.Empty = true
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	buckets, err := h.metrics.WeeklyBuckets(ctx, from, to)
	if err != nil || len(buckets) == 0 {
		vm.Empty = true
		vm.EmptyTitle = "No carry-over data"
		vm.EmptyMsg = "Carry-over rates appear once tasks roll forward."
		return vm
	}
	labels := make([]string, len(buckets))
	data := make([]float64, len(buckets))
	for i, b := range buckets {
		labels[i] = b.Label
		data[i] = b.CarryOverRate * 100
	}
	vm.ConfigJSON = mustChartJSON(chartPayload{
		Labels: labels,
		Datasets: []chartDataset{{
			Label: "Carry-over %", Data: data, BorderColor: "#fbbf24", Fill: false, Tension: 0.2,
		}},
	})
	return vm
}

func (h *AnalyticsHandler) buildCategoryChart(ctx context.Context, rng service.AnalyticsRange) AnalyticsChartVM {
	vm := AnalyticsChartVM{
		Range: string(rng), ChartID: "chart-category", ChartType: "bar",
		Title: "Category ROI", Subtitle: "Completion rate by category",
	}
	if h.metrics == nil {
		vm.Empty = true
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	cats, err := h.metrics.Categories(ctx, from, to)
	if err != nil || len(cats) == 0 {
		vm.Empty = true
		vm.EmptyTitle = "Not enough data"
		vm.EmptyMsg = "Tag tasks with categories to compare ROI."
		return vm
	}
	var filtered []model.CategoryStats
	for _, c := range cats {
		if c.Total > 0 {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		vm.Empty = true
		vm.EmptyTitle = "Not enough data"
		vm.EmptyMsg = "Tag tasks with categories to compare ROI."
		return vm
	}
	labels := make([]string, len(filtered))
	data := make([]float64, len(filtered))
	for i, c := range filtered {
		labels[i] = humanCategory(c.Category)
		data[i] = c.CompletionRate * 100
	}
	vm.ConfigJSON = mustChartJSON(chartPayload{
		Labels: labels,
		Datasets: []chartDataset{{
			Label: "Completion %", Data: data, BackgroundColor: "#6ea8fe",
		}},
	})
	return vm
}

func (h *AnalyticsHandler) buildProductivityChart(ctx context.Context, rng service.AnalyticsRange) AnalyticsChartVM {
	vm := AnalyticsChartVM{
		Range: string(rng), ChartID: "chart-productivity", ChartType: "bar",
		Title: "Weekly productivity", Subtitle: "Tasks completed per week",
	}
	if h.metrics == nil {
		vm.Empty = true
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	buckets, err := h.metrics.WeeklyBuckets(ctx, from, to)
	if err != nil || len(buckets) == 0 {
		vm.Empty = true
		return vm
	}
	labels := make([]string, len(buckets))
	data := make([]float64, len(buckets))
	total := 0
	for i, b := range buckets {
		labels[i] = b.Label
		data[i] = float64(b.Completed)
		total += b.Completed
	}
	if total == 0 {
		vm.Empty = true
		vm.EmptyTitle = "No productivity data"
		vm.EmptyMsg = "Complete tasks to see weekly output."
		return vm
	}
	vm.ConfigJSON = mustChartJSON(chartPayload{
		Labels: labels,
		Datasets: []chartDataset{{
			Label: "Completed", Data: data, BackgroundColor: "#4ade80",
		}},
	})
	return vm
}

func (h *AnalyticsHandler) buildOverdueChart(ctx context.Context, rng service.AnalyticsRange) AnalyticsChartVM {
	vm := AnalyticsChartVM{
		Range: string(rng), ChartID: "chart-overdue", ChartType: "line",
		Title: "Overdue rate", Subtitle: "Weekly missed-task rate %",
	}
	if h.metrics == nil {
		vm.Empty = true
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	buckets, err := h.metrics.WeeklyBuckets(ctx, from, to)
	if err != nil || len(buckets) == 0 {
		vm.Empty = true
		return vm
	}
	labels := make([]string, len(buckets))
	data := make([]float64, len(buckets))
	for i, b := range buckets {
		labels[i] = b.Label
		data[i] = b.OverdueRate * 100
	}
	vm.ConfigJSON = mustChartJSON(chartPayload{
		Labels: labels,
		Datasets: []chartDataset{{
			Label: "Overdue %", Data: data, BorderColor: "#f87171", Fill: false, Tension: 0.2,
		}},
	})
	return vm
}

func (h *AnalyticsHandler) buildExecutionChart(ctx context.Context, rng service.AnalyticsRange) AnalyticsChartVM {
	vm := AnalyticsChartVM{
		Range: string(rng), ChartID: "chart-execution", ChartType: "line",
		Title: "Average execution time", Subtitle: "Weekly avg actual minutes",
	}
	if h.metrics == nil {
		vm.Empty = true
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	buckets, err := h.metrics.WeeklyBuckets(ctx, from, to)
	if err != nil || len(buckets) == 0 {
		vm.Empty = true
		return vm
	}
	labels := make([]string, len(buckets))
	data := make([]float64, len(buckets))
	hasData := false
	for i, b := range buckets {
		labels[i] = b.Label
		data[i] = b.AvgMinutes
		if b.AvgMinutes > 0 {
			hasData = true
		}
	}
	if !hasData {
		vm.Empty = true
		vm.EmptyTitle = "No execution data"
		vm.EmptyMsg = "Log actual minutes on completed tasks."
		return vm
	}
	vm.ConfigJSON = mustChartJSON(chartPayload{
		Labels: labels,
		Datasets: []chartDataset{{
			Label: "Avg minutes", Data: data, BorderColor: "#a78bfa", Fill: true, Tension: 0.3,
		}},
	})
	return vm
}

func (h *AnalyticsHandler) buildStreakChart(ctx context.Context, rng service.AnalyticsRange) AnalyticsChartVM {
	vm := AnalyticsChartVM{
		Range: string(rng), ChartID: "chart-streak", ChartType: "bar",
		Title: "Streak history", Subtitle: "Daily completion activity",
	}
	if h.metrics == nil {
		vm.Empty = true
		return vm
	}
	from, to := h.metrics.RangeWindow(rng)
	days, err := h.metrics.StreakHistory(ctx, from, to)
	if err != nil || len(days) == 0 {
		vm.Empty = true
		vm.EmptyTitle = "No streak data"
		vm.EmptyMsg = "Your activity heatmap will appear here."
		return vm
	}
	labels := make([]string, len(days))
	data := make([]float64, len(days))
	colors := make([]string, len(days))
	hasActivity := false
	for i, d := range days {
		labels[i] = d.Date.Format("Jan 2")
		if d.Count > 0 {
			data[i] = 1
			colors[i] = "#4ade80"
			hasActivity = true
		} else {
			data[i] = 0
			colors[i] = "rgba(138,146,156,0.25)"
		}
	}
	if !hasActivity {
		vm.Empty = true
		vm.EmptyTitle = "No streak data yet"
		vm.EmptyMsg = "Complete at least one task to start tracking."
		return vm
	}
	vm.ConfigJSON = mustChartJSON(chartPayload{
		Labels: labels,
		Datasets: []chartDataset{{
			Label: "Active day", Data: data, BackgroundColor: colors,
		}},
	})
	return vm
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *AnalyticsHandler) parseRange(c *gin.Context) service.AnalyticsRange {
	return service.ParseAnalyticsRange(strings.TrimSpace(c.Query("range")))
}

func mustChartJSON(p chartPayload) string {
	b, err := json.Marshal(p)
	if err != nil {
		return `{"labels":[],"datasets":[]}`
	}
	return string(b)
}
