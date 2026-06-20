package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTaskNoteNotFound   = errors.New("task note not found")
	ErrTaskNoteTitleEmpty = errors.New("task note title is required")
	ErrInvalidNoteType    = errors.New("invalid note type")
)

// NoteType classifies structured notes attached to tasks.
type NoteType string

const (
	NoteTypeGeneral  NoteType = "GENERAL_NOTE"
	NoteTypeLearning NoteType = "LEARNING_NOTE"
	NoteTypeReview   NoteType = "REVIEW_NOTE"
	NoteTypeDM       NoteType = "DM"
	NoteTypeJobApp    NoteType = "JOB_APPLICATION"
)

func AllNoteTypes() []NoteType {
	return []NoteType{
		NoteTypeGeneral,
		NoteTypeLearning,
		NoteTypeReview,
		NoteTypeDM,
		NoteTypeJobApp,
	}
}

func (t NoteType) IsValid() bool {
	switch t {
	case NoteTypeGeneral, NoteTypeLearning, NoteTypeReview, NoteTypeDM, NoteTypeJobApp:
		return true
	}
	return false
}

func (t NoteType) Label() string {
	switch t {
	case NoteTypeGeneral:
		return "General note"
	case NoteTypeLearning:
		return "Learning note"
	case NoteTypeReview:
		return "Review note"
	case NoteTypeDM:
		return "DM"
	case NoteTypeJobApp:
		return "Job application"
	default:
		return string(t)
	}
}

// ReplyStatus tracks outreach response state for DM notes.
type ReplyStatus string

const (
	ReplyStatusNotSent     ReplyStatus = "not_sent"
	ReplyStatusSent        ReplyStatus = "sent"
	ReplyStatusReplied     ReplyStatus = "replied"
	ReplyStatusFollowedUp  ReplyStatus = "followed_up"
	ReplyStatusNoResponse  ReplyStatus = "no_response"
)

func AllReplyStatuses() []ReplyStatus {
	return []ReplyStatus{
		ReplyStatusNotSent,
		ReplyStatusSent,
		ReplyStatusReplied,
		ReplyStatusFollowedUp,
		ReplyStatusNoResponse,
	}
}

func (s ReplyStatus) IsValid() bool {
	switch s {
	case ReplyStatusNotSent, ReplyStatusSent, ReplyStatusReplied, ReplyStatusFollowedUp, ReplyStatusNoResponse:
		return true
	}
	return false
}

func (s ReplyStatus) Label() string {
	switch s {
	case ReplyStatusNotSent:
		return "Not sent"
	case ReplyStatusSent:
		return "Sent"
	case ReplyStatusReplied:
		return "Replied"
	case ReplyStatusFollowedUp:
		return "Followed up"
	case ReplyStatusNoResponse:
		return "No response"
	default:
		return string(s)
	}
}

// ApplicationStatus tracks job application pipeline state.
type ApplicationStatus string

const (
	AppStatusSaved     ApplicationStatus = "saved"
	AppStatusApplied   ApplicationStatus = "applied"
	AppStatusInterview ApplicationStatus = "interview"
	AppStatusRejected  ApplicationStatus = "rejected"
	AppStatusOffer     ApplicationStatus = "offer"
	AppStatusGhosted   ApplicationStatus = "ghosted"
)

func AllApplicationStatuses() []ApplicationStatus {
	return []ApplicationStatus{
		AppStatusSaved,
		AppStatusApplied,
		AppStatusInterview,
		AppStatusRejected,
		AppStatusOffer,
		AppStatusGhosted,
	}
}

func (s ApplicationStatus) IsValid() bool {
	switch s {
	case AppStatusSaved, AppStatusApplied, AppStatusInterview, AppStatusRejected, AppStatusOffer, AppStatusGhosted:
		return true
	}
	return false
}

func (s ApplicationStatus) Label() string {
	switch s {
	case AppStatusSaved:
		return "Saved"
	case AppStatusApplied:
		return "Applied"
	case AppStatusInterview:
		return "Interview"
	case AppStatusRejected:
		return "Rejected"
	case AppStatusOffer:
		return "Offer"
	case AppStatusGhosted:
		return "Ghosted"
	default:
		return string(s)
	}
}

// ApplicationSource identifies where a job application came from.
type ApplicationSource string

const (
	SourceSupabase       ApplicationSource = "Supabase"
	SourceGitLab         ApplicationSource = "GitLab"
	SourceGrafana        ApplicationSource = "Grafana"
	SourceConfluent      ApplicationSource = "Confluent"
	SourceWeb3Career     ApplicationSource = "Web3Career"
	SourceCryptoJobsList ApplicationSource = "CryptoJobsList"
	SourceLinkedIn       ApplicationSource = "LinkedIn"
	SourceX              ApplicationSource = "X"
	SourceCompanyWebsite ApplicationSource = "Company Website"
	SourceOther          ApplicationSource = "Other"
)

func AllApplicationSources() []ApplicationSource {
	return []ApplicationSource{
		SourceSupabase,
		SourceGitLab,
		SourceGrafana,
		SourceConfluent,
		SourceWeb3Career,
		SourceCryptoJobsList,
		SourceLinkedIn,
		SourceX,
		SourceCompanyWebsite,
		SourceOther,
	}
}

func (s ApplicationSource) IsValid() bool {
	for _, v := range AllApplicationSources() {
		if v == s {
			return true
		}
	}
	return false
}

// TaskNote is a note attached to a task. Structured types (DM, JOB_APPLICATION)
// populate type-specific fields; simple types use Notes.
type TaskNote struct {
	ID        uuid.UUID
	TaskID    uuid.UUID
	NoteType  NoteType
	Title     string
	Content   string // legacy mirror of Notes for older rows
	CreatedAt time.Time
	UpdatedAt time.Time

	// DM fields
	PersonName     string
	Company        string
	RoleTitle      string
	Platform       string
	ProfileURL     string
	MessageContent string
	SentAt         *time.Time
	ReplyStatus    ReplyStatus
	ReplyAt        *time.Time

	// Job application fields
	JobTitle          string
	JobURL            string
	ApplicationStatus ApplicationStatus
	AppliedAt         *time.Time
	ResumeVersion     string
	FitScore          *int
	Source            ApplicationSource

	Notes string
	IsMarked bool
}

// TaskNoteWithTask enriches a note with its parent task title for list views.
type TaskNoteWithTask struct {
	TaskNote
	TaskTitle string
}

func (n *TaskNote) Validate() error {
	if !n.NoteType.IsValid() {
		return ErrInvalidNoteType
	}
	if strings.TrimSpace(n.Title) == "" {
		return ErrTaskNoteTitleEmpty
	}
	if n.NoteType == NoteTypeDM && n.ReplyStatus != "" && !n.ReplyStatus.IsValid() {
		return errors.New("invalid reply status")
	}
	if n.NoteType == NoteTypeJobApp {
		if n.ApplicationStatus != "" && !n.ApplicationStatus.IsValid() {
			return errors.New("invalid application status")
		}
		if n.Source != "" && !n.Source.IsValid() {
			return errors.New("invalid application source")
		}
	}
	return nil
}

// EffectiveNotes returns the free-form notes body, falling back to legacy content.
func (n *TaskNote) EffectiveNotes() string {
	if strings.TrimSpace(n.Notes) != "" {
		return n.Notes
	}
	return n.Content
}

// JobHuntSummary holds dashboard card counts.
type JobHuntSummary struct {
	TotalDMs           int
	TotalApplications  int
	DMTasks            int
	ApplicationTasks   int
}
