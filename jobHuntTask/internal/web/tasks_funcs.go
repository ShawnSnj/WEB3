package web

import (
	"html/template"

	"github.com/shawn/jobhunttask/internal/model"
)

// ViewItem is a tab entry surfaced to templates by the `views` func.
// Defined at package level (not inside the func) so templates can name
// the field accessors.
type ViewItem struct {
	Key   tasksView
	Label string
}

func tasksFuncMap() template.FuncMap {
	return template.FuncMap{
		"views": func() []ViewItem {
			return []ViewItem{
				{viewToday, "Today"},
				{viewOverdue, "Overdue"},
				{viewCompleted, "Completed"},
				{viewCarried, "Carried over"},
				{viewAll, "All"},
			}
		},
		"viewCount": func(v tasksView, c TasksCountsVM) int {
			switch v {
			case viewToday:
				return c.Today
			case viewOverdue:
				return c.Overdue
			case viewCompleted:
				return c.Completed
			case viewCarried:
				return c.CarriedOver
			}
			return 0
		},
		"statusLabel":   func(s model.Status) string { return humanStatus(s) },
		"priorityLabel": func(p model.Priority) string { return humanPriority(p) },
		"categoryLabel": func(c model.Category) string { return humanCategory(c) },
	}
}
