package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

type UpsertOutcome int

const (
	UpsertInserted UpsertOutcome = iota
	UpsertUpdated
	UpsertSkippedDuplicate
)

// StartFetchRun creates a running fetch audit row.
func (s *Store) StartFetchRun(ctx context.Context) (uuid.UUID, time.Time, error) {
	var id uuid.UUID
	var started time.Time
	err := s.pool.QueryRow(ctx, `
		INSERT INTO job_fetch_runs (status) VALUES ('running')
		RETURNING id, started_at`).Scan(&id, &started)
	return id, started, err
}

// FinishFetchRun persists final fetch statistics.
func (s *Store) FinishFetchRun(ctx context.Context, id uuid.UUID, result *crm.FetchRunResult) error {
	src, _ := json.Marshal(result.SourceStats)
	_, err := s.pool.Exec(ctx, `
		UPDATE job_fetch_runs SET
			finished_at = NOW(),
			status = $2,
			jobs_fetched = $3,
			jobs_inserted = $4,
			jobs_updated = $5,
			jobs_skipped = $6,
			jobs_failed = $7,
			jobs_scored = $8,
			source_stats = $9,
			error_message = $10
		WHERE id = $1`,
		id, result.Status, result.JobsFetched, result.JobsInserted,
		result.JobsUpdated, result.JobsSkipped, result.JobsFailed,
		result.JobsScored, src, result.ErrorMessage,
	)
	return err
}

// LatestFetchRun returns the most recent fetch audit row.
func (s *Store) LatestFetchRun(ctx context.Context) (*crm.FetchRunResult, error) {
	const q = `
		SELECT id, started_at, finished_at, status,
		       jobs_fetched, jobs_inserted, jobs_updated, jobs_skipped,
		       jobs_failed, jobs_scored, source_stats, error_message
		FROM job_fetch_runs ORDER BY started_at DESC LIMIT 1`
	var result crm.FetchRunResult
	var id uuid.UUID
	var finished *time.Time
	var src []byte
	err := s.pool.QueryRow(ctx, q).Scan(
		&id, &result.StartedAt, &finished, &result.Status,
		&result.JobsFetched, &result.JobsInserted, &result.JobsUpdated,
		&result.JobsSkipped, &result.JobsFailed, &result.JobsScored,
		&src, &result.ErrorMessage,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	result.RunID = id.String()
	result.FinishedAt = finished
	if len(src) > 0 {
		_ = json.Unmarshal(src, &result.SourceStats)
	}
	return &result, nil
}

// CountJobsBySource returns active job counts grouped by source.
func (s *Store) CountJobsBySource(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT source, COUNT(*) FROM job_postings WHERE is_active GROUP BY source ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var src string
		var n int
		if err := rows.Scan(&src, &n); err != nil {
			return nil, err
		}
		out[src] = n
	}
	return out, rows.Err()
}

// CountScoredJobs returns jobs with a match row.
func (s *Store) CountScoredJobs(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM job_postings j
		INNER JOIN job_matches m ON m.job_id = j.id WHERE j.is_active`).Scan(&n)
	return n, err
}

// CountJobsByDecision returns match decision buckets.
func (s *Store) CountJobsByDecision(ctx context.Context) (apply, maybe, skip int, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT decision, COUNT(*) FROM job_matches GROUP BY decision`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var n int
		if err := rows.Scan(&d, &n); err != nil {
			return 0, 0, 0, err
		}
		switch d {
		case "apply":
			apply = n
		case "maybe":
			maybe = n
		case "skip":
			skip = n
		}
	}
	return apply, maybe, skip, rows.Err()
}

// UpsertJobOutcome upserts a job and reports insert/update/skip.
func (s *Store) UpsertJobOutcome(ctx context.Context, j *crm.RawJob) (*crm.JobPosting, UpsertOutcome, error) {
	if j.ApplicationURL != "" {
		var dupID uuid.UUID
		err := s.pool.QueryRow(ctx, `
			SELECT id FROM job_postings
			WHERE application_url = $1 AND (source <> $2 OR external_id <> $3)
			LIMIT 1`, j.ApplicationURL, j.Source, j.ExternalID).Scan(&dupID)
		if err == nil {
			return nil, UpsertSkippedDuplicate, nil
		}
		if err != pgx.ErrNoRows {
			return nil, 0, err
		}
	}
	var existed bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM job_postings WHERE source = $1 AND external_id = $2)`,
		j.Source, j.ExternalID,
	).Scan(&existed); err != nil {
		return nil, 0, err
	}
	job, err := s.UpsertJob(ctx, j)
	if err != nil {
		return nil, 0, err
	}
	if existed {
		return job, UpsertUpdated, nil
	}
	return job, UpsertInserted, nil
}
