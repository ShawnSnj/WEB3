package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/service"
)

type metricsHandler struct {
	svc *service.MetricsService
}

func newMetricsHandler(svc *service.MetricsService) *metricsHandler {
	return &metricsHandler{svc: svc}
}

func (h *metricsHandler) register(g *gin.RouterGroup) {
	m := g.Group("/metrics")
	{
		m.GET("/dashboard",  h.dashboard)
		m.GET("/today",      h.today)
		m.GET("/weekly",     h.weekly)
		m.GET("/trend",      h.trend)
		m.GET("/streak",     h.streak)
		m.GET("/categories", h.categories)
	}
}

func (h *metricsHandler) dashboard(c *gin.Context) {
	out, err := h.svc.Dashboard(c.Request.Context())
	if err != nil {
		respondMetricsError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *metricsHandler) today(c *gin.Context) {
	out, err := h.svc.Today(c.Request.Context())
	if err != nil {
		respondMetricsError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *metricsHandler) weekly(c *gin.Context) {
	out, err := h.svc.Weekly(c.Request.Context())
	if err != nil {
		respondMetricsError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *metricsHandler) trend(c *gin.Context) {
	out, err := h.svc.TrendComparison(c.Request.Context())
	if err != nil {
		respondMetricsError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *metricsHandler) streak(c *gin.Context) {
	out, err := h.svc.Streak(c.Request.Context())
	if err != nil {
		respondMetricsError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// categories accepts optional ?from=YYYY-MM-DD&to=YYYY-MM-DD; both must be
// provided together or neither (in which case the rolling week is used).
func (h *metricsHandler) categories(c *gin.Context) {
	var from, to time.Time
	rawFrom := c.Query("from")
	rawTo := c.Query("to")
	switch {
	case rawFrom == "" && rawTo == "":
		// use defaults inside the service
	case rawFrom == "" || rawTo == "":
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to must both be provided"})
		return
	default:
		var err error
		from, err = time.Parse("2006-01-02", rawFrom)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from, expected YYYY-MM-DD"})
			return
		}
		to, err = time.Parse("2006-01-02", rawTo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to, expected YYYY-MM-DD"})
			return
		}
		if !to.After(from) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be after from"})
			return
		}
	}

	out, err := h.svc.Categories(c.Request.Context(), from, to)
	if err != nil {
		respondMetricsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out,
		"count": len(out),
	})
}

func respondMetricsError(c *gin.Context, _ error) {
	// Metrics endpoints are pure reads — there are no domain-shape errors
	// to surface; any failure here is an infrastructure error.
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
