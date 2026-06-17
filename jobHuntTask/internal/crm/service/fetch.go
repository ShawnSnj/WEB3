package service

import (
	"context"
	"log/slog"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/repository"
)

// FetchJobs pulls from all configured boards, stores results, and scores new jobs.
func (s *CRM) FetchJobs(ctx context.Context) (*crm.FetchRunResult, error) {
	runID, started, err := s.store.StartFetchRun(ctx)
	if err != nil {
		return nil, err
	}

	result := &crm.FetchRunResult{
		RunID:     runID.String(),
		Status:    crm.FetchRunning,
		StartedAt: started,
	}

	collect, err := s.agg.CollectDetailed(ctx)
	if err != nil {
		result.Status = crm.FetchError
		result.ErrorMessage = err.Error()
		_ = s.store.FinishFetchRun(ctx, runID, result)
		return result, err
	}

	for _, st := range collect.Sources {
		result.SourceStats = append(result.SourceStats, crm.SourceFetchStats{
			Source:   st.Source,
			Fetched:  st.Fetched,
			Filtered: st.Filtered,
			Error:    st.Error,
		})
	}
	result.JobsFetched = len(collect.Jobs) + collect.FilteredOut

	var sourceErrors int
	for _, st := range collect.Sources {
		if st.Error != "" {
			sourceErrors++
		}
	}

	for _, r := range collect.Jobs {
		job, outcome, err := s.store.UpsertJobOutcome(ctx, &r)
		if err != nil {
			result.JobsFailed++
			s.log.Warn("upsert job failed",
				slog.String("source", r.Source),
				slog.String("title", r.Title),
				slog.String("error", err.Error()),
			)
			continue
		}
		switch outcome {
		case repository.UpsertInserted:
			result.JobsInserted++
		case repository.UpsertUpdated:
			result.JobsUpdated++
		case repository.UpsertSkippedDuplicate:
			result.JobsSkipped++
		}
		if job != nil {
			if err := s.events.PublishJobIngested(ctx, job.ID); err != nil {
				s.log.Warn("kafka publish failed", slog.String("error", err.Error()))
			}
		}
	}
	result.JobsSkipped += collect.FilteredOut

	scored, scoreErr := s.ScorePending(ctx, 500)
	result.JobsScored = scored
	if scoreErr != nil {
		s.log.Warn("score after fetch failed", slog.String("error", scoreErr.Error()))
		if result.ErrorMessage == "" {
			result.ErrorMessage = "scoring: " + scoreErr.Error()
		}
	}

	switch {
	case sourceErrors == len(collect.Sources) && result.JobsInserted == 0 && result.JobsUpdated == 0:
		result.Status = crm.FetchError
		if result.ErrorMessage == "" {
			result.ErrorMessage = "all sources failed"
		}
	case sourceErrors > 0 || result.JobsFailed > 0 || scoreErr != nil:
		result.Status = crm.FetchPartial
	default:
		result.Status = crm.FetchOK
	}

	now := s.clock()
	result.FinishedAt = &now
	_ = s.store.FinishFetchRun(ctx, runID, result)

	result.TotalInDB, _ = s.store.CountActiveJobs(ctx)
	result.JobsBySource, _ = s.store.CountJobsBySource(ctx)

	s.log.Info("job fetch complete",
		slog.Int("fetched", result.JobsFetched),
		slog.Int("inserted", result.JobsInserted),
		slog.Int("updated", result.JobsUpdated),
		slog.Int("skipped", result.JobsSkipped),
		slog.Int("failed", result.JobsFailed),
		slog.Int("scored", result.JobsScored),
		slog.String("status", string(result.Status)),
	)
	return result, nil
}

// GetFetchStatus returns debug info for the jobs page.
func (s *CRM) GetFetchStatus(ctx context.Context) (*crm.FetchStatus, error) {
	status := &crm.FetchStatus{}
	total, _ := s.store.CountActiveJobs(ctx)
	status.TotalJobsInDB = total
	scored, _ := s.store.CountScoredJobs(ctx)
	status.ScoredJobsInDB = scored
	status.JobsBySource, _ = s.store.CountJobsBySource(ctx)
	status.ApplyDecisions, status.MaybeDecisions, status.SkipDecisions, _ = s.store.CountJobsByDecision(ctx)

	last, err := s.store.LatestFetchRun(ctx)
	if err == nil && last != nil {
		status.LastFetchAt = &last.StartedAt
		status.LastFetchStatus = last.Status
		status.JobsInsertedCount = last.JobsInserted
		status.JobsFetchedCount = last.JobsFetched
		status.JobsScoredCount = last.JobsScored
		status.FetchErrorMessage = last.ErrorMessage
		status.SourceStats = last.SourceStats
	}
	return status, nil
}
