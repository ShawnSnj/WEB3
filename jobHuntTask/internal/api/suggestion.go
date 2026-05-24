package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

type suggestionHandler struct {
	svc *service.SuggestionService
}

func newSuggestionHandler(svc *service.SuggestionService) *suggestionHandler {
	return &suggestionHandler{svc: svc}
}

func (h *suggestionHandler) register(g *gin.RouterGroup) {
	s := g.Group("/suggestions")
	{
		s.GET("",                 h.list)
		s.POST("/refresh",        h.refresh)
		s.GET("/:id",             h.get)
		s.POST("/:id/dismiss",    h.dismiss)
		s.DELETE("/:id",          h.delete)
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// list returns suggestions filtered by ?status= (default: active) and
// optional ?kind= (repeatable).
func (h *suggestionHandler) list(c *gin.Context) {
	f := repository.SuggestionFilter{}

	if statuses := c.QueryArray("status"); len(statuses) > 0 {
		for _, raw := range statuses {
			st := model.SuggestionStatus(raw)
			if !st.IsValid() {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status: " + raw})
				return
			}
			f.Statuses = append(f.Statuses, st)
		}
	} else {
		f.Statuses = []model.SuggestionStatus{model.SuggestionStatusActive}
	}

	for _, raw := range c.QueryArray("kind") {
		k := model.SuggestionKind(raw)
		if !k.IsValid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kind: " + raw})
			return
		}
		f.Kinds = append(f.Kinds, k)
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
		respondSuggestionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": suggestionListResponse(items),
		"count": len(items),
	})
}

func (h *suggestionHandler) refresh(c *gin.Context) {
	res, err := h.svc.Refresh(c.Request.Context())
	if err != nil {
		respondSuggestionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"created":       suggestionListResponse(res.Created),
		"kept":          suggestionListResponse(res.Kept),
		"expired_count": res.ExpiredCount,
		"created_count": len(res.Created),
		"kept_count":    len(res.Kept),
	})
}

func (h *suggestionHandler) get(c *gin.Context) {
	id, ok := parseSuggestionID(c)
	if !ok {
		return
	}
	sg, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondSuggestionError(c, err)
		return
	}
	c.JSON(http.StatusOK, suggestionResponse(sg))
}

func (h *suggestionHandler) dismiss(c *gin.Context) {
	id, ok := parseSuggestionID(c)
	if !ok {
		return
	}
	sg, err := h.svc.Dismiss(c.Request.Context(), id)
	if err != nil {
		respondSuggestionError(c, err)
		return
	}
	c.JSON(http.StatusOK, suggestionResponse(sg))
}

func (h *suggestionHandler) delete(c *gin.Context) {
	id, ok := parseSuggestionID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondSuggestionError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func parseSuggestionID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func respondSuggestionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrSuggestionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrSuggestionInvalidTransition):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrInvalidSuggestionKind),
		errors.Is(err, model.ErrInvalidSuggestionStatus),
		errors.Is(err, model.ErrInvalidSuggestionSeverity),
		errors.Is(err, model.ErrSuggestionTitleEmpty),
		errors.Is(err, model.ErrSuggestionMessageEmpty),
		errors.Is(err, model.ErrSuggestionDedupKeyEmpty):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
