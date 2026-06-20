package web

import (
	"html/template"
	"strings"

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
				{viewUpcoming, "Next 30 days"},
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
			case viewUpcoming:
				return c.Upcoming
			case viewOverdue:
				return c.Overdue
			case viewCompleted:
				return c.Completed
			case viewCarried:
				return c.CarriedOver
			case viewAll:
				return c.All
			}
			return 0
		},
		"statusLabel":   func(s model.Status) string { return humanStatus(s) },
		"priorityLabel": func(p model.Priority) string { return humanPriority(p) },
		"categoryLabel": func(c model.Category) string { return humanCategory(c) },
		"noteSelectedID": func(d *TaskNoteDetailVM) string {
			if d == nil {
				return ""
			}
			return d.ID
		},
		"showNoteFields": showNoteFields,
	}
}

// showNoteFields reports whether the current note type uses a field group.
func showNoteFields(currentNoteType string, fieldSpec string) bool {
	current := strings.TrimSpace(currentNoteType)
	if current == "" {
		current = string(model.NoteTypeGeneral)
	}
	for _, part := range strings.Split(fieldSpec, ",") {
		if strings.TrimSpace(part) == current {
			return true
		}
	}
	return false
}
