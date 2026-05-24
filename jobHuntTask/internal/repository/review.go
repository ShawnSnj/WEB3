package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// ReviewFilter captures optional query parameters for ListReviews.
// Zero-value fields mean "no filter".
type ReviewFilter struct {
	From   *time.Time // inclusive lower bound on review_date
	To     *time.Time // inclusive upper bound on review_date
	Limit  int        // 0 -> use a sensible default
	Offset int
}

// ReviewRepository is the storage contract for daily reviews.
type ReviewRepository interface {
	// Upsert inserts a review or updates the existing one for the same
	// review_date. On success the receiver's ID, CreatedAt and UpdatedAt
	// are populated.
	Upsert(ctx context.Context, r *model.DailyReview) error

	// GetByDate fetches the review for the given calendar day.
	// Returns model.ErrReviewNotFound when none exists.
	GetByDate(ctx context.Context, date time.Time) (*model.DailyReview, error)

	// GetByID fetches the review with the given ID.
	GetByID(ctx context.Context, id uuid.UUID) (*model.DailyReview, error)

	// List returns reviews matching filter, ordered by review_date DESC.
	List(ctx context.Context, f ReviewFilter) ([]*model.DailyReview, error)

	// Delete removes the review for the given date.
	Delete(ctx context.Context, date time.Time) error
}
