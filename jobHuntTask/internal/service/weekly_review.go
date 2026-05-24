package service

import (
	"context"
	"strings"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
)

// UpsertWeeklyReviewInput is the service DTO for weekly note upserts.
type UpsertWeeklyReviewInput struct {
	WeekStart          time.Time
	Wins               *string
	Bottlenecks        *string
	ImprovementNotes   *string
	NextWeekPriorities *string
}

// WeeklyReviewService manages persisted weekly reflection notes.
type WeeklyReviewService struct {
	repo  repository.WeeklyReviewRepository
	clock Clock
}

func NewWeeklyReviewService(repo repository.WeeklyReviewRepository, clock Clock) *WeeklyReviewService {
	if clock == nil {
		clock = SystemClock
	}
	return &WeeklyReviewService{repo: repo, clock: clock}
}

func (s *WeeklyReviewService) Upsert(ctx context.Context, in UpsertWeeklyReviewInput) (*model.WeeklyReview, error) {
	weekStart := model.NormalizeWeekStart(in.WeekStart)
	if in.WeekStart.IsZero() {
		weekStart = model.NormalizeWeekStart(s.clock.Now().AddDate(0, 0, -6))
	}

	existing, err := s.repo.GetByWeekStart(ctx, weekStart)
	if err != nil && err != model.ErrWeeklyReviewNotFound {
		return nil, err
	}

	rv := &model.WeeklyReview{WeekStart: weekStart}
	if existing != nil {
		rv = existing
	}
	if in.Wins != nil {
		rv.Wins = strings.TrimSpace(*in.Wins)
	}
	if in.Bottlenecks != nil {
		rv.Bottlenecks = strings.TrimSpace(*in.Bottlenecks)
	}
	if in.ImprovementNotes != nil {
		rv.ImprovementNotes = strings.TrimSpace(*in.ImprovementNotes)
	}
	if in.NextWeekPriorities != nil {
		rv.NextWeekPriorities = strings.TrimSpace(*in.NextWeekPriorities)
	}
	if err := s.repo.Upsert(ctx, rv); err != nil {
		return nil, err
	}
	return rv, nil
}

func (s *WeeklyReviewService) GetByWeekStart(ctx context.Context, weekStart time.Time) (*model.WeeklyReview, error) {
	return s.repo.GetByWeekStart(ctx, weekStart)
}
