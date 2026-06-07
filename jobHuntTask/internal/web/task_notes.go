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
	TaskID    string
	TaskTitle string
	Notes     []TaskNoteRowVM
	SelectedID string
	Empty     bool
}

type TaskNoteRowVM struct {
	ID            string
	TaskID        string
	TaskTitle     string
	Title         string
	Content       string
	ContentPreview string
	UpdatedLabel  string
}

type TaskNoteDetailVM struct {
	ID        string
	TaskID    string
	TaskTitle string
	Title     string
	Content   string
	Error     string
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
	vm := h.buildNotesModal(ctx, t, "")
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
	h.rd.RenderPartial(c, "task_note_detail", TaskNoteDetailVM{
		TaskID:    taskID.String(),
		TaskTitle: t.Title,
	})
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
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	n, err := h.notes.Create(ctx, service.CreateTaskNoteInput{
		TaskID:  taskID,
		Title:   c.PostForm("title"),
		Content: c.PostForm("content"),
	})
	if err != nil {
		setToast(c, "warning", noteHumanError(err))
		c.Status(http.StatusUnprocessableEntity)
		h.rd.RenderPartial(c, "task_note_detail", TaskNoteDetailVM{
			TaskID:    taskID.String(),
			TaskTitle: t.Title,
			Title:     c.PostForm("title"),
			Content:   c.PostForm("content"),
			Error:     noteHumanError(err),
		})
		return
	}
	setToast(c, "success", "Note created")
	c.Header("HX-Trigger", fmt.Sprintf(`{"task-notes-changed":{"taskId":"%s","noteId":"%s"}}`, taskID, n.ID))
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
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "bad form")
		return
	}
	ctx := c.Request.Context()
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		h.notFoundOrErr(c, err)
		return
	}
	title := c.PostForm("title")
	content := c.PostForm("content")
	_, err = h.notes.Update(ctx, noteID, service.UpdateTaskNoteInput{
		Title:   &title,
		Content: &content,
	})
	if err != nil {
		setToast(c, "warning", noteHumanError(err))
		c.Status(http.StatusUnprocessableEntity)
		h.rd.RenderPartial(c, "task_note_detail", TaskNoteDetailVM{
			ID:        noteID.String(),
			TaskID:    taskID.String(),
			TaskTitle: t.Title,
			Title:     title,
			Content:   content,
			Error:     noteHumanError(err),
		})
		return
	}
	setToast(c, "success", "Note saved")
	c.Header("HX-Trigger", fmt.Sprintf(`{"task-notes-changed":{"taskId":"%s","noteId":"%s"}}`, taskID, noteID))
	h.rd.RenderPartial(c, "task_notes_panel", h.buildNotesModal(ctx, t, noteID.String()))
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
	c.Header("HX-Trigger", fmt.Sprintf(`{"task-notes-changed":{"taskId":"%s"}}`, taskID))
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
	return TaskNoteDetailVM{
		ID:        n.ID.String(),
		TaskID:    t.ID.String(),
		TaskTitle: t.Title,
		Title:     n.Title,
		Content:   n.Content,
	}, nil
}

func (h *TasksHandler) toNoteRowVM(t *model.Task, n *model.TaskNote, now time.Time) TaskNoteRowVM {
	preview := strings.TrimSpace(n.Content)
	if preview == "" {
		preview = "—"
	} else if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	return TaskNoteRowVM{
		ID:             n.ID.String(),
		TaskID:         t.ID.String(),
		TaskTitle:      t.Title,
		Title:          n.Title,
		Content:        n.Content,
		ContentPreview: preview,
		UpdatedLabel:   formatNoteUpdated(n.UpdatedAt, now),
	}
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
	case errors.Is(err, model.ErrTaskNotFound):
		return "Task not found."
	case errors.Is(err, model.ErrTaskNoteNotFound):
		return "Note not found."
	}
	return "Could not save the note."
}
