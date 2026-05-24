package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// TasksHandler owns every endpoint under /tasks. It depends only on
// TaskService — all HTMX glue, query parsing, and view-model projection
// lives here.
type TasksHandler struct {
	rd    *Renderer
	tasks *service.TaskService
	clock service.Clock
	log   *slog.Logger
}

func NewTasksHandler(rd *Renderer, tasks *service.TaskService, clock service.Clock, log *slog.Logger) *TasksHandler {
	if clock == nil {
		clock = service.SystemClock
	}
	if log == nil {
		log = slog.Default()
	}
	return &TasksHandler{rd: rd, tasks: tasks, clock: clock, log: log}
}

// Register installs all routes. The full page lives at /tasks; everything
// under /tasks/* either returns HTML fragments (HTMX) or 204/4xx.
func (h *TasksHandler) Register(r *gin.Engine) {
	r.GET("/tasks", h.page)
	g := r.Group("/tasks")
	g.GET("/list", h.list)
	g.GET("/form", h.formNew)
	g.GET("/:id/form", h.formEdit)
	g.GET("/:id/row", h.row)
	g.POST("", h.create)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.POST("/:id/complete", h.markComplete)
	g.POST("/:id/in_progress", h.markInProgress)
	g.POST("/:id/missed", h.markMissed)
	g.POST("/:id/carry_over", h.carryOver)
	g.POST("/bulk/complete", h.bulkComplete)
	g.POST("/bulk/delete", h.bulkDelete)
}

// ---------------------------------------------------------------------------
// Query model — what the URL "owns"
// ---------------------------------------------------------------------------

// View is one of the canonical filter presets shown as tabs.
type tasksView string

const (
	viewToday      tasksView = "today"
	viewOverdue    tasksView = "overdue"
	viewCompleted  tasksView = "completed"
	viewCarried    tasksView = "carried_over"
	viewAll        tasksView = "all"
)

func (v tasksView) Valid() bool {
	switch v {
	case viewToday, viewOverdue, viewCompleted, viewCarried, viewAll:
		return true
	}
	return false
}

// tasksQuery is the parsed URL state that drives both the filter and the
// "preserve params on the next click" behaviour of every link.
type tasksQuery struct {
	View       tasksView
	Status     model.Status   // empty == any
	Priority   model.Priority // empty == any
	Category   model.Category // empty == any
	Sort       string         // "due_date" | "priority" | "created_at"
	SortDir    string         // "asc" | "desc"
	Search     string         // case-insensitive substring on title (client-side filter)
}

func parseQuery(c *gin.Context) tasksQuery {
	q := tasksQuery{
		View:    tasksView(strings.ToLower(c.Query("view"))),
		Status:  model.Status(strings.ToLower(c.Query("status"))),
		Priority: model.Priority(strings.ToLower(c.Query("priority"))),
		Category: model.Category(strings.ToLower(c.Query("category"))),
		Sort:    strings.ToLower(c.Query("sort")),
		SortDir: strings.ToLower(c.Query("dir")),
		Search:  strings.TrimSpace(c.Query("q")),
	}
	if !q.View.Valid() {
		q.View = viewToday
	}
	if q.Status != "" && !q.Status.IsValid() {
		q.Status = ""
	}
	if q.Priority != "" && !q.Priority.IsValid() {
		q.Priority = ""
	}
	if q.Category != "" && !q.Category.IsValid() {
		q.Category = ""
	}
	switch q.Sort {
	case "due_date", "priority", "created_at":
	default:
		q.Sort = "due_date"
	}
	if q.SortDir != "asc" && q.SortDir != "desc" {
		q.SortDir = "asc"
	}
	return q
}

// asValues serialises the query back to URL form. Used by templates to
// preserve filters when changing one parameter.
func (q tasksQuery) asValues() url.Values {
	v := url.Values{}
	if q.View != "" {
		v.Set("view", string(q.View))
	}
	if q.Status != "" {
		v.Set("status", string(q.Status))
	}
	if q.Priority != "" {
		v.Set("priority", string(q.Priority))
	}
	if q.Category != "" {
		v.Set("category", string(q.Category))
	}
	if q.Sort != "" {
		v.Set("sort", q.Sort)
	}
	if q.SortDir != "" {
		v.Set("dir", q.SortDir)
	}
	if q.Search != "" {
		v.Set("q", q.Search)
	}
	return v
}

func (q tasksQuery) WithSort(field string) tasksQuery {
	out := q
	if out.Sort == field {
		if out.SortDir == "asc" {
			out.SortDir = "desc"
		} else {
			out.SortDir = "asc"
		}
	} else {
		out.Sort = field
		out.SortDir = "asc"
	}
	return out
}

func (q tasksQuery) WithView(v tasksView) tasksQuery   { out := q; out.View = v; return out }
func (q tasksQuery) ListPath() string                  { return "/tasks/list?" + q.asValues().Encode() }
func (q tasksQuery) PagePath() string                  { return "/tasks?" + q.asValues().Encode() }
func (q tasksQuery) SortIcon(field string) string {
	if q.Sort != field {
		return "" // no glyph until clicked
	}
	if q.SortDir == "asc" {
		return "↑"
	}
	return "↓"
}

// toFilter projects a query into a repository TaskFilter. The view drives
// the date / status / carried_over predicates; explicit filters override
// what the view sets where it makes sense.
func (q tasksQuery) toFilter(now time.Time) repository.TaskFilter {
	f := repository.TaskFilter{Limit: 200}

	if q.Status != "" {
		f.Statuses = []model.Status{q.Status}
	}
	if q.Priority != "" {
		f.Priorities = []model.Priority{q.Priority}
	}
	if q.Category != "" {
		f.Categories = []model.Category{q.Category}
	}

	switch q.View {
	case viewToday:
		// "Today" = anything with no due_date OR due_date <= end of today,
		// and status != completed/missed (unless user filtered explicitly).
		end := startOfDay(now).Add(24 * time.Hour)
		f.DueBefore = &end
		if len(f.Statuses) == 0 {
			f.Statuses = []model.Status{model.StatusPending, model.StatusInProgress}
		}
	case viewOverdue:
		f.OnlyOverdue = true
		if len(f.Statuses) == 0 {
			f.Statuses = []model.Status{model.StatusPending, model.StatusInProgress}
		}
	case viewCompleted:
		if len(f.Statuses) == 0 {
			f.Statuses = []model.Status{model.StatusCompleted}
		}
	case viewCarried:
		truthy := true
		f.CarriedOver = &truthy
	}

	switch q.Sort {
	case "priority":
		f.OrderBy = "priority"
	case "due_date":
		f.OrderBy = "due_date"
	default:
		f.OrderBy = "created_at"
	}
	return f
}

// ---------------------------------------------------------------------------
// View models
// ---------------------------------------------------------------------------

type TasksPageVM struct {
	Query      tasksQuery
	List       TasksListVM
	Counts     TasksCountsVM
	Categories []model.Category
	Priorities []model.Priority
	Statuses   []model.Status
}

type TasksListVM struct {
	Query tasksQuery
	Tasks []TaskRowVM
	Empty bool
}

type TasksCountsVM struct {
	Today       int
	Overdue     int
	Completed   int
	CarriedOver int
}

type TaskRowVM struct {
	ID               string
	Title            string
	Description      string
	Status           string
	StatusLabel      string
	Priority         string
	PriorityLabel    string
	Category         string
	CategoryLabel    string
	EstimatedMinutes int
	ActualMinutes    int
	DueDate          *time.Time
	DueDateLabel     string
	DueDateInput     string // YYYY-MM-DD for <input type="date">
	IsOverdue        bool
	CarryOverCount   int
	CompletedAt      *time.Time
	CreatedAt        time.Time
	CanComplete      bool
	CanInProgress    bool
	CanMiss          bool
	CanCarry         bool
}

type TaskFormVM struct {
	Mode       string // "create" | "edit"
	Action     string
	Method     string
	ID         string
	Title      string
	Description string
	Priority   string
	Category   string
	Estimated  int
	DueDate    string // YYYY-MM-DD or ""
	Categories []model.Category
	Priorities []model.Priority
	Error      string
}

// ---------------------------------------------------------------------------
// Handlers — page / list
// ---------------------------------------------------------------------------

func (h *TasksHandler) page(c *gin.Context) {
	q := parseQuery(c)
	vm := h.buildPage(c.Request.Context(), q)
	h.rd.Render(c, "tasks", PageData{Title: "Tasks", Active: "tasks", Data: vm})
}

func (h *TasksHandler) list(c *gin.Context) {
	q := parseQuery(c)
	vm := h.buildList(c.Request.Context(), q)
	h.rd.RenderPartial(c, "tasks_list", vm)
}

func (h *TasksHandler) buildPage(ctx context.Context, q tasksQuery) TasksPageVM {
	return TasksPageVM{
		Query:      q,
		List:       h.buildList(ctx, q),
		Counts:     h.buildCounts(ctx),
		Categories: model.AllCategories(),
		Priorities: model.AllPriorities(),
		Statuses: []model.Status{
			model.StatusPending, model.StatusInProgress,
			model.StatusCompleted, model.StatusMissed,
		},
	}
}

func (h *TasksHandler) buildList(ctx context.Context, q tasksQuery) TasksListVM {
	vm := TasksListVM{Query: q, Tasks: []TaskRowVM{}}
	if h.tasks == nil {
		return vm
	}
	now := h.clock.Now()
	tasks, err := h.tasks.List(ctx, q.toFilter(now))
	if err != nil {
		h.log.Warn("tasks.list", slog.String("err", err.Error()))
		return vm
	}
	// Client-side search trim — keeps the URL semantics simple without
	// adding a search predicate to the repo just for this UI.
	needle := strings.ToLower(q.Search)
	for _, t := range tasks {
		if needle != "" && !strings.Contains(strings.ToLower(t.Title), needle) {
			continue
		}
		vm.Tasks = append(vm.Tasks, toRowVM(t, now))
	}
	// Apply sort direction (the repo already ordered ASC; flip if needed).
	if q.SortDir == "desc" {
		sort.SliceStable(vm.Tasks, func(i, j int) bool { return false }) // no-op stable
		reverse(vm.Tasks)
	}
	vm.Empty = len(vm.Tasks) == 0
	return vm
}

func (h *TasksHandler) buildCounts(ctx context.Context) TasksCountsVM {
	out := TasksCountsVM{}
	if h.tasks == nil {
		return out
	}
	now := h.clock.Now()
	for _, view := range []tasksView{viewToday, viewOverdue, viewCompleted, viewCarried} {
		q := tasksQuery{View: view}
		tasks, err := h.tasks.List(ctx, q.toFilter(now))
		if err != nil {
			h.log.Warn("tasks.counts", slog.String("view", string(view)), slog.String("err", err.Error()))
			continue
		}
		switch view {
		case viewToday:
			out.Today = len(tasks)
		case viewOverdue:
			out.Overdue = len(tasks)
		case viewCompleted:
			out.Completed = len(tasks)
		case viewCarried:
			out.CarriedOver = len(tasks)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Handlers — form / create / update / delete / row
// ---------------------------------------------------------------------------

func (h *TasksHandler) formNew(c *gin.Context) {
	vm := newFormVM("create", "/tasks", "post", nil)
	h.rd.RenderPartial(c, "task_form", vm)
}

func (h *TasksHandler) formEdit(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	t, err := h.tasks.Get(c.Request.Context(), id)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	vm := newFormVM("edit", "/tasks/"+id.String(), "patch", t)
	h.rd.RenderPartial(c, "task_form", vm)
}

func (h *TasksHandler) row(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	t, err := h.tasks.Get(c.Request.Context(), id)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	h.rd.RenderPartial(c, "task_row", toRowVM(t, h.clock.Now()))
}

func (h *TasksHandler) create(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}
	in := service.CreateTaskInput{
		Title:            c.PostForm("title"),
		Description:      c.PostForm("description"),
		Priority:         model.Priority(c.PostForm("priority")),
		Category:         model.Category(c.PostForm("category")),
		EstimatedMinutes: atoiOr(c.PostForm("estimated_minutes"), 0),
	}
	if d, ok := parseDateInput(c.PostForm("due_date")); ok {
		in.DueDate = &d
	}
	t, err := h.tasks.Create(c.Request.Context(), in)
	if err != nil {
		h.renderFormError(c, "create", "/tasks", "post", nil, err)
		return
	}
	setToast(c, "success", "Task created")
	h.triggerTasksChanged(c)
	c.Status(http.StatusCreated)
	h.rd.RenderPartial(c, "task_row", toRowVM(t, h.clock.Now()))
}

func (h *TasksHandler) update(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}
	in := service.UpdateTaskInput{}
	if v := c.PostForm("title"); v != "" {
		in.Title = &v
	}
	if v, ok := c.GetPostForm("description"); ok {
		in.Description = &v
	}
	if v := c.PostForm("priority"); v != "" {
		p := model.Priority(v)
		in.Priority = &p
	}
	if v := c.PostForm("category"); v != "" {
		ct := model.Category(v)
		in.Category = &ct
	}
	if v := c.PostForm("estimated_minutes"); v != "" {
		n := atoiOr(v, 0)
		in.EstimatedMinutes = &n
	}
	if v, ok := c.GetPostForm("due_date"); ok {
		if v == "" {
			in.ClearDueDate = true
		} else if d, ok := parseDateInput(v); ok {
			in.DueDate = &d
		}
	}
	t, err := h.tasks.Update(c.Request.Context(), id, in)
	if err != nil {
		// On invalid input, re-render the form with the error so the
		// inline edit row stays editable.
		if cur, ferr := h.tasks.Get(c.Request.Context(), id); ferr == nil {
			h.renderFormError(c, "edit", "/tasks/"+id.String(), "patch", cur, err)
			return
		}
		h.notFoundOrErr(c, err)
		return
	}
	setToast(c, "success", "Task updated")
	h.renderTaskSwap(c, t)
}

func (h *TasksHandler) delete(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	if err := h.tasks.Delete(c.Request.Context(), id); err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	setToast(c, "info", "Task deleted")
	h.triggerTasksChanged(c)
	h.renderTaskDelete(c, id)
}

// ---------------------------------------------------------------------------
// State transitions + carry-over
// ---------------------------------------------------------------------------

func (h *TasksHandler) markComplete(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	actual := atoiOr(c.PostForm("actual_minutes"), 0)
	t, err := h.tasks.MarkCompleted(c.Request.Context(), id, actual)
	if err != nil {
		h.transitionErr(c, err)
		return
	}
	setToast(c, "success", "Marked completed")
	h.triggerTasksChanged(c)
	h.renderTaskSwap(c, t)
}

func (h *TasksHandler) markInProgress(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	t, err := h.tasks.MarkInProgress(c.Request.Context(), id)
	if err != nil {
		h.transitionErr(c, err)
		return
	}
	setToast(c, "info", "Marked in progress")
	h.renderTaskSwap(c, t)
}

func (h *TasksHandler) markMissed(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	t, err := h.tasks.MarkMissed(c.Request.Context(), id)
	if err != nil {
		h.transitionErr(c, err)
		return
	}
	setToast(c, "warning", "Marked missed")
	h.triggerTasksChanged(c)
	h.renderTaskSwap(c, t)
}

func (h *TasksHandler) carryOver(c *gin.Context) {
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	if _, err := h.tasks.CarryOverTask(c.Request.Context(), id); err != nil {
		h.transitionErr(c, err)
		return
	}
	// Two tasks changed (source + new). Just refresh the list.
	setToast(c, "success", "Task carried over to tomorrow")
	h.triggerTasksChanged(c)
	c.Status(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Bulk
// ---------------------------------------------------------------------------

func (h *TasksHandler) bulkComplete(c *gin.Context) {
	ids := h.parseBulkIDs(c)
	if len(ids) == 0 {
		setToast(c, "warning", "Select at least one task")
		c.Status(http.StatusOK)
		return
	}
	done := 0
	for _, id := range ids {
		if _, err := h.tasks.MarkCompleted(c.Request.Context(), id, 0); err == nil {
			done++
		}
	}
	setToast(c, "success", "Completed "+strconv.Itoa(done)+" of "+strconv.Itoa(len(ids)))
	h.triggerTasksChanged(c)
	c.Status(http.StatusOK)
}

func (h *TasksHandler) bulkDelete(c *gin.Context) {
	ids := h.parseBulkIDs(c)
	if len(ids) == 0 {
		setToast(c, "warning", "Select at least one task")
		c.Status(http.StatusOK)
		return
	}
	done := 0
	for _, id := range ids {
		if err := h.tasks.Delete(c.Request.Context(), id); err == nil {
			done++
		}
	}
	setToast(c, "info", "Deleted "+strconv.Itoa(done)+" of "+strconv.Itoa(len(ids)))
	h.triggerTasksChanged(c)
	c.Status(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *TasksHandler) parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func (h *TasksHandler) parseBulkIDs(c *gin.Context) []uuid.UUID {
	if err := c.Request.ParseForm(); err != nil {
		return nil
	}
	raw := c.Request.PostForm["ids"]
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		if id, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func (h *TasksHandler) notFoundOrErr(c *gin.Context, err error) {
	if errors.Is(err, model.ErrTaskNotFound) {
		if c.GetHeader("HX-Request") == "true" {
			h.rd.renderHTMXError(c, http.StatusNotFound, "Task not found.")
			return
		}
		c.String(http.StatusNotFound, "task not found")
		return
	}
	h.log.Warn("tasks", slog.String("err", err.Error()))
	if c.GetHeader("HX-Request") == "true" {
		h.rd.renderHTMXError(c, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}
	c.String(http.StatusInternalServerError, "internal error")
}

func (h *TasksHandler) renderTaskSwap(c *gin.Context, t *model.Task) {
	h.rd.RenderPartial(c, "task_swap_bundle", toRowVM(t, h.clock.Now()))
}

func (h *TasksHandler) renderTaskDelete(c *gin.Context, id uuid.UUID) {
	h.rd.RenderPartial(c, "task_delete_bundle", map[string]string{"ID": id.String()})
}

func (h *TasksHandler) transitionErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrTaskNotFound):
		c.String(http.StatusNotFound, "task not found")
	case errors.Is(err, model.ErrInvalidTransition):
		setToast(c, "warning", "Invalid status change")
		c.Status(http.StatusConflict)
	case errors.Is(err, model.ErrTaskNotEligibleCarry):
		setToast(c, "warning", "Task can't be carried over")
		c.Status(http.StatusConflict)
	default:
		h.log.Warn("tasks.transition", slog.String("err", err.Error()))
		c.String(http.StatusInternalServerError, "internal error")
	}
}

func (h *TasksHandler) renderFormError(c *gin.Context, mode, action, method string, t *model.Task, err error) {
	vm := newFormVM(mode, action, method, t)
	vm.Error = humanError(err)
	c.Status(http.StatusUnprocessableEntity)
	h.rd.RenderPartial(c, "task_form", vm)
}

func (h *TasksHandler) triggerTasksChanged(c *gin.Context) {
	// Compose with any toast trigger already set.
	existing := c.Writer.Header().Get("HX-Trigger")
	if existing == "" {
		c.Header("HX-Trigger", `{"tasks-changed":{}}`)
		return
	}
	// existing is JSON; merge by string-splice. Keeps deps zero.
	if strings.Contains(existing, `"tasks-changed"`) {
		return
	}
	if strings.HasSuffix(existing, "}") {
		c.Header("HX-Trigger", existing[:len(existing)-1]+`,"tasks-changed":{}}`)
	}
}

func humanError(err error) string {
	switch {
	case errors.Is(err, model.ErrTitleRequired):
		return "Title is required."
	case errors.Is(err, model.ErrInvalidPriority):
		return "Invalid priority."
	case errors.Is(err, model.ErrInvalidCategory):
		return "Invalid category."
	case errors.Is(err, model.ErrEstimatedNegative):
		return "Estimated minutes must be ≥ 0."
	case errors.Is(err, model.ErrActualNegative):
		return "Actual minutes must be ≥ 0."
	}
	return "Could not save the task."
}

// ---------------------------------------------------------------------------
// Projection helpers
// ---------------------------------------------------------------------------

func toRowVM(t *model.Task, now time.Time) TaskRowVM {
	v := TaskRowVM{
		ID:               t.ID.String(),
		Title:            t.Title,
		Description:      t.Description,
		Status:           string(t.Status),
		StatusLabel:      humanStatus(t.Status),
		Priority:         string(t.Priority),
		PriorityLabel:    humanPriority(t.Priority),
		Category:         string(t.Category),
		CategoryLabel:    humanCategory(t.Category),
		EstimatedMinutes: t.EstimatedMinutes,
		ActualMinutes:    t.ActualMinutes,
		DueDate:          t.DueDate,
		CarryOverCount:   t.CarryOverCount,
		CompletedAt:      t.CompletedAt,
		CreatedAt:        t.CreatedAt,
	}
	if t.DueDate != nil {
		v.DueDateLabel = relativeDue(*t.DueDate, now)
		v.DueDateInput = t.DueDate.Format("2006-01-02")
		v.IsOverdue = t.DueDate.Before(now) && !t.Status.IsTerminal()
	} else {
		v.DueDateLabel = "—"
	}
	v.CanComplete = t.Status.CanTransitionTo(model.StatusCompleted)
	v.CanInProgress = t.Status.CanTransitionTo(model.StatusInProgress)
	v.CanMiss = t.Status.CanTransitionTo(model.StatusMissed)
	v.CanCarry = !t.Status.IsTerminal()
	return v
}

func newFormVM(mode, action, method string, t *model.Task) TaskFormVM {
	vm := TaskFormVM{
		Mode: mode, Action: action, Method: method,
		Categories: model.AllCategories(),
		Priorities: model.AllPriorities(),
		Priority:   string(model.PriorityMedium),
		Category:   string(model.CategoryMisc),
	}
	if t != nil {
		vm.ID = t.ID.String()
		vm.Title = t.Title
		vm.Description = t.Description
		vm.Priority = string(t.Priority)
		vm.Category = string(t.Category)
		vm.Estimated = t.EstimatedMinutes
		if t.DueDate != nil {
			vm.DueDate = t.DueDate.Format("2006-01-02")
		}
	}
	return vm
}

func humanStatus(s model.Status) string {
	switch s {
	case model.StatusPending:
		return "Pending"
	case model.StatusInProgress:
		return "In progress"
	case model.StatusCompleted:
		return "Completed"
	case model.StatusMissed:
		return "Missed"
	}
	return string(s)
}

func humanPriority(p model.Priority) string {
	switch p {
	case model.PriorityLow:
		return "Low"
	case model.PriorityMedium:
		return "Medium"
	case model.PriorityHigh:
		return "High"
	case model.PriorityUrgent:
		return "Urgent"
	}
	return string(p)
}

func humanCategory(c model.Category) string {
	switch c {
	case model.CategoryJobApply:
		return "Job apply"
	case model.CategoryRecruiterOutreach:
		return "Recruiter outreach"
	case model.CategoryGithub:
		return "GitHub"
	case model.CategoryTwitter:
		return "Twitter"
	case model.CategoryNetworking:
		return "Networking"
	case model.CategoryLearning:
		return "Learning"
	case model.CategoryInterview:
		return "Interview"
	case model.CategoryMisc:
		return "Misc"
	}
	return string(c)
}

func relativeDue(due, now time.Time) string {
	d := due.Sub(now)
	abs := d
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs < 24*time.Hour:
		if d < 0 {
			return "overdue"
		}
		return "today"
	case d < 0:
		return "due " + due.Format("Jan 2")
	}
	return "due " + due.Format("Jan 2")
}

func parseDateInput(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func atoiOr(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func reverse[T any](xs []T) {
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
}
