package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/shawn/jobhunttask/internal/model"
)

// ImportRowError describes one CSV row that could not be imported.
type ImportRowError struct {
	Line    int
	Title   string
	Message string
}

// ImportResult summarises a bulk CSV import.
type ImportResult struct {
	Created int
	Skipped int
	Errors  []ImportRowError
}

// ImportFromCSV parses CSV task rows and creates pending tasks.
//
// Expected columns (header row recommended, case-insensitive):
//
//	title, description, category, priority, estimated_minutes, due_date
//
// Extra columns such as task_id are ignored. Blank due_date defaults to
// defaultDue (typically today for daily plans).
func (s *TaskService) ImportFromCSV(ctx context.Context, r io.Reader, defaultDue time.Time) (ImportResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return ImportResult{}, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) == 0 {
		return ImportResult{}, fmt.Errorf("csv is empty")
	}

	start := 0
	col := map[string]int{}
	if looksLikeHeader(records[0]) {
		col = buildImportColumnMap(records[0])
		start = 1
	} else {
		col = defaultImportColumns()
	}

	var out ImportResult
	for i := start; i < len(records); i++ {
		line := i + 1
		row := records[i]
		if isBlankImportRow(row) {
			continue
		}
		if isHeaderLikeDataRow(row, col) {
			continue
		}

		in, rowErr := s.parseImportRow(row, col, defaultDue)
		if rowErr != "" {
			out.Skipped++
			out.Errors = append(out.Errors, ImportRowError{
				Line: line, Title: displayImportTitle(row, col), Message: rowErr,
			})
			continue
		}

		if in.DueDate != nil {
			exists, err := s.planExists(ctx, in.Title, *in.DueDate)
			if err != nil {
				return out, err
			}
			if exists {
				out.Skipped++
				out.Errors = append(out.Errors, ImportRowError{
					Line: line, Title: in.Title,
					Message: "already exists for this due date",
				})
				continue
			}
		}

		if _, err := s.Create(ctx, in); err != nil {
			out.Skipped++
			out.Errors = append(out.Errors, ImportRowError{
				Line: line, Title: in.Title, Message: humanImportErr(err),
			})
			continue
		}
		out.Created++
	}
	return out, nil
}

func buildImportColumnMap(header []string) map[string]int {
	col := map[string]int{}
	for i, h := range header {
		key := normalizeImportHeader(h)
		if key == "" || key == "ignore" {
			continue
		}
		// First occurrence wins — avoids duplicate header cells clobbering.
		if _, exists := col[key]; !exists {
			col[key] = i
		}
	}
	return col
}

func looksLikeHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	known := 0
	for _, c := range row {
		switch normalizeImportHeader(c) {
		case "title", "task_id", "description", "category", "priority", "estimated_minutes", "due_date":
			known++
		}
	}
	return known >= 2
}

func defaultImportColumns() map[string]int {
	return map[string]int{
		"title": 0, "description": 1, "category": 2,
		"priority": 3, "estimated_minutes": 4, "due_date": 5,
	}
}

func normalizeImportHeader(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\ufeff") // Excel UTF-8 BOM
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	switch s {
	case "task", "name", "task_name", "task_title", "tasktitle":
		return "title"
	case "task_id", "id", "code", "ref", "reference":
		return "task_id"
	case "desc", "details", "notes":
		return "description"
	case "mins", "minutes", "minute", "estimate", "estimated", "est_minutes",
		"est_min", "est", "time_estimate", "time", "duration", "estimated_time":
		return "estimated_minutes"
	case "due", "date", "due_on", "deadline":
		return "due_date"
	case "cat", "type":
		return "category"
	case "prio", "importance":
		return "priority"
	default:
		return s
	}
}

func (s *TaskService) parseImportRow(row []string, col map[string]int, defaultDue time.Time) (CreateTaskInput, string) {
	title := strings.TrimSpace(cell(row, col, "title"))
	if title == "" {
		// Some sheets use task_id as the visible label when title is empty.
		title = strings.TrimSpace(cell(row, col, "task_id"))
	}
	if title == "" {
		return CreateTaskInput{}, "title is required"
	}

	in := CreateTaskInput{
		Title:       title,
		Description: strings.TrimSpace(cell(row, col, "description")),
		Priority:    model.Priority(strings.ToLower(strings.TrimSpace(cell(row, col, "priority")))),
		Category:    resolveImportCategory(cell(row, col, "category")),
	}

	if em := strings.TrimSpace(cell(row, col, "estimated_minutes")); em != "" {
		n, ok := parseImportMinutes(em)
		if !ok {
			return CreateTaskInput{}, fmt.Sprintf("estimated_minutes %q is not a valid number", em)
		}
		in.EstimatedMinutes = n
	}

	dueRaw := strings.TrimSpace(cell(row, col, "due_date"))
	if dueRaw == "" {
		d := s.cal.StartOfDay(defaultDue)
		in.DueDate = &d
	} else {
		d, ok := s.parseImportDate(dueRaw)
		if !ok {
			return CreateTaskInput{}, "due_date must be YYYY-MM-DD"
		}
		in.DueDate = &d
	}
	return in, ""
}

func parseImportMinutes(s string) (int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\ufeff")
	if s == "" {
		return 0, true
	}

	lower := strings.ToLower(s)
	for _, suffix := range []string{" minutes", " minute", " mins", " min", "m"} {
		if strings.HasSuffix(lower, suffix) {
			s = strings.TrimSpace(s[:len(s)-len(suffix)])
			lower = strings.ToLower(s)
		}
	}

	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)

	// Plain integer path (most common).
	if n, err := strconv.Atoi(s); err == nil {
		return n, n >= 0
	}

	// Excel / Sheets often export 45.0
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f < 0 {
			return 0, false
		}
		return int(f + 0.5), true
	}

	// Last resort: strip non-numeric cruft but keep digits and one dot.
	var b strings.Builder
	seenDot := false
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == '.' && !seenDot {
			seenDot = true
			b.WriteRune(r)
		}
	}
	clean := b.String()
	if clean == "" {
		return 0, false
	}
	if f, err := strconv.ParseFloat(clean, 64); err == nil && f >= 0 {
		return int(f + 0.5), true
	}
	return 0, false
}

// resolveImportCategory maps CSV category labels to the app's fixed category set.
// Known aliases (e.g. "application" → job_apply) are normalized; unrecognized
// labels default to misc so imports don't fail on spreadsheet vocabulary.
func resolveImportCategory(raw string) model.Category {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	if s == "" {
		return model.CategoryMisc
	}
	if cat := model.Category(s); cat.IsValid() {
		return cat
	}
	aliases := map[string]model.Category{
		// Job applications
		"application": model.CategoryJobApply, "applications": model.CategoryJobApply,
		"apply": model.CategoryJobApply, "job_application": model.CategoryJobApply,
		"job_applications": model.CategoryJobApply, "job_apply": model.CategoryJobApply,
		// Outreach / DMs
		"outreach": model.CategoryRecruiterOutreach, "recruiter": model.CategoryRecruiterOutreach,
		"recruiter_outreach": model.CategoryRecruiterOutreach, "dm": model.CategoryRecruiterOutreach,
		"cold_outreach": model.CategoryRecruiterOutreach,
		// GitHub / portfolio
		"portfolio": model.CategoryGithub, "github_profile": model.CategoryGithub,
		"readme": model.CategoryGithub, "code": model.CategoryGithub,
		// Twitter / visibility
		"visibility": model.CategoryTwitter, "social": model.CategoryTwitter,
		"social_media": model.CategoryTwitter, "twitter": model.CategoryTwitter,
		"content": model.CategoryTwitter, "post": model.CategoryTwitter,
		// Networking / branding
		"branding": model.CategoryNetworking, "linkedin": model.CategoryNetworking,
		"network": model.CategoryNetworking, "networking": model.CategoryNetworking,
		"personal_brand": model.CategoryNetworking,
		// Learning / prep
		"learning": model.CategoryLearning, "study": model.CategoryLearning,
		"practice": model.CategoryLearning, "prep": model.CategoryLearning,
		"interview_prep": model.CategoryInterview,
		// Interview (extra aliases)
		"mock_interview": model.CategoryInterview, "system_design": model.CategoryInterview,
		// General / planning
		"career": model.CategoryMisc, "resume": model.CategoryMisc,
		"research": model.CategoryMisc, "review": model.CategoryMisc,
		"planning": model.CategoryMisc, "tracker": model.CategoryMisc,
		"metrics": model.CategoryMisc, "general": model.CategoryMisc,
	}
	if mapped, ok := aliases[s]; ok {
		return mapped
	}
	return model.CategoryMisc
}

func cell(row []string, col map[string]int, name string) string {
	i, ok := col[name]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}

func displayImportTitle(row []string, col map[string]int) string {
	if t := strings.TrimSpace(cell(row, col, "title")); t != "" {
		return t
	}
	return strings.TrimSpace(cell(row, col, "task_id"))
}

func isHeaderLikeDataRow(row []string, col map[string]int) bool {
	title := normalizeImportHeader(displayImportTitle(row, col))
	if title == "title" || title == "task_id" {
		return true
	}
	if em := normalizeImportHeader(cell(row, col, "estimated_minutes")); em == "estimated_minutes" {
		return true
	}
	return false
}

func isBlankImportRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func (s *TaskService) parseImportDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if d, ok := s.cal.ParseDate(raw); ok {
		return d, true
	}
	if d, ok := s.cal.ParseDateSlash(raw); ok {
		return d, true
	}
	return time.Time{}, false
}

func humanImportErr(err error) string {
	switch {
	case err == model.ErrTitleRequired:
		return "title is required"
	case err == model.ErrInvalidPriority:
		return "invalid priority"
	case err == model.ErrInvalidCategory:
		return "invalid category"
	case err == model.ErrEstimatedNegative:
		return "estimated_minutes cannot be negative"
	default:
		return "could not create task"
	}
}
