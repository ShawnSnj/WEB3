package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// UpsertReviewInput is the service-layer DTO for create-or-update.
// Nil pointer fields preserve the existing value when updating.
type UpsertReviewInput struct {
	Date              time.Time
	Reflection        *string
	Blockers          *[]string
	Wins              *[]string
	Distractions      *[]string
	Notes             *string
	EnergyLevel       *int
	ProductivityScore *int
}

// ListReviewsInput aliases the repository filter for clean service-level use.
type ListReviewsInput = repository.ReviewFilter

// DailyReviewService implements the business rules for daily reviews.
type DailyReviewService struct {
	repo  repository.ReviewRepository
	clock Clock
}

func NewDailyReviewService(repo repository.ReviewRepository, clock Clock) *DailyReviewService {
	if clock == nil {
		clock = SystemClock
	}
	return &DailyReviewService{repo: repo, clock: clock}
}

// Upsert creates a review for in.Date, or updates the existing one. Fields
// left nil on update keep their current values.
func (s *DailyReviewService) Upsert(ctx context.Context, in UpsertReviewInput) (*model.DailyReview, error) {
	date := model.NormalizeDate(in.Date)
	if in.Date.IsZero() {
		date = model.NormalizeDate(s.clock.Now())
	}

	// Start from existing row when present so nil fields preserve values.
	existing, err := s.repo.GetByDate(ctx, date)
	if err != nil && err != model.ErrReviewNotFound {
		return nil, err
	}

	rv := &model.DailyReview{
		ReviewDate:   date,
		Blockers:     []string{},
		Wins:         []string{},
		Distractions: []string{},
	}
	if existing != nil {
		rv = existing
		rv.ReviewDate = date // normalised
	}

	if in.Reflection != nil {
		rv.Reflection = strings.TrimSpace(*in.Reflection)
	}
	if in.Blockers != nil {
		rv.Blockers = sanitizeStringSlice(*in.Blockers)
	}
	if in.Wins != nil {
		rv.Wins = sanitizeStringSlice(*in.Wins)
	}
	if in.Distractions != nil {
		rv.Distractions = sanitizeStringSlice(*in.Distractions)
	}
	if in.Notes != nil {
		rv.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.EnergyLevel != nil {
		rv.EnergyLevel = *in.EnergyLevel
	}
	if in.ProductivityScore != nil {
		rv.ProductivityScore = *in.ProductivityScore
	}

	if err := s.repo.Upsert(ctx, rv); err != nil {
		return nil, err
	}
	return rv, nil
}

// GetByDate returns the review for the given calendar day.
func (s *DailyReviewService) GetByDate(ctx context.Context, date time.Time) (*model.DailyReview, error) {
	return s.repo.GetByDate(ctx, date)
}

// GetByID returns the review with the given ID.
func (s *DailyReviewService) GetByID(ctx context.Context, id uuid.UUID) (*model.DailyReview, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns reviews matching filter.
func (s *DailyReviewService) List(ctx context.Context, f ListReviewsInput) ([]*model.DailyReview, error) {
	return s.repo.List(ctx, f)
}

// Delete removes the review for the given date.
func (s *DailyReviewService) Delete(ctx context.Context, date time.Time) error {
	return s.repo.Delete(ctx, date)
}

// sanitizeStringSlice trims whitespace and drops empty entries while
// preserving order. The result is never nil (empty slice when all dropped).
func sanitizeStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
