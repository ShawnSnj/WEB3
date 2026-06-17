package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/repository"
	crmsvc "github.com/shawn/jobhunttask/internal/crm/service"
)

type crmHandler struct {
	crm *crmsvc.CRM
}

func newCRMHandler(c *crmsvc.CRM) *crmHandler {
	return &crmHandler{crm: c}
}

func (h *crmHandler) register(r *gin.RouterGroup) {
	g := r.Group("/crm")
	g.GET("/dashboard", h.dashboard)
	g.POST("/pipeline/run", h.runPipeline)
	g.POST("/collect", h.collect)

	g.POST("/jobs/fetch", h.fetchJobs)
	g.GET("/jobs/fetch/status", h.fetchStatus)
	g.POST("/jobs/rescore", h.rescoreJobs)
	g.GET("/jobs", h.listJobs)
	g.GET("/jobs/recommended", h.recommendedJobs)
	g.GET("/jobs/:id/fit", h.getJobFit)
	g.GET("/jobs/:id", h.getJob)
	g.POST("/jobs/:id/score", h.scoreJob)
	g.POST("/jobs/:id/apply", h.applyToJob)
	g.POST("/jobs/:id/action", h.jobAction)
	g.POST("/jobs/:id/resume", h.analyzeResume)

	g.GET("/applications", h.listApplications)
	g.PATCH("/applications/:id", h.updateApplication)

	g.GET("/skills", h.skillGaps)
	g.POST("/skills/analyze", h.analyzeSkills)

	g.GET("/outreach", h.outreach)
	g.POST("/outreach/:contactId/:jobId", h.generateOutreach)
	g.POST("/outreach/complete", h.completeOutreach)

	g.GET("/weekly", h.weeklyReport)
	g.POST("/weekly/generate", h.generateWeekly)

	g.GET("/coach", h.coach)
	g.GET("/profile", h.getProfile)
	g.PUT("/profile", h.updateProfile)

	// Phase 1 — Candidate Master Profile (Resume Intelligence)
	g.GET("/candidate-profile", h.getCandidateProfile)
	g.PUT("/candidate-profile", h.updateCandidateProfile)
	g.POST("/resumes/upload", h.uploadResume)
	g.POST("/resumes/parse", h.parseResumes)

	g.GET("/analytics/applications", h.appAnalytics)

	// Career OS intelligence layers
	g.GET("/market/trends", h.marketTrends)
	g.GET("/interview/readiness", h.interviewReadiness)
	g.GET("/offers/predictions", h.offerPredictions)
	g.GET("/skills/levels", h.userSkills)
}

func (h *crmHandler) dashboard(c *gin.Context) {
	brief, err := h.crm.GetDailyBrief(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, brief)
}

func (h *crmHandler) runPipeline(c *gin.Context) {
	if err := h.crm.RunDailyPipeline(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	brief, _ := h.crm.GetDailyBrief(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"status": "ok", "brief": brief})
}

func (h *crmHandler) collect(c *gin.Context) {
	result, err := h.crm.FetchJobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "result": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *crmHandler) fetchJobs(c *gin.Context) {
	result, err := h.crm.FetchJobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "result": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *crmHandler) fetchStatus(c *gin.Context) {
	status, err := h.crm.GetFetchStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *crmHandler) listJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	tier := c.DefaultQuery("tier", "AB")
	f := repository.JobFilter{
		Tier:           tier,
		MinFinalScore:  65,
		ScoredOnly:     true,
		ExcludeSkipped: true,
		HideFrontend:   c.DefaultQuery("hide_frontend", "true") != "false",
		HideJunior:     c.DefaultQuery("hide_junior", "true") != "false",
		HideNonRemote:  c.DefaultQuery("hide_non_remote", "true") != "false",
		HideMarketing:  c.DefaultQuery("hide_marketing", "true") != "false",
		Limit:          limit,
		Active:         true,
		Search:         c.Query("q"),
	}
	if c.Query("remote") == "true" {
		t := true
		f.Remote = &t
	}
	if c.Query("web3") == "true" {
		t := true
		f.Web3 = &t
	}
	jobs, err := h.crm.ListJobs(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func (h *crmHandler) jobAction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action required: apply, skip, save"})
		return
	}
	if err := h.crm.JobAction(c.Request.Context(), id, body.Action); err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "action": body.Action})
}

func (h *crmHandler) rescoreJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "300"))
	n, err := h.crm.RescoreAll(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rescored": n})
}

func (h *crmHandler) recommendedJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	jobs, err := h.crm.RecommendedJobs(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func (h *crmHandler) getJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	job, err := h.crm.GetJob(c.Request.Context(), id)
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *crmHandler) getJobFit(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	job, err := h.crm.GetJobFit(c.Request.Context(), id)
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *crmHandler) scoreJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	m, err := h.crm.ScoreJob(c.Request.Context(), id)
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *crmHandler) applyToJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := h.crm.SaveApplication(c.Request.Context(), id, crm.AppApplied)
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusCreated, a)
}

func (h *crmHandler) analyzeResume(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := h.crm.AnalyzeResume(c.Request.Context(), id)
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *crmHandler) listApplications(c *gin.Context) {
	apps, err := h.crm.ListApplications(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"applications": apps})
}

func (h *crmHandler) updateApplication(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	status := crm.ApplicationStatus(body.Status)
	if !status.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	if err := h.crm.UpdateApplicationStatus(c.Request.Context(), id, status); err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *crmHandler) skillGaps(c *gin.Context) {
	a, err := h.crm.LatestSkillGaps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *crmHandler) analyzeSkills(c *gin.Context) {
	a, err := h.crm.AnalyzeSkillGaps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *crmHandler) outreach(c *gin.Context) {
	contacts, err := h.crm.ListContacts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	brief, _ := h.crm.GetDailyBrief(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"contacts": contacts, "daily_targets": brief.OutreachTargets})
}

func (h *crmHandler) generateOutreach(c *gin.Context) {
	contactID, err := uuid.Parse(c.Param("contactId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contact id"})
		return
	}
	jobID, err := uuid.Parse(c.Param("jobId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	msg, err := h.crm.GenerateOutreach(c.Request.Context(), contactID, jobID)
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (h *crmHandler) completeOutreach(c *gin.Context) {
	if err := h.crm.CompleteOutreach(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *crmHandler) weeklyReport(c *gin.Context) {
	r, err := h.crm.GetWeeklyReport(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h *crmHandler) generateWeekly(c *gin.Context) {
	r, err := h.crm.GenerateWeeklyReport(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h *crmHandler) coach(c *gin.Context) {
	out, err := h.crm.Coach(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *crmHandler) getProfile(c *gin.Context) {
	p, err := h.crm.GetProfile(c.Request.Context())
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *crmHandler) updateProfile(c *gin.Context) {
	var p crm.UserProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	existing, err := h.crm.GetProfile(c.Request.Context())
	if err != nil {
		writeCRMError(c, err)
		return
	}
	p.ID = existing.ID
	if err := h.crm.UpdateProfile(c.Request.Context(), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *crmHandler) appAnalytics(c *gin.Context) {
	a, err := h.crm.ApplicationAnalytics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, a)
}

func writeCRMError(c *gin.Context, err error) {
	if errors.Is(err, crm.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func (h *crmHandler) marketTrends(c *gin.Context) {
	snap, err := h.crm.GetMarketTrends(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap)
}

func (h *crmHandler) interviewReadiness(c *gin.Context) {
	readiness, err := h.crm.GetInterviewReadiness(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"companies": readiness})
}

func (h *crmHandler) offerPredictions(c *gin.Context) {
	pred, err := h.crm.GetOfferPrediction(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pred)
}

func (h *crmHandler) userSkills(c *gin.Context) {
	skills, err := h.crm.GetUserSkills(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills})
}
