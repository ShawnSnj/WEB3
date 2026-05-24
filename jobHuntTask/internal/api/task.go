package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

// taskHandler bundles the service dependency for all /tasks endpoints.
type taskHandler struct {
	svc *service.TaskService
}

func newTaskHandler(svc *service.TaskService) *taskHandler {
	return &taskHandler{svc: svc}
}

// register wires all task routes onto the given group.
func (h *taskHandler) register(g *gin.RouterGroup) {
	t := g.Group("/tasks")
	{
		t.POST("",       h.create)
		t.GET("",        h.list)
		t.GET("/overdue", h.listOverdue)
		t.GET("/:id",    h.get)
		t.PATCH("/:id",  h.update)
		t.DELETE("/:id", h.delete)

		t.POST("/:id/start",       h.markInProgress)
		t.POST("/:id/complete",    h.markCompleted)
		t.POST("/:id/miss",        h.markMissed)
		t.POST("/:id/carry-over",  h.carryOver)
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *taskHandler) create(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), req.toInput())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, taskResponse(t, time.Now()))
}

func (h *taskHandler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, taskResponse(t, time.Now()))
}

func (h *taskHandler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	t, err := h.svc.Update(c.Request.Context(), id, req.toInput())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, taskResponse(t, time.Now()))
}

func (h *taskHandler) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *taskHandler) list(c *gin.Context) {
	filter, err := parseListFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tasks, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": taskListResponse(tasks, time.Now()),
		"count": len(tasks),
	})
}

func (h *taskHandler) listOverdue(c *gin.Context) {
	tasks, err := h.svc.ListOverdue(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": taskListResponse(tasks, time.Now()),
		"count": len(tasks),
	})
}

func (h *taskHandler) markInProgress(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.MarkInProgress(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, taskResponse(t, time.Now()))
}

func (h *taskHandler) markCompleted(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req completeTaskRequest
	// Body is optional — empty body is allowed.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBindError(c, err)
			return
		}
	}
	t, err := h.svc.MarkCompleted(c.Request.Context(), id, req.ActualMinutes)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, taskResponse(t, time.Now()))
}

func (h *taskHandler) markMissed(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.MarkMissed(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, taskResponse(t, time.Now()))
}

func (h *taskHandler) carryOver(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.CarryOverTask(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, taskResponse(t, time.Now()))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.Param("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return uuid.Nil, false
	}
	return id, true
}

func parseListFilter(c *gin.Context) (repository.TaskFilter, error) {
	f := repository.TaskFilter{}

	if vals := c.QueryArray("status"); len(vals) > 0 {
		for _, v := range splitCSV(vals) {
			s := model.Status(v)
			if !s.IsValid() {
				return f, errors.New("invalid status: " + v)
			}
			f.Statuses = append(f.Statuses, s)
		}
	}
	if vals := c.QueryArray("category"); len(vals) > 0 {
		for _, v := range splitCSV(vals) {
			cat := model.Category(v)
			if !cat.IsValid() {
				return f, errors.New("invalid category: " + v)
			}
			f.Categories = append(f.Categories, cat)
		}
	}
	if vals := c.QueryArray("priority"); len(vals) > 0 {
		for _, v := range splitCSV(vals) {
			p := model.Priority(v)
			if !p.IsValid() {
				return f, errors.New("invalid priority: " + v)
			}
			f.Priorities = append(f.Priorities, p)
		}
	}
	if v := c.Query("overdue"); v == "true" || v == "1" {
		f.OnlyOverdue = true
	}
	if v := c.Query("carried_over"); v != "" {
		b := v == "true" || v == "1"
		f.CarriedOver = &b
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return f, errors.New("invalid limit")
		}
		f.Limit = n
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return f, errors.New("invalid offset")
		}
		f.Offset = n
	}
	f.OrderBy = c.Query("order_by")
	return f, nil
}

// splitCSV accepts both repeated query params (?status=a&status=b) and
// comma-separated values (?status=a,b) and flattens to a single slice.
func splitCSV(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// respondBindError emits a structured 400 for validator/binding failures.
func respondBindError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":   "invalid request",
		"details": err.Error(),
	})
}

// respondServiceError maps domain errors to HTTP status codes.
func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrInvalidTransition),
		errors.Is(err, model.ErrTaskNotEligibleCarry):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrTitleRequired),
		errors.Is(err, model.ErrInvalidStatus),
		errors.Is(err, model.ErrInvalidPriority),
		errors.Is(err, model.ErrInvalidCategory),
		errors.Is(err, model.ErrEstimatedNegative),
		errors.Is(err, model.ErrActualNegative):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
