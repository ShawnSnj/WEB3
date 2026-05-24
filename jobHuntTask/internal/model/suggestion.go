package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Kind: which rule produced the suggestion. Kept in sync with the DB CHECK.
// ---------------------------------------------------------------------------

type SuggestionKind string

const (
	SuggestionReduceWorkload SuggestionKind = "reduce_workload"
	SuggestionSmallerTasks   SuggestionKind = "smaller_tasks"
	SuggestionEasierWins     SuggestionKind = "easier_wins"
	SuggestionFocusShift     SuggestionKind = "focus_shift"
)

func (k SuggestionKind) IsValid() bool {
	switch k {
	case SuggestionReduceWorkload,
		SuggestionSmallerTasks,
		SuggestionEasierWins,
		SuggestionFocusShift:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Severity: how much weight the UI should give it.
// ---------------------------------------------------------------------------

type SuggestionSeverity string

const (
	SeverityInfo     SuggestionSeverity = "info"
	SeverityWarning  SuggestionSeverity = "warning"
	SeverityCritical SuggestionSeverity = "critical"
)

func (s SuggestionSeverity) IsValid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Status lifecycle: active -> dismissed | expired
// ---------------------------------------------------------------------------

type SuggestionStatus string

const (
	SuggestionStatusActive    SuggestionStatus = "active"
	SuggestionStatusDismissed SuggestionStatus = "dismissed"
	SuggestionStatusExpired   SuggestionStatus = "expired"
)

func (s SuggestionStatus) IsValid() bool {
	switch s {
	case SuggestionStatusActive,
		SuggestionStatusDismissed,
		SuggestionStatusExpired:
		return true
	}
	return false
}

// CanTransitionTo enforces the simple state machine. Only active rows may
// transition; dismissed/expired are terminal.
func (s SuggestionStatus) CanTransitionTo(next SuggestionStatus) bool {
	if s != SuggestionStatusActive {
		return false
	}
	return next == SuggestionStatusDismissed || next == SuggestionStatusExpired
}

// ---------------------------------------------------------------------------
// Suggestion entity
// ---------------------------------------------------------------------------

// Suggestion is one recommendation surfaced to the user by a rule.
//
// Payload carries rule-specific context (the numbers that justified the
// recommendation) — kept as a free-form map so each rule can attach what
// it likes without schema churn. It's serialized as JSONB.
type Suggestion struct {
	ID           uuid.UUID
	Kind         SuggestionKind
	Severity     SuggestionSeverity
	Status       SuggestionStatus
	Title        string
	Message      string
	Payload      map[string]any
	DedupKey     string
	GeneratedAt  time.Time
	ExpiresAt    *time.Time
	DismissedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s *Suggestion) Validate() error {
	if !s.Kind.IsValid() {
		return ErrInvalidSuggestionKind
	}
	if !s.Severity.IsValid() {
		return ErrInvalidSuggestionSeverity
	}
	if !s.Status.IsValid() {
		return ErrInvalidSuggestionStatus
	}
	if s.Title == "" {
		return ErrSuggestionTitleEmpty
	}
	if s.Message == "" {
		return ErrSuggestionMessageEmpty
	}
	if s.DedupKey == "" {
		return ErrSuggestionDedupKeyEmpty
	}
	return nil
}

// DedupKeyForWeek returns the canonical dedup-key for a (kind, week)
// pairing. ISO-year + ISO-week is used so week boundaries are independent
// of timezone DST shifts.
func DedupKeyForWeek(kind SuggestionKind, t time.Time) string {
	y, w := t.UTC().ISOWeek()
	return string(kind) + ":" + isoLabel(y, w)
}

func isoLabel(year, week int) string {
	// 2026-W21 style. Keep dependencies tiny; this fits in two-digit weeks.
	const digits = "0123456789"
	yr := []byte{
		digits[(year/1000)%10],
		digits[(year/100)%10],
		digits[(year/10)%10],
		digits[year%10],
	}
	wk := []byte{
		'W',
		digits[(week/10)%10],
		digits[week%10],
	}
	return string(yr) + "-" + string(wk)
}

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

var (
	ErrSuggestionNotFound        = errors.New("suggestion: not found")
	ErrInvalidSuggestionKind     = errors.New("suggestion: invalid kind")
	ErrInvalidSuggestionSeverity = errors.New("suggestion: invalid severity")
	ErrInvalidSuggestionStatus   = errors.New("suggestion: invalid status")
	ErrSuggestionTitleEmpty      = errors.New("suggestion: title is required")
	ErrSuggestionMessageEmpty    = errors.New("suggestion: message is required")
	ErrSuggestionDedupKeyEmpty   = errors.New("suggestion: dedup_key is required")
	ErrSuggestionInvalidTransition = errors.New("suggestion: invalid status transition")
)
