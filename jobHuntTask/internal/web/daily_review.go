package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// DailyReviewHandler renders the journal-style daily review page and
// exposes HTMX endpoints for autosave, snapshot refresh, and submit.
type DailyReviewHandler struct {
	rd       *Renderer
	reviews  *service.DailyReviewService
	tasks    *service.TaskService
	sessions *service.TaskSessionService
	clock    service.Clock
	log      *slog.Logger
}

func NewDailyReviewHandler(
	rd *Renderer,
	reviews *service.DailyReviewService,
	tasks *service.TaskService,
	sessions *service.TaskSessionService,
	clock service.Clock,
	log *slog.Logger,
) *DailyReviewHandler {
	if clock == nil {
		clock = service.SystemClock
	}
	if log == nil {
		log = slog.Default()
	}
	return &DailyReviewHandler{
		rd: rd, reviews: reviews, tasks: tasks, sessions: sessions,
		clock: clock, log: log,
	}
}

func (h *DailyReviewHandler) Register(r *gin.Engine) {
	r.GET("/reviews/daily", h.page)
	g := r.Group("/reviews/daily")
	g.GET("/snapshot", h.snapshot)
	g.PATCH("/autosave", h.autosave)
	g.POST("/submit", h.submit)
}

// ---------------------------------------------------------------------------
// View models
// ---------------------------------------------------------------------------

type DailyReviewPageVM struct {
	Date         time.Time
	DateLabel    string
	DateInput    string // YYYY-MM-DD
	PrevDate     string
	NextDate     string
	PrevPath     string
	NextPath     string
	SnapshotPath string
	IsToday      bool
	Review       ReviewJournalVM
	Snapshot     ReviewSnapshotVM
	SaveStatus   ReviewSaveStatusVM
}

type ReviewJournalVM struct {
	Reflection        string
	WinsText          string
	BlockersText      string
	DistractionsText  string
	Notes             string
	EnergyLevel       int
	ProductivityScore int
	UpdatedAt         *time.Time
}

type ReviewTaskItemVM struct {
	ID       string
	Title    string
	Category string
	Minutes  int
}

type ReviewSnapshotVM struct {
	Date              time.Time
	DateLabel         string
	ExecutionMinutes  int
	Completed         []ReviewTaskItemVM
	Unfinished        []ReviewTaskItemVM
	Overdue           []ReviewTaskItemVM
	CompletedCount    int
	UnfinishedCount   int
	OverdueCount      int
}

type ReviewSaveStatusVM struct {
	State   string // "idle" | "saved" | "error"
	Message string
	Time    string
}

// reviewSection identifies which journal block an autosave request targets.
type reviewSection string

const (
	sectionReflection   reviewSection = "reflection"
	sectionWins         reviewSection = "wins"
	sectionBlockers     reviewSection = "blockers"
	sectionDistractions reviewSection = "distractions"
	sectionNotes        reviewSection = "notes"
	sectionScores       reviewSection = "scores"
)

func (s reviewSection) valid() bool {
	switch s {
	case sectionReflection, sectionWins, sectionBlockers,
		sectionDistractions, sectionNotes, sectionScores:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *DailyReviewHandler) page(c *gin.Context) {
	date := h.parseDate(c)
	vm := h.buildPage(c.Request.Context(), date)
	h.rd.Render(c, "daily_review", PageData{
		Title:  "Daily Review",
		Active: "daily_review",
		Data:   vm,
	})
}

func (h *DailyReviewHandler) snapshot(c *gin.Context) {
	date := h.parseDate(c)
	vm := h.buildSnapshot(c.Request.Context(), date)
	h.rd.RenderPartial(c, "review_snapshot", vm)
}

func (h *DailyReviewHandler) autosave(c *gin.Context) {
	ctx := c.Request.Context()
	date := h.parseDate(c)
	section := reviewSection(strings.ToLower(c.Query("section")))
	if !section.valid() {
		c.String(http.StatusBadRequest, "invalid section")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}

	in := service.UpsertReviewInput{Date: date}
	switch section {
	case sectionReflection:
		v := c.PostForm("reflection")
		in.Reflection = &v
	case sectionWins:
		lines := linesFromTextarea(c.PostForm("wins"))
		in.Wins = &lines
	case sectionBlockers:
		lines := linesFromTextarea(c.PostForm("blockers"))
		in.Blockers = &lines
	case sectionDistractions:
		lines := linesFromTextarea(c.PostForm("distractions"))
		in.Distractions = &lines
	case sectionNotes:
		v := c.PostForm("notes")
		in.Notes = &v
	case sectionScores:
		energy := atoiOr(c.PostForm("energy_level"), 0)
		prod := atoiOr(c.PostForm("productivity_score"), 0)
		in.EnergyLevel = &energy
		in.ProductivityScore = &prod
	}

	if _, err := h.reviews.Upsert(ctx, in); err != nil {
		h.log.Warn("review.autosave", slog.String("section", string(section)), slog.String("err", err.Error()))
		h.rd.RenderPartial(c, "review_save_status", ReviewSaveStatusVM{
			State: "error", Message: humanReviewError(err),
		})
		return
	}
	h.rd.RenderPartial(c, "review_save_status", ReviewSaveStatusVM{
		State: "saved", Message: "Draft saved", Time: h.clock.Now().Format("15:04"),
	})
}

func (h *DailyReviewHandler) submit(c *gin.Context) {
	ctx := c.Request.Context()
	date := h.parseDate(c)
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}

	energy := atoiOr(c.PostForm("energy_level"), 0)
	prod := atoiOr(c.PostForm("productivity_score"), 0)
	ref := c.PostForm("reflection")
	notes := c.PostForm("notes")
	wins := linesFromTextarea(c.PostForm("wins"))
	blockers := linesFromTextarea(c.PostForm("blockers"))
	distractions := linesFromTextarea(c.PostForm("distractions"))

	_, err := h.reviews.Upsert(ctx, service.UpsertReviewInput{
		Date:              date,
		Reflection:        &ref,
		Wins:              &wins,
		Blockers:          &blockers,
		Distractions:      &distractions,
		Notes:             &notes,
		EnergyLevel:       &energy,
		ProductivityScore: &prod,
	})
	if err != nil {
		setToast(c, "warning", humanReviewError(err))
		h.rd.RenderPartial(c, "review_save_status", ReviewSaveStatusVM{
			State: "error", Message: humanReviewError(err),
		})
		return
	}
	setToast(c, "success", "Review saved")
	h.rd.RenderPartial(c, "review_save_status", ReviewSaveStatusVM{
		State: "saved", Message: "Review saved", Time: h.clock.Now().Format("15:04"),
	})
}

// ---------------------------------------------------------------------------
// Builders
// ---------------------------------------------------------------------------

func (h *DailyReviewHandler) buildPage(ctx context.Context, date time.Time) DailyReviewPageVM {
	date = model.NormalizeDate(date)
	today := model.NormalizeDate(h.clock.Now())
	prev := date.AddDate(0, 0, -1)
	next := date.AddDate(0, 0, 1)

	vm := DailyReviewPageVM{
		Date:         date,
		DateLabel:    date.Format("Monday, Jan 2, 2006"),
		DateInput:    date.Format("2006-01-02"),
		PrevDate:     prev.Format("2006-01-02"),
		NextDate:     next.Format("2006-01-02"),
		PrevPath:     reviewDatePath(prev, today),
		NextPath:     reviewDatePath(next, today),
		SnapshotPath: "/reviews/daily/snapshot?date=" + date.Format("2006-01-02"),
		IsToday:      date.Equal(today),
		Review:       h.buildJournal(ctx, date),
		Snapshot:     h.buildSnapshot(ctx, date),
		SaveStatus:   ReviewSaveStatusVM{State: "idle", Message: "Edits autosave as you type"},
	}
	return vm
}

func (h *DailyReviewHandler) buildJournal(ctx context.Context, date time.Time) ReviewJournalVM {
	vm := ReviewJournalVM{}
	if h.reviews == nil {
		return vm
	}
	rv, err := h.reviews.GetByDate(ctx, date)
	if err != nil && !errors.Is(err, model.ErrReviewNotFound) {
		h.log.Warn("review.journal", slog.String("err", err.Error()))
		return vm
	}
	if rv == nil {
		return vm
	}
	vm.Reflection = rv.Reflection
	vm.WinsText = joinLines(rv.Wins)
	vm.BlockersText = joinLines(rv.Blockers)
	vm.DistractionsText = joinLines(rv.Distractions)
	vm.Notes = rv.Notes
	vm.EnergyLevel = rv.EnergyLevel
	vm.ProductivityScore = rv.ProductivityScore
	t := rv.UpdatedAt
	vm.UpdatedAt = &t
	return vm
}

func (h *DailyReviewHandler) buildSnapshot(ctx context.Context, date time.Time) ReviewSnapshotVM {
	date = model.NormalizeDate(date)
	vm := ReviewSnapshotVM{
		Date:      date,
		DateLabel: date.Format("Mon, Jan 2"),
		Completed: []ReviewTaskItemVM{},
		Unfinished: []ReviewTaskItemVM{},
		Overdue:   []ReviewTaskItemVM{},
	}
	if h.sessions != nil {
		mins, err := h.sessions.TotalEffectiveMinutesForDay(ctx, date)
		if err != nil {
			h.log.Warn("review.snapshot.minutes", slog.String("err", err.Error()))
		} else {
			vm.ExecutionMinutes = mins
		}
	}
	if h.tasks == nil {
		return vm
	}

	from := date
	to := date.Add(24 * time.Hour)

	all, err := h.tasks.List(ctx, repository.TaskFilter{Limit: 500})
	if err != nil {
		h.log.Warn("review.snapshot.tasks", slog.String("err", err.Error()))
		return vm
	}

	for _, t := range all {
		item := ReviewTaskItemVM{
			ID:       t.ID.String(),
			Title:    t.Title,
			Category: humanCategory(t.Category),
			Minutes:  t.ActualMinutes,
		}
		switch {
		case t.Status == model.StatusCompleted && completedOnDay(t, from, to):
			vm.Completed = append(vm.Completed, item)
		case !t.Status.IsTerminal():
			if t.DueDate != nil && t.DueDate.Before(h.clock.Now()) {
				vm.Overdue = append(vm.Overdue, item)
			} else {
				vm.Unfinished = append(vm.Unfinished, item)
			}
		}
	}
	vm.CompletedCount = len(vm.Completed)
	vm.UnfinishedCount = len(vm.Unfinished)
	vm.OverdueCount = len(vm.Overdue)
	return vm
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *DailyReviewHandler) parseDate(c *gin.Context) time.Time {
	raw := strings.TrimSpace(c.Query("date"))
	if raw == "" {
		raw = strings.TrimSpace(c.PostForm("date"))
	}
	if raw == "" {
		return model.NormalizeDate(h.clock.Now())
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return model.NormalizeDate(h.clock.Now())
	}
	return model.NormalizeDate(t)
}

func completedOnDay(t *model.Task, from, to time.Time) bool {
	if t.CompletedAt != nil {
		c := t.CompletedAt.UTC()
		return !c.Before(from) && c.Before(to)
	}
	// Fallback: updated today while completed.
	u := t.UpdatedAt.UTC()
	return t.Status == model.StatusCompleted && !u.Before(from) && u.Before(to)
}

func linesFromTextarea(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func humanReviewError(err error) string {
	switch {
	case errors.Is(err, model.ErrInvalidEnergyLevel):
		return "Energy level must be 0–10."
	case errors.Is(err, model.ErrInvalidProductivity):
		return "Productivity score must be 0–10."
	case errors.Is(err, model.ErrBlockerEmpty), errors.Is(err, model.ErrWinEmpty),
		errors.Is(err, model.ErrDistractionEmpty):
		return "Remove blank lines from list fields."
	}
	return "Could not save review."
}

func reviewDatePath(date time.Time, today time.Time) string {
	if date.Equal(today) {
		return "/reviews/daily"
	}
	return "/reviews/daily?date=" + date.Format("2006-01-02")
}
