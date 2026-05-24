package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

type sessionHandler struct {
	svc *service.TaskSessionService
}

func newSessionHandler(svc *service.TaskSessionService) *sessionHandler {
	return &sessionHandler{svc: svc}
}

// register mounts both task-scoped and session-scoped routes.
//
//	POST   /tasks/:id/sessions/start    -> begin a new session
//	GET    /tasks/:id/sessions/current  -> active|paused session, if any
//	GET    /tasks/:id/sessions          -> all sessions for the task
//
//	GET    /sessions                         -> list all (filtered)
//	GET    /sessions/:id                     -> one
//	POST   /sessions/:id/pause               -> active -> paused
//	POST   /sessions/:id/resume              -> paused -> active
//	POST   /sessions/:id/stop                -> end (no task completion)
//	POST   /sessions/:id/complete            -> end + mark task completed
//	DELETE /sessions/:id                     -> delete
func (h *sessionHandler) register(g *gin.RouterGroup) {
	taskScoped := g.Group("/tasks/:id/sessions")
	{
		taskScoped.POST("/start",  h.start)
		taskScoped.GET("/current", h.current)
		taskScoped.GET("",         h.listByTask)
	}

	s := g.Group("/sessions")
	{
		s.GET("",              h.list)
		s.GET("/:id",          h.get)
		s.POST("/:id/pause",   h.pause)
		s.POST("/:id/resume",  h.resume)
		s.POST("/:id/stop",    h.stop)
		s.POST("/:id/complete", h.complete)
		s.DELETE("/:id",       h.delete)
	}
}

// ---------------------------------------------------------------------------
// task-scoped handlers
// ---------------------------------------------------------------------------

func (h *sessionHandler) start(c *gin.Context) {
	taskID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.Start(c.Request.Context(), taskID)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, sessionResponse(s, time.Now()))
}

func (h *sessionHandler) current(c *gin.Context) {
	taskID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.CurrentForTask(c.Request.Context(), taskID)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionResponse(s, time.Now()))
}

func (h *sessionHandler) listByTask(c *gin.Context) {
	taskID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.List(c.Request.Context(), repository.SessionFilter{
		TaskID: &taskID,
	})
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": sessionListResponse(items, time.Now()),
		"count": len(items),
	})
}

// ---------------------------------------------------------------------------
// session-scoped handlers
// ---------------------------------------------------------------------------

func (h *sessionHandler) get(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionResponse(s, time.Now()))
}

func (h *sessionHandler) pause(c *gin.Context) {
	h.transition(c, h.svc.Pause)
}

func (h *sessionHandler) resume(c *gin.Context) {
	h.transition(c, h.svc.Resume)
}

func (h *sessionHandler) stop(c *gin.Context) {
	h.finish(c, h.svc.Stop)
}

func (h *sessionHandler) complete(c *gin.Context) {
	h.finish(c, h.svc.Complete)
}

func (h *sessionHandler) delete(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondSessionError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *sessionHandler) list(c *gin.Context) {
	f := repository.SessionFilter{}

	if v := c.Query("task_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
			return
		}
		f.TaskID = &id
	}
	if vals := c.QueryArray("status"); len(vals) > 0 {
		for _, v := range splitCSV(vals) {
			st := model.SessionStatus(v)
			if !st.IsValid() {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status: " + v})
				return
			}
			f.Statuses = append(f.Statuses, st)
		}
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		f.Limit = n
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset"})
			return
		}
		f.Offset = n
	}

	items, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": sessionListResponse(items, time.Now()),
		"count": len(items),
	})
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

// transitionFn is the shape of Pause / Resume — pure state change, no body.
type transitionFn func(ctx context.Context, id uuid.UUID) (*model.TaskSession, error)

func (h *sessionHandler) transition(c *gin.Context, fn transitionFn) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	s, err := fn(c.Request.Context(), id)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionResponse(s, time.Now()))
}

// finishFn is the shape of Stop / Complete — accepts an optional body with
// interruptions / quality / notes.
type finishFn func(ctx context.Context, id uuid.UUID, in service.FinishSessionInput) (*model.TaskSession, error)

func (h *sessionHandler) finish(c *gin.Context, fn finishFn) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req finishSessionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBindError(c, err)
			return
		}
	}
	s, err := fn(c.Request.Context(), id, req.toInput())
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionResponse(s, time.Now()))
}

func parseUUIDParam(c *gin.Context, name string) (uuid.UUID, bool) {
	raw := c.Param(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return uuid.Nil, false
	}
	return id, true
}

func respondSessionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrSessionNotFound),
		errors.Is(err, model.ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrSessionAlreadyRunning),
		errors.Is(err, model.ErrInvalidSessionTransition),
		errors.Is(err, model.ErrInvalidTransition),
		errors.Is(err, model.ErrSessionNotRunning):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrInvalidSessionStatus),
		errors.Is(err, model.ErrInvalidQuality),
		errors.Is(err, model.ErrInvalidInterruptions):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
