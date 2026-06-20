package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// CreateTaskNoteInput is the service DTO for creating a note.
type CreateTaskNoteInput struct {
	TaskID  uuid.UUID
	NoteType model.NoteType
	Title   string
	Content string
	Notes   string

	PersonName     string
	Company        string
	RoleTitle      string
	Platform       string
	ProfileURL     string
	MessageContent string
	SentAt         *time.Time
	ReplyStatus    model.ReplyStatus
	ReplyAt        *time.Time

	JobTitle          string
	JobURL            string
	ApplicationStatus model.ApplicationStatus
	AppliedAt         *time.Time
	ResumeVersion     string
	FitScore          *int
	Source            model.ApplicationSource
}

// UpdateTaskNoteInput is the service DTO for updating a note.
type UpdateTaskNoteInput struct {
	NoteType          *model.NoteType
	Title             *string
	Content           *string
	Notes             *string
	PersonName        *string
	Company           *string
	RoleTitle         *string
	Platform          *string
	ProfileURL        *string
	MessageContent    *string
	SentAt            *time.Time
	ReplyStatus       *model.ReplyStatus
	ReplyAt           *time.Time
	JobTitle          *string
	JobURL            *string
	ApplicationStatus *model.ApplicationStatus
	AppliedAt         *time.Time
	ResumeVersion     *string
	FitScore          *int
	Source            *model.ApplicationSource
}

// TaskNoteService manages notes attached to tasks.
type TaskNoteService struct {
	notes repository.TaskNoteRepository
	tasks repository.TaskRepository
}

func NewTaskNoteService(notes repository.TaskNoteRepository, tasks repository.TaskRepository) *TaskNoteService {
	return &TaskNoteService{notes: notes, tasks: tasks}
}

func (s *TaskNoteService) Create(ctx context.Context, in CreateTaskNoteInput) (*model.TaskNote, error) {
	if _, err := s.tasks.GetByID(ctx, in.TaskID); err != nil {
		return nil, err
	}
	n := buildTaskNoteFromInput(in)
	if err := n.Validate(); err != nil {
		return nil, err
	}
	if err := s.notes.Create(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *TaskNoteService) Get(ctx context.Context, id uuid.UUID) (*model.TaskNote, error) {
	return s.notes.GetByID(ctx, id)
}

func (s *TaskNoteService) GetWithTask(ctx context.Context, id uuid.UUID) (*model.TaskNoteWithTask, error) {
	n, err := s.notes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t, err := s.tasks.GetByID(ctx, n.TaskID)
	if err != nil {
		return nil, err
	}
	return &model.TaskNoteWithTask{TaskNote: *n, TaskTitle: t.Title}, nil
}

func (s *TaskNoteService) Update(ctx context.Context, id uuid.UUID, in UpdateTaskNoteInput) (*model.TaskNote, error) {
	if in.Notes != nil {
		v := strings.TrimSpace(*in.Notes)
		in.Content = &v
	}
	upd := repository.TaskNoteUpdate{
		NoteType:          in.NoteType,
		Title:             in.Title,
		Content:           in.Content,
		Notes:             in.Notes,
		PersonName:        in.PersonName,
		Company:           in.Company,
		RoleTitle:         in.RoleTitle,
		Platform:          in.Platform,
		ProfileURL:        in.ProfileURL,
		MessageContent:    in.MessageContent,
		SentAt:            in.SentAt,
		ReplyStatus:       in.ReplyStatus,
		ReplyAt:           in.ReplyAt,
		JobTitle:          in.JobTitle,
		JobURL:            in.JobURL,
		ApplicationStatus: in.ApplicationStatus,
		AppliedAt:         in.AppliedAt,
		ResumeVersion:     in.ResumeVersion,
		FitScore:          in.FitScore,
		Source:            in.Source,
	}
	return s.notes.Update(ctx, id, upd)
}

func (s *TaskNoteService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.notes.Delete(ctx, id)
}

func (s *TaskNoteService) ListByTask(ctx context.Context, taskID uuid.UUID) ([]*model.TaskNote, error) {
	if _, err := s.tasks.GetByID(ctx, taskID); err != nil {
		return nil, err
	}
	return s.notes.ListByTaskID(ctx, taskID)
}

func (s *TaskNoteService) JobHuntSummary(ctx context.Context) (model.JobHuntSummary, error) {
	dms, err := s.notes.CountByNoteType(ctx, model.NoteTypeDM)
	if err != nil {
		return model.JobHuntSummary{}, err
	}
	apps, err := s.notes.CountByNoteType(ctx, model.NoteTypeJobApp)
	if err != nil {
		return model.JobHuntSummary{}, err
	}
	dmTasks, err := s.notes.CountOutreachTasks(ctx)
	if err != nil {
		return model.JobHuntSummary{}, err
	}
	appTasks, err := s.notes.CountApplicationTasks(ctx)
	if err != nil {
		return model.JobHuntSummary{}, err
	}
	return model.JobHuntSummary{
		TotalDMs:          dms,
		TotalApplications: apps,
		DMTasks:           dmTasks,
		ApplicationTasks:  appTasks,
	}, nil
}

func (s *TaskNoteService) ListDMs(ctx context.Context) ([]*model.TaskNoteWithTask, error) {
	return s.notes.ListByNoteType(ctx, model.NoteTypeDM, 500)
}

func (s *TaskNoteService) ListApplications(ctx context.Context) ([]*model.TaskNoteWithTask, error) {
	return s.notes.ListByNoteType(ctx, model.NoteTypeJobApp, 500)
}

func (s *TaskNoteService) ListOutreachTasks(ctx context.Context) ([]*model.Task, error) {
	return s.notes.ListOutreachTasks(ctx, 500)
}

func (s *TaskNoteService) ListApplicationTasks(ctx context.Context) ([]*model.Task, error) {
	return s.notes.ListApplicationTasks(ctx, 500)
}

func (s *TaskNoteService) GetMarked(ctx context.Context) (*model.TaskNoteWithTask, error) {
	return s.notes.GetMarkedWithTask(ctx)
}

func (s *TaskNoteService) SetMarked(ctx context.Context, id uuid.UUID, marked bool) error {
	if _, err := s.notes.GetByID(ctx, id); err != nil {
		return err
	}
	return s.notes.SetMarked(ctx, id, marked)
}

func (s *TaskNoteService) ToggleMarked(ctx context.Context, id uuid.UUID) (bool, error) {
	n, err := s.notes.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	next := !n.IsMarked
	if err := s.notes.SetMarked(ctx, id, next); err != nil {
		return false, err
	}
	return next, nil
}

func buildTaskNoteFromInput(in CreateTaskNoteInput) *model.TaskNote {
	noteType := in.NoteType
	if !noteType.IsValid() {
		noteType = model.NoteTypeGeneral
	}
	notes := strings.TrimSpace(in.Notes)
	if notes == "" {
		notes = strings.TrimSpace(in.Content)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = defaultStructuredNoteTitle(noteType, in)
	}
	content := notes
	return &model.TaskNote{
		TaskID:            in.TaskID,
		NoteType:          noteType,
		Title:             title,
		Content:           content,
		Notes:             notes,
		PersonName:        strings.TrimSpace(in.PersonName),
		Company:           strings.TrimSpace(in.Company),
		RoleTitle:         strings.TrimSpace(in.RoleTitle),
		Platform:          strings.TrimSpace(in.Platform),
		ProfileURL:        strings.TrimSpace(in.ProfileURL),
		MessageContent:    strings.TrimSpace(in.MessageContent),
		SentAt:            in.SentAt,
		ReplyStatus:       in.ReplyStatus,
		ReplyAt:           in.ReplyAt,
		JobTitle:          strings.TrimSpace(in.JobTitle),
		JobURL:            strings.TrimSpace(in.JobURL),
		ApplicationStatus: in.ApplicationStatus,
		AppliedAt:         in.AppliedAt,
		ResumeVersion:     strings.TrimSpace(in.ResumeVersion),
		FitScore:          in.FitScore,
		Source:            in.Source,
	}
}

func defaultStructuredNoteTitle(noteType model.NoteType, in CreateTaskNoteInput) string {
	switch noteType {
	case model.NoteTypeDM:
		parts := make([]string, 0, 3)
		if p := strings.TrimSpace(in.PersonName); p != "" {
			parts = append(parts, p)
		}
		if c := strings.TrimSpace(in.Company); c != "" {
			parts = append(parts, c)
		}
		if len(parts) > 0 {
			return "DM: " + strings.Join(parts, " @ ")
		}
		return "DM"
	case model.NoteTypeJobApp:
		parts := make([]string, 0, 2)
		if c := strings.TrimSpace(in.Company); c != "" {
			parts = append(parts, c)
		}
		if j := strings.TrimSpace(in.JobTitle); j != "" {
			parts = append(parts, j)
		}
		if len(parts) > 0 {
			return "Application: " + strings.Join(parts, " — ")
		}
		return "Job application"
	case model.NoteTypeLearning:
		return "Learning note"
	case model.NoteTypeReview:
		return "Review note"
	default:
		return defaultNoteTitle(in.Notes)
	}
}

func defaultNoteTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "Untitled note"
	}
	if idx := strings.IndexAny(content, "\n\r"); idx > 0 {
		content = content[:idx]
	}
	if len(content) > 60 {
		content = content[:60]
	}
	return content
}

// ParseTaskNoteForm builds create input from form values.
func ParseTaskNoteForm(taskID uuid.UUID, form map[string][]string) (CreateTaskNoteInput, error) {
	get := func(k string) string {
		if v, ok := form[k]; ok && len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
		return ""
	}
	noteType := model.NoteType(get("note_type"))
	if !noteType.IsValid() {
		noteType = model.NoteTypeGeneral
	}
	in := CreateTaskNoteInput{
		TaskID:            taskID,
		NoteType:          noteType,
		Title:             get("title"),
		Content:           get("content"),
		PersonName:        get("person_name"),
		Company:           get("company"),
		RoleTitle:         get("role_title"),
		Platform:          get("platform"),
		ProfileURL:        get("profile_url"),
		MessageContent:    get("message_content"),
		ReplyStatus:       model.ReplyStatus(get("reply_status")),
		JobTitle:          get("job_title"),
		JobURL:            get("job_url"),
		ApplicationStatus: model.ApplicationStatus(get("application_status")),
		ResumeVersion:     get("resume_version"),
		Source:            model.ApplicationSource(get("source")),
	}
	if v := formNotesValue(form); v != nil {
		in.Notes = *v
	}
	if v := get("fit_score"); v != "" {
		var score int
		if _, err := fmt.Sscanf(v, "%d", &score); err == nil {
			in.FitScore = &score
		}
	}
	if v := get("sent_at"); v != "" {
		if t, err := time.Parse("2006-01-02T15:04", v); err == nil {
			in.SentAt = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			in.SentAt = &t
		}
	}
	if v := get("reply_at"); v != "" {
		if t, err := time.Parse("2006-01-02T15:04", v); err == nil {
			in.ReplyAt = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			in.ReplyAt = &t
		}
	}
	if v := get("applied_at"); v != "" {
		if t, err := time.Parse("2006-01-02T15:04", v); err == nil {
			in.AppliedAt = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			in.AppliedAt = &t
		}
	}
	return in, nil
}

// ParseTaskNoteUpdateForm builds update input from form values.
func ParseTaskNoteUpdateForm(form map[string][]string) UpdateTaskNoteInput {
	get := func(k string) string {
		if v, ok := form[k]; ok && len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
		return ""
	}
	in := UpdateTaskNoteInput{}
	if v := get("note_type"); v != "" {
		nt := model.NoteType(v)
		in.NoteType = &nt
	}
	if _, ok := form["title"]; ok {
		v := get("title")
		in.Title = &v
	}
	if _, ok := form["content"]; ok {
		v := get("content")
		in.Content = &v
	}
	if _, ok := form["notes"]; ok {
		if v := formNotesValue(form); v != nil {
			in.Notes = v
		}
	}
	setStr := func(k string, dst **string) {
		if _, ok := form[k]; ok {
			v := get(k)
			*dst = &v
		}
	}
	setStr("person_name", &in.PersonName)
	setStr("company", &in.Company)
	setStr("role_title", &in.RoleTitle)
	setStr("platform", &in.Platform)
	setStr("profile_url", &in.ProfileURL)
	setStr("message_content", &in.MessageContent)
	setStr("job_title", &in.JobTitle)
	setStr("job_url", &in.JobURL)
	setStr("resume_version", &in.ResumeVersion)
	if v := get("reply_status"); v != "" {
		rs := model.ReplyStatus(v)
		in.ReplyStatus = &rs
	}
	if v := get("application_status"); v != "" {
		as := model.ApplicationStatus(v)
		in.ApplicationStatus = &as
	}
	if v := get("source"); v != "" {
		src := model.ApplicationSource(v)
		in.Source = &src
	}
	if v := get("fit_score"); v != "" {
		var score int
		if _, err := fmt.Sscanf(v, "%d", &score); err == nil {
			in.FitScore = &score
		}
	}
	if v := get("sent_at"); v != "" {
		if t, err := time.Parse("2006-01-02T15:04", v); err == nil {
			in.SentAt = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			in.SentAt = &t
		}
	}
	if v := get("reply_at"); v != "" {
		if t, err := time.Parse("2006-01-02T15:04", v); err == nil {
			in.ReplyAt = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			in.ReplyAt = &t
		}
	}
	if v := get("applied_at"); v != "" {
		if t, err := time.Parse("2006-01-02T15:04", v); err == nil {
			in.AppliedAt = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			in.AppliedAt = &t
		}
	}
	return in
}

func formNotesValue(form map[string][]string) *string {
	vals, ok := form["notes"]
	if !ok || len(vals) == 0 {
		return nil
	}
	// Prefer the last non-empty value (avoids duplicate name="notes" fields).
	chosen := strings.TrimSpace(vals[len(vals)-1])
	for i := len(vals) - 1; i >= 0; i-- {
		if v := strings.TrimSpace(vals[i]); v != "" {
			chosen = v
			break
		}
	}
	v := chosen
	return &v
}

// ApplyNoteFormTitle sets Title on an update from note body fields.
func ApplyNoteFormTitle(in CreateTaskNoteInput, upd *UpdateTaskNoteInput) {
	title := defaultStructuredNoteTitle(in.NoteType, in)
	upd.Title = &title
}

// ApplyNoteFormTitleIfEmpty auto-titles only when the form left title blank.
func ApplyNoteFormTitleIfEmpty(in CreateTaskNoteInput, upd *UpdateTaskNoteInput) {
	if upd.Title != nil && strings.TrimSpace(*upd.Title) != "" {
		return
	}
	ApplyNoteFormTitle(in, upd)
}
