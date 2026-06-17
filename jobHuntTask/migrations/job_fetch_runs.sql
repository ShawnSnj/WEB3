-- job_fetch_runs.sql — fetch audit log for CRM job collector
-- Run: make migrate-job-fetch

BEGIN;

CREATE TABLE IF NOT EXISTS job_fetch_runs (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at        TIMESTAMPTZ,
    status             TEXT         NOT NULL DEFAULT 'running',
    jobs_fetched       INT          NOT NULL DEFAULT 0,
    jobs_inserted      INT          NOT NULL DEFAULT 0,
    jobs_updated       INT          NOT NULL DEFAULT 0,
    jobs_skipped       INT          NOT NULL DEFAULT 0,
    jobs_failed        INT          NOT NULL DEFAULT 0,
    jobs_scored        INT          NOT NULL DEFAULT 0,
    source_stats       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    error_message      TEXT         NOT NULL DEFAULT '',
    CONSTRAINT job_fetch_runs_status_check CHECK (status IN ('running', 'ok', 'partial', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_job_fetch_runs_started
    ON job_fetch_runs (started_at DESC);

-- Dedupe by application URL when non-empty (cross-source).
CREATE UNIQUE INDEX IF NOT EXISTS idx_job_postings_application_url_unique
    ON job_postings (application_url)
    WHERE application_url <> '';

COMMIT;
