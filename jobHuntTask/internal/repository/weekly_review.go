package repository

import (
	"context"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

// WeeklyReviewRepository persists end-of-week reflection notes.
type WeeklyReviewRepository interface {
	Upsert(ctx context.Context, r *model.WeeklyReview) error
	GetByWeekStart(ctx context.Context, weekStart time.Time) (*model.WeeklyReview, error)
	Delete(ctx context.Context, weekStart time.Time) error
}
