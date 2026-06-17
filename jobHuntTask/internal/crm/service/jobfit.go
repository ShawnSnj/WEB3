package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/crm/engine/jobfit"
	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/repository"
)

// GetJobFit returns full fit analysis for a job.
func (s *CRM) GetJobFit(ctx context.Context, jobID uuid.UUID) (*crm.JobPosting, error) {
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.Match != nil {
		return job, nil
	}
	if _, err := s.ScoreJob(ctx, jobID); err != nil {
		return nil, err
	}
	return s.store.GetJob(ctx, jobID)
}

func (s *CRM) scoringProfile(ctx context.Context) (*crm.UserProfile, *crm.CandidateProfile, error) {
	candidate, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return nil, nil, err
	}
	base, err := s.store.GetProfile(ctx)
	if err != nil {
		return nil, candidate, err
	}
	return jobfit.UserProfileFromCandidate(candidate, base), candidate, nil
}

func (s *CRM) enrichAndSaveMatch(ctx context.Context, m *crm.JobMatch, candidate *crm.CandidateProfile, job *crm.JobPosting) error {
	jobfit.EnrichMatch(m, candidate, job)
	return s.store.UpsertMatch(ctx, m)
}

// ListRankedJobs returns scored jobs filtered by apply decision bucket.
func (s *CRM) ListRankedJobs(ctx context.Context, decision crm.FitDecision, limit int) ([]crm.JobPosting, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListJobs(ctx, repository.JobFilter{
		Decision:   string(decision),
		ScoredOnly: true,
		Limit:      limit,
		Active:     true,
	})
}
