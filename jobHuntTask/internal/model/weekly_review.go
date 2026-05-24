package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrWeeklyReviewNotFound = errors.New("weekly review not found")

// WeeklyReview captures end-of-week reflection keyed by the start date of
// the 7-day rolling window (UTC midnight).
type WeeklyReview struct {
	ID                 uuid.UUID
	WeekStart          time.Time
	Wins               string
	Bottlenecks        string
	ImprovementNotes   string
	NextWeekPriorities string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NormalizeWeekStart truncates to UTC midnight — the canonical week key.
func NormalizeWeekStart(t time.Time) time.Time {
	return NormalizeDate(t)
}
