package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

type jobHuntDashboardHandler struct {
	notes *service.TaskNoteService
}

func newJobHuntDashboardHandler(notes *service.TaskNoteService) *jobHuntDashboardHandler {
	return &jobHuntDashboardHandler{notes: notes}
}

func registerJobHuntDashboard(r *gin.Engine, notes *service.TaskNoteService) {
	if notes == nil {
		return
	}
	h := newJobHuntDashboardHandler(notes)
	dash := r.Group("/api/dashboard")
	{
		dash.GET("/job-hunt-summary", h.summary)
		dash.GET("/dms", h.listDMs)
		dash.GET("/applications", h.listApplications)
	}
	r.GET("/api/task-notes/:id", h.getNote)
	r.POST("/api/tasks/:task_id/notes", h.createNote)
}

func (h *jobHuntDashboardHandler) summary(c *gin.Context) {
	s, err := h.notes.JobHuntSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total_dms":           s.TotalDMs,
		"total_applications":  s.TotalApplications,
		"dm_tasks":            s.DMTasks,
		"application_tasks":   s.ApplicationTasks,
	})
}

func (h *jobHuntDashboardHandler) listDMs(c *gin.Context) {
	items, err := h.notes.ListDMs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, n := range items {
		out = append(out, noteListItemJSON(n))
	}
	c.JSON(http.StatusOK, out)
}

func (h *jobHuntDashboardHandler) listApplications(c *gin.Context) {
	items, err := h.notes.ListApplications(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, n := range items {
		out = append(out, noteListItemJSON(n))
	}
	c.JSON(http.StatusOK, out)
}

func (h *jobHuntDashboardHandler) getNote(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}
	n, err := h.notes.GetWithTask(c.Request.Context(), id)
	if err != nil {
		respondNoteError(c, err)
		return
	}
	c.JSON(http.StatusOK, noteDetailJSON(n))
}

type createTaskNoteRequest struct {
	NoteType          string  `json:"note_type"`
	Title             string  `json:"title"`
	Notes             string  `json:"notes"`
	Content           string  `json:"content"`
	PersonName        string  `json:"person_name"`
	Company           string  `json:"company"`
	RoleTitle         string  `json:"role_title"`
	Platform          string  `json:"platform"`
	ProfileURL        string  `json:"profile_url"`
	MessageContent    string  `json:"message_content"`
	SentAt            string  `json:"sent_at"`
	ReplyStatus       string  `json:"reply_status"`
	ReplyAt           string  `json:"reply_at"`
	JobTitle          string  `json:"job_title"`
	JobURL            string  `json:"job_url"`
	ApplicationStatus string  `json:"application_status"`
	AppliedAt         string  `json:"applied_at"`
	ResumeVersion     string  `json:"resume_version"`
	FitScore          *int    `json:"fit_score"`
	Source            string  `json:"source"`
}

func (h *jobHuntDashboardHandler) createNote(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("task_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}
	var req createTaskNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	in := service.CreateTaskNoteInput{
		TaskID:            taskID,
		NoteType:          model.NoteType(req.NoteType),
		Title:             req.Title,
		Notes:             req.Notes,
		Content:           req.Content,
		PersonName:        req.PersonName,
		Company:           req.Company,
		RoleTitle:         req.RoleTitle,
		Platform:          req.Platform,
		ProfileURL:        req.ProfileURL,
		MessageContent:    req.MessageContent,
		ReplyStatus:       model.ReplyStatus(req.ReplyStatus),
		JobTitle:          req.JobTitle,
		JobURL:            req.JobURL,
		ApplicationStatus: model.ApplicationStatus(req.ApplicationStatus),
		ResumeVersion:     req.ResumeVersion,
		FitScore:          req.FitScore,
		Source:            model.ApplicationSource(req.Source),
	}
	in.SentAt = parseAPITime(req.SentAt)
	in.ReplyAt = parseAPITime(req.ReplyAt)
	in.AppliedAt = parseAPITime(req.AppliedAt)

	n, err := h.notes.Create(c.Request.Context(), in)
	if err != nil {
		respondNoteError(c, err)
		return
	}
	withTask, _ := h.notes.GetWithTask(c.Request.Context(), n.ID)
	if withTask == nil {
		withTask = &model.TaskNoteWithTask{TaskNote: *n}
	}
	c.JSON(http.StatusCreated, noteDetailJSON(withTask))
}

func noteListItemJSON(n *model.TaskNoteWithTask) gin.H {
	item := gin.H{
		"id":         n.ID.String(),
		"task_id":    n.TaskID.String(),
		"task_title": n.TaskTitle,
		"note_type":  n.NoteType,
		"title":      n.Title,
		"created_at": n.CreatedAt,
		"updated_at": n.UpdatedAt,
	}
	switch n.NoteType {
	case model.NoteTypeDM:
		item["date"] = formatNoteDate(n.SentAt, n.CreatedAt)
		item["person_name"] = n.PersonName
		item["company"] = n.Company
		item["role_title"] = n.RoleTitle
		item["platform"] = n.Platform
		item["reply_status"] = n.ReplyStatus
	case model.NoteTypeJobApp:
		item["date"] = formatNoteDate(n.AppliedAt, n.CreatedAt)
		item["company"] = n.Company
		item["job_title"] = n.JobTitle
		item["application_status"] = n.ApplicationStatus
		item["fit_score"] = n.FitScore
		item["source"] = n.Source
	}
	return item
}

func noteDetailJSON(n *model.TaskNoteWithTask) gin.H {
	return gin.H{
		"id":                 n.ID.String(),
		"task_id":            n.TaskID.String(),
		"task_title":         n.TaskTitle,
		"note_type":          n.NoteType,
		"title":              n.Title,
		"notes":              n.EffectiveNotes(),
		"person_name":        n.PersonName,
		"company":            n.Company,
		"role_title":         n.RoleTitle,
		"platform":           n.Platform,
		"profile_url":        n.ProfileURL,
		"message_content":    n.MessageContent,
		"sent_at":            n.SentAt,
		"reply_status":       n.ReplyStatus,
		"reply_at":           n.ReplyAt,
		"job_title":          n.JobTitle,
		"job_url":            n.JobURL,
		"application_status": n.ApplicationStatus,
		"applied_at":         n.AppliedAt,
		"resume_version":     n.ResumeVersion,
		"fit_score":          n.FitScore,
		"source":             n.Source,
		"created_at":         n.CreatedAt,
		"updated_at":         n.UpdatedAt,
	}
}

func formatNoteDate(primary *time.Time, fallback time.Time) string {
	if primary != nil && !primary.IsZero() {
		return primary.Format(time.RFC3339)
	}
	return fallback.Format(time.RFC3339)
}

func parseAPITime(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return &t
		}
	}
	return nil
}

func respondNoteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrTaskNoteNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrTaskNoteTitleEmpty),
		errors.Is(err, model.ErrInvalidNoteType):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
