package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// View models — task notes
// ---------------------------------------------------------------------------

type TaskNotesModalVM struct {
	TaskID    string
	TaskTitle string
	Notes     []TaskNoteRowVM
	Selected  *TaskNoteDetailVM
	Empty     bool
}

type TaskNotesTableVM struct {
	TaskID     string
	TaskTitle  string
	Notes      []TaskNoteRowVM
	SelectedID string
	Empty      bool
}

type TaskNoteRowVM struct {
	ID             string
	TaskID         string
	TaskTitle      string
	NoteType       string
	NoteTypeLabel  string
	IsMarked       bool
	Title          string
	Content        string
	ContentPreview string
	UpdatedLabel   string
}

type TaskNoteDetailVM struct {
	ID        string
	TaskID    string
	TaskTitle string
	NoteType  string
	Title     string
	Content   string
	Notes     string
	Error     string

	PersonName        string
	Company           string
	RoleTitle         string
	Platform          string
	ProfileURL        string
	MessageContent    string
	SentAt            string
	ReplyStatus       string
	ReplyAt           string
	JobTitle          string
	JobURL            string
	ApplicationStatus string
	AppliedAt         string
	ResumeVersion     string
	FitScore          string
	Source            string

	NoteTypes            []model.NoteType
	ReplyStatuses        []model.ReplyStatus
	ApplicationStatuses  []model.ApplicationStatus
	ApplicationSources   []model.ApplicationSource
	IsMarked             bool
}

// ---------------------------------------------------------------------------
// Routes — registered from TasksHandler.Register
// ---------------------------------------------------------------------------

func (h *TasksHandler) registerNotesRoutes(g *gin.RouterGroup) {
	g.GET("/:id/notes", h.notesModal)
	g.GET("/:id/notes/list", h.notesList)
	g.GET("/:id/notes/new", h.noteNewForm)
	g.GET("/:id/notes/:noteId", h.noteDetail)
	g.POST("/:id/notes", h.noteCreate)
	g.PATCH("/:id/notes/:noteId", h.noteUpdate)
	g.DELETE("/:id/notes/:noteId", h.noteDelete)
	g.POST("/:id/notes/:noteId/mark", h.noteMark)
}

func (h *TasksHandler) notesModal(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	vm := h.buildNotesModal(ctx, t, c.Query("selected"))
	h.rd.RenderPartial(c, "task_notes_modal", vm)
}

func (h *TasksHandler) notesList(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	selected := c.Query("selected")
	h.rd.RenderPartial(c, "task_notes_table", h.buildNotesTable(ctx, t, selected))
}

func (h *TasksHandler) noteNewForm(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	vm := emptyNoteFormVM(taskID.String(), t.Title)
	vm.NoteType = string(model.NoteTypeGeneral)
	h.rd.RenderPartial(c, "task_note_detail", vm)
}

func (h *TasksHandler) noteDetail(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	noteID, ok := h.parseNoteID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	vm, err := h.buildNoteDetail(ctx, t, noteID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	h.rd.RenderPartial(c, "task_note_detail", vm)
}

func (h *TasksHandler) noteCreate(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	if h.notes == nil {
		c.String(http.StatusServiceUnavailable, "notes unavailable")
		return
	}
	form, err := parseNoteRequestForm(c)
	if err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	in, _ := service.ParseTaskNoteForm(taskID, form)
	n, err := h.notes.Create(ctx, in)
	if err != nil {
		setToast(c, "warning", noteHumanError(err))
		c.Status(http.StatusUnprocessableEntity)
		vm := noteFormFromPost(taskID.String(), t.Title, form)
		vm.Error = noteHumanError(err)
		h.rd.RenderPartial(c, "task_note_detail", vm)
		return
	}
	setToast(c, "success", "Note created")
	c.Header("HX-Trigger", fmt.Sprintf(`{"task-notes-changed":{"taskId":"%s","noteId":"%s"},"job-hunt-dashboard-changed":true,"marked-note-changed":true}`, taskID, n.ID))
	h.rd.RenderPartial(c, "task_notes_panel", h.buildNotesModal(ctx, t, n.ID.String()))
}

func (h *TasksHandler) noteUpdate(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	noteID, ok := h.parseNoteID(c)
	if !ok {
		return
	}
	if h.notes == nil {
		c.String(http.StatusServiceUnavailable, "notes unavailable")
		return
	}
	form, err := parseNoteRequestForm(c)
	if err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	upd := service.ParseTaskNoteUpdateForm(form)
	in, _ := service.ParseTaskNoteForm(taskID, form)
	service.ApplyNoteFormTitleIfEmpty(in, &upd)
	_, err = h.notes.Update(ctx, noteID, upd)
	if err != nil {
		setToast(c, "warning", noteHumanError(err))
		c.Status(http.StatusUnprocessableEntity)
		vm := noteFormFromPost(taskID.String(), t.Title, form)
		vm.ID = noteID.String()
		vm.Error = noteHumanError(err)
		h.rd.RenderPartial(c, "task_note_detail", vm)
		return
	}
	setToast(c, "success", "Note saved")
	c.Header("HX-Trigger", markedNoteChangedTrigger(taskID.String(), noteID.String()))
	h.rd.RenderPartial(c, "task_notes_panel", h.buildNotesModal(ctx, t, noteID.String()))
}

func (h *TasksHandler) noteMark(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	noteID, ok := h.parseNoteID(c)
	if !ok {
		return
	}
	if h.notes == nil {
		c.String(http.StatusServiceUnavailable, "notes unavailable")
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	marked, err := h.notes.ToggleMarked(ctx, noteID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	if marked {
		setToast(c, "success", "Note pinned to tasks page")
	} else {
		setToast(c, "info", "Note unpinned")
	}
	c.Header("HX-Trigger", markedNoteChangedTrigger(taskID.String(), noteID.String()))
	vm, err := h.buildNoteDetail(ctx, t, noteID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	h.rd.RenderPartial(c, "task_note_detail", vm)
}

func (h *TasksHandler) noteDelete(c *gin.Context) {
	taskID, ok := h.parseID(c)
	if !ok {
		return
	}
	noteID, ok := h.parseNoteID(c)
	if !ok {
		return
	}
	if h.notes == nil {
		c.String(http.StatusServiceUnavailable, "notes unavailable")
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	if err := h.notes.Delete(ctx, noteID); err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	setToast(c, "info", "Note deleted")
	c.Header("HX-Trigger", fmt.Sprintf(`{"task-notes-changed":{"taskId":"%s"},"job-hunt-dashboard-changed":true,"marked-note-changed":true}`, taskID))
	h.rd.RenderPartial(c, "task_notes_panel", h.buildNotesModal(ctx, t, ""))
}

// ---------------------------------------------------------------------------
// Builders
// ---------------------------------------------------------------------------

func (h *TasksHandler) buildNotesModal(ctx context.Context, t *model.Task, selectedID string) TaskNotesModalVM {
	table := h.buildNotesTable(ctx, t, selectedID)
	vm := TaskNotesModalVM{
		TaskID:    t.ID.String(),
		TaskTitle: t.Title,
		Notes:     table.Notes,
		Empty:     table.Empty,
	}
	if selectedID != "" {
		if detail, err := h.buildNoteDetail(ctx, t, uuid.MustParse(selectedID)); err == nil {
			vm.Selected = &detail
		}
	}
	return vm
}

func (h *TasksHandler) buildNotesTable(ctx context.Context, t *model.Task, selectedID string) TaskNotesTableVM {
	vm := TaskNotesTableVM{
		TaskID:     t.ID.String(),
		TaskTitle:  t.Title,
		SelectedID: selectedID,
		Notes:      []TaskNoteRowVM{},
	}
	if h.notes == nil {
		vm.Empty = true
		return vm
	}
	items, err := h.notes.ListByTask(ctx, t.ID)
	if err != nil {
		h.log.Warn("task_notes.list", slog.String("err", err.Error()))
		vm.Empty = true
		return vm
	}
	now := h.clock.Now()
	for _, n := range items {
		vm.Notes = append(vm.Notes, h.toNoteRowVM(t, n, now))
	}
	vm.Empty = len(vm.Notes) == 0
	return vm
}

func (h *TasksHandler) buildNoteDetail(ctx context.Context, t *model.Task, noteID uuid.UUID) (TaskNoteDetailVM, error) {
	if h.notes == nil {
		return TaskNoteDetailVM{}, errors.New("notes unavailable")
	}
	n, err := h.notes.Get(ctx, noteID)
	if err != nil {
		return TaskNoteDetailVM{}, err
	}
	if n.TaskID != t.ID {
		return TaskNoteDetailVM{}, model.ErrTaskNoteNotFound
	}
	return noteDetailFromModel(n, t.Title), nil
}

func (h *TasksHandler) toNoteRowVM(t *model.Task, n *model.TaskNote, now time.Time) TaskNoteRowVM {
	preview := strings.TrimSpace(n.EffectiveNotes())
	if preview == "" {
		preview = noteRowPreview(n)
	}
	if preview == "" {
		preview = "—"
	} else if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	return TaskNoteRowVM{
		ID:             n.ID.String(),
		TaskID:         t.ID.String(),
		TaskTitle:      t.Title,
		NoteType:       string(n.NoteType),
		NoteTypeLabel:  n.NoteType.Label(),
		IsMarked:       n.IsMarked,
		Title:          n.Title,
		Content:        n.EffectiveNotes(),
		ContentPreview: preview,
		UpdatedLabel:   formatNoteUpdated(n.UpdatedAt, now),
	}
}

func noteRowPreview(n *model.TaskNote) string {
	switch n.NoteType {
	case model.NoteTypeDM:
		return strings.TrimSpace(n.PersonName + " @ " + n.Company)
	case model.NoteTypeJobApp:
		return strings.TrimSpace(n.Company + " — " + n.JobTitle)
	default:
		return ""
	}
}

func emptyNoteFormVM(taskID, taskTitle string) TaskNoteDetailVM {
	vm := TaskNoteDetailVM{
		TaskID:              taskID,
		TaskTitle:           taskTitle,
		NoteTypes:           model.AllNoteTypes(),
		ReplyStatuses:       model.AllReplyStatuses(),
		ApplicationStatuses: model.AllApplicationStatuses(),
		ApplicationSources:  model.AllApplicationSources(),
	}
	return vm
}

func noteDetailFromModel(n *model.TaskNote, taskTitle string) TaskNoteDetailVM {
	vm := emptyNoteFormVM(n.TaskID.String(), taskTitle)
	vm.ID = n.ID.String()
	vm.NoteType = string(n.NoteType)
	vm.Title = n.Title
	vm.Content = n.EffectiveNotes()
	vm.Notes = n.EffectiveNotes()
	vm.PersonName = n.PersonName
	vm.Company = n.Company
	vm.RoleTitle = n.RoleTitle
	vm.Platform = n.Platform
	vm.ProfileURL = n.ProfileURL
	vm.MessageContent = n.MessageContent
	vm.SentAt = formatNoteInputTime(n.SentAt)
	vm.ReplyStatus = string(n.ReplyStatus)
	vm.ReplyAt = formatNoteInputTime(n.ReplyAt)
	vm.JobTitle = n.JobTitle
	vm.JobURL = n.JobURL
	vm.ApplicationStatus = string(n.ApplicationStatus)
	vm.AppliedAt = formatNoteInputTime(n.AppliedAt)
	vm.ResumeVersion = n.ResumeVersion
	if n.FitScore != nil {
		vm.FitScore = fmt.Sprintf("%d", *n.FitScore)
	}
	vm.Source = string(n.Source)
	vm.IsMarked = n.IsMarked
	return vm
}

func noteFormFromPost(taskID, taskTitle string, form map[string][]string) TaskNoteDetailVM {
	get := func(k string) string {
		if v, ok := form[k]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}
	vm := emptyNoteFormVM(taskID, taskTitle)
	vm.NoteType = get("note_type")
	vm.Title = get("title")
	vm.Content = get("content")
	vm.Notes = get("notes")
	vm.PersonName = get("person_name")
	vm.Company = get("company")
	vm.RoleTitle = get("role_title")
	vm.Platform = get("platform")
	vm.ProfileURL = get("profile_url")
	vm.MessageContent = get("message_content")
	vm.SentAt = get("sent_at")
	vm.ReplyStatus = get("reply_status")
	vm.ReplyAt = get("reply_at")
	vm.JobTitle = get("job_title")
	vm.JobURL = get("job_url")
	vm.ApplicationStatus = get("application_status")
	vm.AppliedAt = get("applied_at")
	vm.ResumeVersion = get("resume_version")
	vm.FitScore = get("fit_score")
	vm.Source = get("source")
	return vm
}

func formatNoteInputTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04")
}

func formatNoteUpdated(at, now time.Time) string {
	if at.IsZero() {
		return "—"
	}
	if hSameDay(at, now) {
		return "Today " + at.Format("3:04 PM")
	}
	if hSameDay(at, now.AddDate(0, 0, -1)) {
		return "Yesterday"
	}
	return at.Format("Jan 2, 2006")
}

func hSameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (h *TasksHandler) parseNoteID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid note id")
		return uuid.Nil, false
	}
	return id, true
}

func noteHumanError(err error) string {
	switch {
	case errors.Is(err, model.ErrTaskNoteTitleEmpty):
		return "Title is required."
	case errors.Is(err, model.ErrInvalidNoteType):
		return "Invalid note type."
	case errors.Is(err, model.ErrTaskNotFound):
		return "Task not found."
	case errors.Is(err, model.ErrTaskNoteNotFound):
		return "Note not found."
	}
	return "Could not save the note."
}
