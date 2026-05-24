package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

type reviewHandler struct {
	svc *service.DailyReviewService
}

func newReviewHandler(svc *service.DailyReviewService) *reviewHandler {
	return &reviewHandler{svc: svc}
}

func (h *reviewHandler) register(g *gin.RouterGroup) {
	rv := g.Group("/reviews")
	{
		rv.GET("",         h.list)
		rv.GET("/today",   h.getToday)
		rv.PUT("/today",   h.upsertToday)
		rv.GET("/:date",   h.get)
		rv.PUT("/:date",   h.upsert)
		rv.DELETE("/:date", h.delete)
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *reviewHandler) upsertToday(c *gin.Context) {
	h.upsertOnDate(c, time.Now().UTC())
}

func (h *reviewHandler) upsert(c *gin.Context) {
	date, ok := parseDateParam(c, "date")
	if !ok {
		return
	}
	h.upsertOnDate(c, date)
}

func (h *reviewHandler) upsertOnDate(c *gin.Context, date time.Time) {
	var req upsertReviewRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBindError(c, err)
			return
		}
	}
	rv, err := h.svc.Upsert(c.Request.Context(), req.toInput(date))
	if err != nil {
		respondReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, reviewResponse(rv))
}

func (h *reviewHandler) get(c *gin.Context) {
	date, ok := parseDateParam(c, "date")
	if !ok {
		return
	}
	rv, err := h.svc.GetByDate(c.Request.Context(), date)
	if err != nil {
		respondReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, reviewResponse(rv))
}

func (h *reviewHandler) getToday(c *gin.Context) {
	rv, err := h.svc.GetByDate(c.Request.Context(), time.Now().UTC())
	if err != nil {
		respondReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, reviewResponse(rv))
}

func (h *reviewHandler) list(c *gin.Context) {
	f := repository.ReviewFilter{}
	if v := c.Query("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
			return
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
			return
		}
		f.To = &t
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
		respondReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": reviewListResponse(items),
		"count": len(items),
	})
}

func (h *reviewHandler) delete(c *gin.Context) {
	date, ok := parseDateParam(c, "date")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), date); err != nil {
		respondReviewError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func parseDateParam(c *gin.Context, name string) (time.Time, bool) {
	raw := c.Param(name)
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, expected YYYY-MM-DD"})
		return time.Time{}, false
	}
	return t, true
}

func respondReviewError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrReviewNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrInvalidEnergyLevel),
		errors.Is(err, model.ErrInvalidProductivity),
		errors.Is(err, model.ErrBlockerEmpty),
		errors.Is(err, model.ErrWinEmpty):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
