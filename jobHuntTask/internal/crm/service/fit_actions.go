package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/crm/engine/jobfit"
	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/repository"
)

// ListFitJobs returns personalized A/B ranked jobs for the UI.
func (s *CRM) ListFitJobs(ctx context.Context, tier string, limit int) ([]crm.JobPosting, error) {
	if limit <= 0 {
		limit = 30
	}
	f := s.defaultFitFilter(limit)
	if tier == "A" || tier == "B" {
		f.Tier = tier
	}
	return s.store.ListJobs(ctx, f)
}

// JobAction records apply, skip, or save on a scored job.
func (s *CRM) JobAction(ctx context.Context, jobID uuid.UUID, action string) error {
	candidate, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return err
	}
	candidate.NormalizeMasterProfile()
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	ensureAction := func(userAction string) error {
		if job.Match != nil {
			return s.store.SetJobUserAction(ctx, jobID, userAction)
		}
		m := jobfit.Score(candidate, job)
		s.enhanceMatch(m, job)
		m.UserAction = userAction
		return s.store.UpsertMatch(ctx, m)
	}

	switch action {
	case "skip":
		return ensureAction("skipped")
	case "save":
		if err := ensureAction("saved"); err != nil {
			return err
		}
		_, err := s.SaveApplication(ctx, jobID, crm.AppSaved)
		return err
	case "apply":
		_ = ensureAction("applied")
		_, err := s.SaveApplication(ctx, jobID, crm.AppApplied)
		return err
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

// RescoreAll re-runs fit engine on active jobs (precision over volume).
func (s *CRM) RescoreAll(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 300
	}
	jobs, err := s.store.ListJobs(ctx, repository.JobFilter{Limit: limit, Active: true})
	if err != nil {
		return 0, err
	}
	candidate, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return 0, err
	}
	candidate.NormalizeMasterProfile()
	done := 0
	for _, job := range jobs {
		m := jobfit.Score(candidate, &job)
		if job.Match != nil {
			m.ID = job.Match.ID
			m.UserAction = job.Match.UserAction
		}
		s.enhanceMatch(m, &job)
		if err := s.store.UpsertMatch(ctx, m); err != nil {
			continue
		}
		done++
	}
	return done, nil
}
