// Package model holds the core domain entities and value types for the
// job-hunt task tracker. This package has no infrastructure dependencies —
// it must remain importable from any layer (repository, service, api).
package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

// Sentinel errors returned by the service / repository layers. Handlers map
// these to HTTP status codes; repositories translate driver errors into them.
var (
	ErrTaskNotFound        = errors.New("task not found")
	ErrInvalidStatus       = errors.New("invalid status")
	ErrInvalidPriority     = errors.New("invalid priority")
	ErrInvalidCategory     = errors.New("invalid category")
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrTitleRequired        = errors.New("title is required")
	ErrEstimatedNegative    = errors.New("estimated_minutes cannot be negative")
	ErrActualNegative       = errors.New("actual_minutes cannot be negative")
	ErrTaskNotEligibleCarry = errors.New("task not eligible for carry-over")
)

// ---------------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------------

// Status is the lifecycle state of a task.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusMissed     Status = "missed"
)

// IsValid reports whether s is a known status value.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusInProgress, StatusCompleted, StatusMissed:
		return true
	}
	return false
}

// IsTerminal reports whether the status is a final state. Terminal tasks
// cannot be re-opened — instead a new (carried-over) task is created.
func (s Status) IsTerminal() bool {
	return s == StatusCompleted || s == StatusMissed
}

// CanTransitionTo encodes the allowed state machine.
//
//	pending     -> in_progress | completed | missed
//	in_progress -> pending     | completed | missed
//	completed   -> (terminal)
//	missed      -> (terminal)
func (s Status) CanTransitionTo(next Status) bool {
	if !next.IsValid() {
		return false
	}
	if s == next {
		return true // idempotent
	}
	switch s {
	case StatusPending:
		return next == StatusInProgress || next == StatusCompleted || next == StatusMissed
	case StatusInProgress:
		return next == StatusPending || next == StatusCompleted || next == StatusMissed
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Priority
// ---------------------------------------------------------------------------

// Priority indicates how urgent a task is. Higher = more urgent.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// IsValid reports whether p is a known priority value.
func (p Priority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// priorityRank maps each priority to an integer for comparison / boosting.
var priorityRank = map[Priority]int{
	PriorityLow:    0,
	PriorityMedium: 1,
	PriorityHigh:   2,
	PriorityUrgent: 3,
}

var rankPriority = []Priority{
	PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent,
}

// AllPriorities returns every supported priority in canonical order
// (low → urgent). Mirrors AllCategories so handlers/templates can iterate
// without hard-coding the set.
func AllPriorities() []Priority {
	return []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent}
}

// AllStatuses returns every supported task status in lifecycle order.
func AllStatuses() []Status {
	return []Status{StatusPending, StatusInProgress, StatusCompleted, StatusMissed}
}

// Bump returns the next-higher priority, capping at urgent. Used when an
// unfinished task is carried over to the following day.
func (p Priority) Bump() Priority {
	r, ok := priorityRank[p]
	if !ok {
		return PriorityMedium
	}
	if r >= len(rankPriority)-1 {
		return PriorityUrgent
	}
	return rankPriority[r+1]
}

// ---------------------------------------------------------------------------
// Category
// ---------------------------------------------------------------------------

// Category groups tasks by job-hunt activity type.
type Category string

const (
	CategoryJobApply          Category = "job_apply"
	CategoryRecruiterOutreach Category = "recruiter_outreach"
	CategoryGithub            Category = "github"
	CategoryTwitter           Category = "twitter"
	CategoryNetworking        Category = "networking"
	CategoryLearning          Category = "learning"
	CategoryInterview         Category = "interview"
	CategoryMisc              Category = "misc"
)

// AllCategories returns every supported category — useful for selects and tests.
func AllCategories() []Category {
	return []Category{
		CategoryJobApply,
		CategoryRecruiterOutreach,
		CategoryGithub,
		CategoryTwitter,
		CategoryNetworking,
		CategoryLearning,
		CategoryInterview,
		CategoryMisc,
	}
}

// IsValid reports whether c is a known category value.
func (c Category) IsValid() bool {
	for _, v := range AllCategories() {
		if v == c {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Task
// ---------------------------------------------------------------------------

// Task is the central domain entity. Fields map 1:1 to the `tasks` table.
type Task struct {
	ID               uuid.UUID
	Title            string
	Description      string
	Priority         Priority
	Category         Category
	Status           Status
	EstimatedMinutes int
	ActualMinutes    int
	DueDate          *time.Time
	CarryOverCount   int
	CompletedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Validate verifies the invariants a freshly-built or freshly-loaded Task
// must satisfy before being persisted.
func (t *Task) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return ErrTitleRequired
	}
	if !t.Priority.IsValid() {
		return ErrInvalidPriority
	}
	if !t.Category.IsValid() {
		return ErrInvalidCategory
	}
	if !t.Status.IsValid() {
		return ErrInvalidStatus
	}
	if t.EstimatedMinutes < 0 {
		return ErrEstimatedNegative
	}
	if t.ActualMinutes < 0 {
		return ErrActualNegative
	}
	return nil
}

// IsOverdue reports whether the task has a due date in the past AND is not
// in a terminal state. Used by the dashboard and by the carry-over job.
func (t *Task) IsOverdue(now time.Time) bool {
	if t.Status.IsTerminal() {
		return false
	}
	if t.DueDate == nil {
		return false
	}
	return t.DueDate.Before(now)
}

// IsCarriedOver returns true when the task has been rolled over at least once.
func (t *Task) IsCarriedOver() bool {
	return t.CarryOverCount > 0
}
