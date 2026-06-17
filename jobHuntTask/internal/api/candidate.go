package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/engine/resume"
)

const maxResumeBytes = 512 * 1024

func (h *crmHandler) getCandidateProfile(c *gin.Context) {
	out, err := h.crm.GetCandidateProfile(c.Request.Context())
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *crmHandler) updateCandidateProfile(c *gin.Context) {
	var patch crm.CandidateProfile
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.crm.UpdateCandidateProfile(c.Request.Context(), &patch)
	if err != nil {
		writeCRMError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *crmHandler) uploadResume(c *gin.Context) {
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		h.uploadResumeMultipart(c)
		return
	}
	h.uploadResumeJSON(c)
}

func (h *crmHandler) uploadResumeMultipart(c *gin.Context) {
	lang := crm.ResumeLanguage(strings.ToLower(c.PostForm("language")))
	if !lang.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "language must be en or zh"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file — choose a .doc, .docx, .pdf, or .txt file"})
		return
	}
	defer file.Close()

	data, err := resume.ReadFile(file, maxResumeBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filename := header.Filename
	if filename == "" {
		filename = string(lang) + "-resume"
	}
	contentType := header.Header.Get("Content-Type")

	doc, err := h.crm.UploadResumeFile(c.Request.Context(), lang, filename, data, contentType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *crmHandler) uploadResumeJSON(c *gin.Context) {
	var body struct {
		Language string `json:"language"`
		Text     string `json:"text"`
		Filename string `json:"filename"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lang := crm.ResumeLanguage(strings.ToLower(body.Language))
	if !lang.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "language must be en or zh"})
		return
	}
	filename := body.Filename
	if filename == "" {
		filename = string(lang) + "-resume.txt"
	}
	doc, err := h.crm.UploadResume(c.Request.Context(), lang, filename, body.Text, "text/plain")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *crmHandler) parseResumes(c *gin.Context) {
	out, err := h.crm.ParseResumes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
