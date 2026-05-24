package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Review-specific errors
// ---------------------------------------------------------------------------

var (
	ErrReviewNotFound        = errors.New("review not found")
	ErrInvalidEnergyLevel    = errors.New("energy_level must be between 0 and 10")
	ErrInvalidProductivity   = errors.New("productivity_score must be between 0 and 10")
	ErrBlockerEmpty          = errors.New("blocker entries cannot be blank")
	ErrWinEmpty              = errors.New("win entries cannot be blank")
	ErrDistractionEmpty      = errors.New("distraction entries cannot be blank")
)

// ---------------------------------------------------------------------------
// DailyReview
// ---------------------------------------------------------------------------

// DailyReview is the user's reflection for a specific calendar day.
// There is at most one review per ReviewDate.
type DailyReview struct {
	ID                uuid.UUID
	ReviewDate        time.Time // midnight UTC — see NormalizeDate
	Reflection        string
	Blockers          []string
	Wins              []string
	Distractions      []string
	Notes             string
	EnergyLevel       int // 0 = not set, 1..10 = scale
	ProductivityScore int // 0 = not set, 1..10 = scale
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NormalizeDate truncates t to midnight UTC, the canonical form used by the
// repository and the unique constraint on review_date.
func NormalizeDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Validate verifies invariants on a freshly built or freshly loaded review.
func (r *DailyReview) Validate() error {
	if r.EnergyLevel < 0 || r.EnergyLevel > 10 {
		return ErrInvalidEnergyLevel
	}
	if r.ProductivityScore < 0 || r.ProductivityScore > 10 {
		return ErrInvalidProductivity
	}
	for _, b := range r.Blockers {
		if strings.TrimSpace(b) == "" {
			return ErrBlockerEmpty
		}
	}
	for _, w := range r.Wins {
		if strings.TrimSpace(w) == "" {
			return ErrWinEmpty
		}
	}
	for _, d := range r.Distractions {
		if strings.TrimSpace(d) == "" {
			return ErrDistractionEmpty
		}
	}
	return nil
}
