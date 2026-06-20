package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/model"
)

// MarkedNotePanelVM renders the pinned note card on the tasks page header.
type MarkedNotePanelVM struct {
	HasNote       bool
	ID            string
	TaskID        string
	TaskTitle     string
	Title         string
	NoteTypeLabel string
	Body          string
}

func (h *TasksHandler) markedNotePanel(c *gin.Context) {
	vm := h.buildMarkedNotePanel(c.Request.Context())
	h.rd.RenderPartial(c, "marked_note_panel", vm)
}

func (h *TasksHandler) buildMarkedNotePanel(ctx context.Context) MarkedNotePanelVM {
	vm := MarkedNotePanelVM{}
	if h.notes == nil {
		return vm
	}
	n, err := h.notes.GetMarked(ctx)
	if err != nil || n == nil {
		return vm
	}
	vm.HasNote = true
	vm.ID = n.ID.String()
	vm.TaskID = n.TaskID.String()
	vm.TaskTitle = n.TaskTitle
	vm.Title = n.Title
	vm.NoteTypeLabel = n.NoteType.Label()
	vm.Body = markedNoteBody(n)
	return vm
}

func markedNoteBody(n *model.TaskNoteWithTask) string {
	if body := strings.TrimSpace(n.EffectiveNotes()); body != "" {
		return body
	}
	switch n.NoteType {
	case model.NoteTypeDM:
		parts := []string{}
		if p := strings.TrimSpace(n.PersonName); p != "" {
			parts = append(parts, p)
		}
		if c := strings.TrimSpace(n.Company); c != "" {
			parts = append(parts, c)
		}
		if r := strings.TrimSpace(n.RoleTitle); r != "" {
			parts = append(parts, r)
		}
		if len(parts) > 0 {
			return strings.Join(parts, " · ")
		}
		return strings.TrimSpace(n.MessageContent)
	case model.NoteTypeJobApp:
		parts := []string{}
		if c := strings.TrimSpace(n.Company); c != "" {
			parts = append(parts, c)
		}
		if j := strings.TrimSpace(n.JobTitle); j != "" {
			parts = append(parts, j)
		}
		if len(parts) > 0 {
			return strings.Join(parts, " — ")
		}
	}
	return ""
}

func markedNoteChangedTrigger(taskID, noteID string) string {
	return fmt.Sprintf(`{"task-notes-changed":{"taskId":"%s","noteId":"%s"},"job-hunt-dashboard-changed":true,"marked-note-changed":true}`, taskID, noteID)
}
