package web

import (
	_ "embed"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed static/import/daily_tasks_template.csv
var dailyTasksTemplateCSV []byte

type TasksImportFormVM struct {
	DefaultDue string // YYYY-MM-DD
}

type ImportRowErrorVM struct {
	Line    int
	Title   string
	Message string
}

type TasksImportResultVM struct {
	Created int
	Skipped int
	Errors  []ImportRowErrorVM
}

func (h *TasksHandler) importForm(c *gin.Context) {
	h.rd.RenderPartial(c, "tasks_import_form", TasksImportFormVM{
		DefaultDue: h.cal.FormatDate(h.clock.Now()),
	})
}

func (h *TasksHandler) importTemplate(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="daily_tasks_template.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", dailyTasksTemplateCSV)
}

func (h *TasksHandler) importCSV(c *gin.Context) {
	reader, errMsg := h.importReader(c)
	if errMsg != "" {
		c.Status(http.StatusUnprocessableEntity)
		h.rd.RenderPartial(c, "tasks_import_result", TasksImportResultVM{
			Skipped: 1,
			Errors:  []ImportRowErrorVM{{Line: 0, Message: errMsg}},
		})
		return
	}

	defaultDue := h.clock.Now()
	if d, ok := h.cal.ParseDate(c.PostForm("default_due")); ok {
		defaultDue = d
	}

	res, err := h.tasks.ImportFromCSV(c.Request.Context(), reader, defaultDue)
	if err != nil {
		c.Status(http.StatusUnprocessableEntity)
		h.rd.RenderPartial(c, "tasks_import_result", TasksImportResultVM{
			Skipped: 1,
			Errors:  []ImportRowErrorVM{{Line: 0, Message: err.Error()}},
		})
		return
	}

	vm := TasksImportResultVM{Created: res.Created, Skipped: res.Skipped}
	for _, e := range res.Errors {
		vm.Errors = append(vm.Errors, ImportRowErrorVM{
			Line: e.Line, Title: e.Title, Message: e.Message,
		})
	}

	if res.Created > 0 {
		setToast(c, "success", "Imported "+itoa(res.Created)+" task(s)")
		h.triggerTasksChanged(c)
	} else if res.Skipped > 0 {
		setToast(c, "warning", "No tasks imported — check the errors below")
	}

	c.Status(http.StatusOK)
	h.rd.RenderPartial(c, "tasks_import_result", vm)
}

func (h *TasksHandler) importReader(c *gin.Context) (io.Reader, string) {
	if err := c.Request.ParseMultipartForm(4 << 20); err != nil && c.Request.MultipartForm == nil {
		// Fall through — may be urlencoded paste-only form.
	}
	if c.Request.MultipartForm != nil {
		if files := c.Request.MultipartForm.File["file"]; len(files) > 0 && files[0].Size > 0 {
			f, err := files[0].Open()
			if err != nil {
				return nil, "could not read uploaded file"
			}
			return f, ""
		}
	}
	csvText := strings.TrimSpace(c.PostForm("csv"))
	if csvText == "" {
		return nil, "paste CSV or choose a file to import"
	}
	return strings.NewReader(csvText), ""
}
