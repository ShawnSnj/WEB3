package model

import "time"

// FetchRunStatus is the outcome of a job fetch run.
type FetchRunStatus string

const (
	FetchRunning FetchRunStatus = "running"
	FetchOK      FetchRunStatus = "ok"
	FetchPartial FetchRunStatus = "partial"
	FetchError   FetchRunStatus = "error"
)

// SourceFetchStats counts per external job board.
type SourceFetchStats struct {
	Source   string `json:"source"`
	Fetched  int    `json:"fetched"`
	Filtered int    `json:"filtered"`
	Error    string `json:"error,omitempty"`
}

// FetchRunResult is returned by POST /jobs/fetch.
type FetchRunResult struct {
	RunID         string             `json:"run_id"`
	Status        FetchRunStatus     `json:"status"`
	JobsFetched   int                `json:"jobs_fetched"`
	JobsInserted  int                `json:"jobs_inserted"`
	JobsUpdated   int                `json:"jobs_updated"`
	JobsSkipped   int                `json:"jobs_skipped"`
	JobsFailed    int                `json:"jobs_failed"`
	JobsScored    int                `json:"jobs_scored"`
	SourceStats   []SourceFetchStats `json:"source_stats"`
	ErrorMessage  string             `json:"error_message,omitempty"`
	StartedAt     time.Time          `json:"started_at"`
	FinishedAt    *time.Time         `json:"finished_at,omitempty"`
	TotalInDB     int                `json:"total_in_db"`
	JobsBySource  map[string]int     `json:"jobs_by_source"`
}

// FetchStatus is returned by GET /jobs/fetch/status for the debug panel.
type FetchStatus struct {
	LastFetchAt        *time.Time         `json:"last_fetch_at,omitempty"`
	LastFetchStatus    FetchRunStatus     `json:"last_fetch_status,omitempty"`
	JobsInsertedCount  int                `json:"jobs_inserted_count"`
	JobsFetchedCount   int                `json:"jobs_fetched_count"`
	JobsScoredCount    int                `json:"jobs_scored_count"`
	FetchErrorMessage  string             `json:"fetch_error_message,omitempty"`
	TotalJobsInDB      int                `json:"total_jobs_in_db"`
	ScoredJobsInDB     int                `json:"scored_jobs_in_db"`
	ApplyDecisions     int                `json:"apply_decisions"`
	MaybeDecisions     int                `json:"maybe_decisions"`
	SkipDecisions      int                `json:"skip_decisions"`
	JobsBySource       map[string]int     `json:"jobs_by_source"`
	SourceStats        []SourceFetchStats `json:"source_stats,omitempty"`
}
