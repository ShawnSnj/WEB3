package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/calendar"
	"github.com/shawn/jobhunttask/internal/model"
)

// ---------------------------------------------------------------------------
// View models — job hunt progress dashboard
// ---------------------------------------------------------------------------

type JobHuntDashboardVM struct {
	Summary   JobHuntSummaryVM
	ActiveTab string
}

type JobHuntSummaryVM struct {
	TotalDMs          int
	TotalApplications int
	DMTasks           int
	ApplicationTasks  int
}

type JobHuntDMRowVM struct {
	ID           string
	DateLabel    string
	PersonName   string
	Company      string
	RoleTitle    string
	Platform     string
	ReplyStatus  string
	ReplyLabel   string
	TaskTitle    string
}

type JobHuntAppRowVM struct {
	ID            string
	DateLabel     string
	Company       string
	JobTitle      string
	Status        string
	StatusLabel   string
	FitScore      string
	Source        string
	TaskTitle     string
}

type JobHuntTaskRowVM struct {
	ID             string
	DueDateLabel   string
	Title          string
	Status         string
	StatusLabel    string
	Priority       string
	PriorityLabel  string
	EstimatedLabel string
}

type JobHuntNoteDetailVM struct {
	ID                string
	NoteType          string
	NoteTypeLabel     string
	TaskID            string
	TaskTitle         string
	PersonName        string
	Company           string
	RoleTitle         string
	Platform          string
	ProfileURL        string
	MessageContent    string
	SentAtLabel       string
	ReplyStatus       string
	ReplyLabel        string
	ReplyAtLabel      string
	JobTitle          string
	JobURL            string
	ApplicationStatus string
	AppStatusLabel    string
	AppliedAtLabel    string
	ResumeVersion     string
	FitScore          string
	Source            string
	Notes             string
	CreatedAtLabel    string
	UpdatedAtLabel    string
}

type JobHuntTaskDetailVM struct {
	ID             string
	Title          string
	Status         string
	StatusLabel    string
	Priority       string
	PriorityLabel  string
	DueDateLabel   string
	EstimatedLabel string
	CategoryLabel  string
}

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

func (h *TasksHandler) registerJobHuntRoutes(g *gin.RouterGroup) {
	jh := g.Group("/job-hunt")
	jh.GET("/summary", h.jobHuntSummaryPartial)
	jh.GET("/dms", h.jobHuntDMList)
	jh.GET("/applications", h.jobHuntAppList)
	jh.GET("/dm-tasks", h.jobHuntDMTasks)
	jh.GET("/application-tasks", h.jobHuntAppTasks)
	jh.GET("/notes/:noteId", h.jobHuntNoteDetail)
	jh.GET("/tasks/:taskId", h.jobHuntTaskDetail)
}

func (h *TasksHandler) jobHuntSummaryPartial(c *gin.Context) {
	vm := h.buildJobHuntDashboard(c.Request.Context())
	h.rd.RenderPartial(c, "job_hunt_dashboard", vm)
}

func (h *TasksHandler) jobHuntDMList(c *gin.Context) {
	rows := h.buildDMRows(c.Request.Context())
	h.rd.RenderPartial(c, "job_hunt_dms_table", gin.H{
		"Rows":      rows,
		"Empty":     len(rows) == 0,
		"ActiveTab": "dms",
	})
}

func (h *TasksHandler) jobHuntAppList(c *gin.Context) {
	rows := h.buildAppRows(c.Request.Context())
	h.rd.RenderPartial(c, "job_hunt_apps_table", gin.H{
		"Rows":      rows,
		"Empty":     len(rows) == 0,
		"ActiveTab": "applications",
	})
}

func (h *TasksHandler) jobHuntDMTasks(c *gin.Context) {
	rows := h.buildOutreachTaskRows(c.Request.Context())
	h.rd.RenderPartial(c, "job_hunt_tasks_table", gin.H{
		"Rows":      rows,
		"Empty":     len(rows) == 0,
		"ActiveTab": "dm_tasks",
		"Kind":      "dm",
	})
}

func (h *TasksHandler) jobHuntAppTasks(c *gin.Context) {
	rows := h.buildApplicationTaskRows(c.Request.Context())
	h.rd.RenderPartial(c, "job_hunt_tasks_table", gin.H{
		"Rows":      rows,
		"Empty":     len(rows) == 0,
		"ActiveTab": "application_tasks",
		"Kind":      "application",
	})
}

func (h *TasksHandler) jobHuntNoteDetail(c *gin.Context) {
	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid note id")
		return
	}
	if h.notes == nil {
		c.String(http.StatusServiceUnavailable, "notes unavailable")
		return
	}
	n, err := h.notes.GetWithTask(c.Request.Context(), noteID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	vm := h.toJobHuntNoteDetailVM(n)
	if n.NoteType == model.NoteTypeDM {
		h.rd.RenderPartial(c, "job_hunt_dm_detail_modal", vm)
		return
	}
	if n.NoteType == model.NoteTypeJobApp {
		h.rd.RenderPartial(c, "job_hunt_app_detail_modal", vm)
		return
	}
	h.rd.RenderPartial(c, "job_hunt_generic_note_modal", vm)
}

func (h *TasksHandler) jobHuntTaskDetail(c *gin.Context) {
	taskID, ok := h.parseIDFromParam(c, "taskId")
	if !ok {
		return
	}
	t, err := h.tasks.Get(c.Request.Context(), taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	h.rd.RenderPartial(c, "job_hunt_task_detail_modal", h.toJobHuntTaskDetailVM(t))
}

func (h *TasksHandler) parseIDFromParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// ---------------------------------------------------------------------------
// Builders
// ---------------------------------------------------------------------------

func (h *TasksHandler) buildJobHuntDashboard(ctx context.Context) JobHuntDashboardVM {
	vm := JobHuntDashboardVM{}
	if h.notes == nil {
		return vm
	}
	s, err := h.notes.JobHuntSummary(ctx)
	if err != nil {
		h.log.Warn("job_hunt.summary", slog.String("err", err.Error()))
		return vm
	}
	vm.Summary = JobHuntSummaryVM{
		TotalDMs:          s.TotalDMs,
		TotalApplications: s.TotalApplications,
		DMTasks:           s.DMTasks,
		ApplicationTasks:  s.ApplicationTasks,
	}
	return vm
}

func (h *TasksHandler) buildDMRows(ctx context.Context) []JobHuntDMRowVM {
	if h.notes == nil {
		return nil
	}
	items, err := h.notes.ListDMs(ctx)
	if err != nil {
		h.log.Warn("job_hunt.dms", slog.String("err", err.Error()))
		return nil
	}
	out := make([]JobHuntDMRowVM, 0, len(items))
	for _, n := range items {
		out = append(out, JobHuntDMRowVM{
			ID:          n.ID.String(),
			DateLabel:   formatJobHuntDate(n.SentAt, n.CreatedAt, h.cal),
			PersonName:  dashOr(n.PersonName),
			Company:     dashOr(n.Company),
			RoleTitle:   dashOr(n.RoleTitle),
			Platform:    dashOr(n.Platform),
			ReplyStatus: string(n.ReplyStatus),
			ReplyLabel:  replyStatusLabel(n.ReplyStatus),
			TaskTitle:   n.TaskTitle,
		})
	}
	return out
}

func (h *TasksHandler) buildAppRows(ctx context.Context) []JobHuntAppRowVM {
	if h.notes == nil {
		return nil
	}
	items, err := h.notes.ListApplications(ctx)
	if err != nil {
		h.log.Warn("job_hunt.applications", slog.String("err", err.Error()))
		return nil
	}
	out := make([]JobHuntAppRowVM, 0, len(items))
	for _, n := range items {
		fit := "—"
		if n.FitScore != nil {
			fit = fmt.Sprintf("%d", *n.FitScore)
		}
		out = append(out, JobHuntAppRowVM{
			ID:          n.ID.String(),
			DateLabel:   formatJobHuntDate(n.AppliedAt, n.CreatedAt, h.cal),
			Company:     dashOr(n.Company),
			JobTitle:    dashOr(n.JobTitle),
			Status:      string(n.ApplicationStatus),
			StatusLabel: appStatusLabel(n.ApplicationStatus),
			FitScore:    fit,
			Source:      dashOr(string(n.Source)),
			TaskTitle:   n.TaskTitle,
		})
	}
	return out
}

func (h *TasksHandler) buildOutreachTaskRows(ctx context.Context) []JobHuntTaskRowVM {
	if h.notes == nil {
		return nil
	}
	tasks, err := h.notes.ListOutreachTasks(ctx)
	if err != nil {
		h.log.Warn("job_hunt.dm_tasks", slog.String("err", err.Error()))
		return nil
	}
	return h.tasksToJobHuntRows(tasks)
}

func (h *TasksHandler) buildApplicationTaskRows(ctx context.Context) []JobHuntTaskRowVM {
	if h.notes == nil {
		return nil
	}
	tasks, err := h.notes.ListApplicationTasks(ctx)
	if err != nil {
		h.log.Warn("job_hunt.application_tasks", slog.String("err", err.Error()))
		return nil
	}
	return h.tasksToJobHuntRows(tasks)
}

func (h *TasksHandler) tasksToJobHuntRows(tasks []*model.Task) []JobHuntTaskRowVM {
	out := make([]JobHuntTaskRowVM, 0, len(tasks))
	for _, t := range tasks {
		row := h.toRowVM(context.Background(), t, h.clock.Now())
		out = append(out, JobHuntTaskRowVM{
			ID:             t.ID.String(),
			DueDateLabel:   row.DueDateLabel,
			Title:          t.Title,
			Status:         string(t.Status),
			StatusLabel:    row.StatusLabel,
			Priority:       string(t.Priority),
			PriorityLabel:  row.PriorityLabel,
			EstimatedLabel: row.EstimatedLabel,
		})
	}
	return out
}

func (h *TasksHandler) toJobHuntNoteDetailVM(n *model.TaskNoteWithTask) JobHuntNoteDetailVM {
	fit := "—"
	if n.FitScore != nil {
		fit = fmt.Sprintf("%d", *n.FitScore)
	}
	return JobHuntNoteDetailVM{
		ID:                n.ID.String(),
		NoteType:          string(n.NoteType),
		NoteTypeLabel:     n.NoteType.Label(),
		TaskID:            n.TaskID.String(),
		TaskTitle:         n.TaskTitle,
		PersonName:        dashOr(n.PersonName),
		Company:           dashOr(n.Company),
		RoleTitle:         dashOr(n.RoleTitle),
		Platform:          dashOr(n.Platform),
		ProfileURL:        dashOr(n.ProfileURL),
		MessageContent:    dashOr(n.MessageContent),
		SentAtLabel:       formatJobHuntDateTime(n.SentAt),
		ReplyStatus:       string(n.ReplyStatus),
		ReplyLabel:        replyStatusLabel(n.ReplyStatus),
		ReplyAtLabel:      formatJobHuntDateTime(n.ReplyAt),
		JobTitle:          dashOr(n.JobTitle),
		JobURL:            dashOr(n.JobURL),
		ApplicationStatus: string(n.ApplicationStatus),
		AppStatusLabel:    appStatusLabel(n.ApplicationStatus),
		AppliedAtLabel:    formatJobHuntDateTime(n.AppliedAt),
		ResumeVersion:     dashOr(n.ResumeVersion),
		FitScore:          fit,
		Source:            dashOr(string(n.Source)),
		Notes:             dashOr(n.EffectiveNotes()),
		CreatedAtLabel:    n.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
		UpdatedAtLabel:    n.UpdatedAt.Format("Jan 2, 2006 3:04 PM"),
	}
}

func (h *TasksHandler) toJobHuntTaskDetailVM(t *model.Task) JobHuntTaskDetailVM {
	row := h.toRowVM(context.Background(), t, h.clock.Now())
	return JobHuntTaskDetailVM{
		ID:             t.ID.String(),
		Title:          t.Title,
		Status:         string(t.Status),
		StatusLabel:    row.StatusLabel,
		Priority:       string(t.Priority),
		PriorityLabel:  row.PriorityLabel,
		DueDateLabel:   row.DueDateLabel,
		EstimatedLabel: row.EstimatedLabel,
		CategoryLabel:  row.CategoryLabel,
	}
}

func formatJobHuntDate(primary *time.Time, fallback time.Time, cal *calendar.Calendar) string {
	if primary != nil && !primary.IsZero() {
		return cal.FormatDueDate(*primary)
	}
	return fallback.Format("Jan 2, 2006")
}

func formatJobHuntDateTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006 3:04 PM")
}

func dashOr(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func replyStatusLabel(s model.ReplyStatus) string {
	if s == "" {
		return "—"
	}
	return s.Label()
}

func appStatusLabel(s model.ApplicationStatus) string {
	if s == "" {
		return "—"
	}
	return s.Label()
}
