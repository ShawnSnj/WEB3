package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shawn/jobhunttask/internal/crm/repository"
	crmsvc "github.com/shawn/jobhunttask/internal/crm/service"
)

// registerJobsAlias exposes /api/v1/jobs/* as shorthand for CRM job endpoints.
func registerJobsAlias(r *gin.RouterGroup, crm *crmsvc.CRM) {
	h := &crmHandler{crm: crm}
	j := r.Group("/jobs")
	j.POST("/fetch", h.fetchJobs)
	j.GET("/fetch/status", h.fetchStatus)
	j.GET("", h.listJobsAlias)
}

func (h *crmHandler) listJobsAlias(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	tier := c.DefaultQuery("tier", "AB")
	f := repository.JobFilter{
		Tier:           tier,
		MinFinalScore:  65,
		ScoredOnly:     true,
		ExcludeSkipped: true,
		HideFrontend:   true,
		HideJunior:     true,
		HideNonRemote:  true,
		HideMarketing:  true,
		Limit:          limit,
		Active:         true,
		Search:         c.Query("q"),
	}
	jobs, err := h.crm.ListJobs(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}
