-- job_fit.sql — Phase 2: Job Fit Engine columns on job_matches
-- Run: make migrate-job-fit

BEGIN;

ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS decision TEXT NOT NULL DEFAULT 'skip';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS career_roi_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS interview_probability SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS domain_match_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS missing_keywords TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS resume_version_recommendation TEXT NOT NULL DEFAULT '';

DO $$ BEGIN
  ALTER TABLE job_matches ADD CONSTRAINT job_matches_decision_check
    CHECK (decision IN ('apply', 'maybe', 'skip'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  ALTER TABLE job_matches ADD CONSTRAINT job_matches_career_roi_range
    CHECK (career_roi_score >= 0 AND career_roi_score <= 100);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_job_matches_decision_roi
  ON job_matches (decision, career_roi_score DESC, fit_score DESC);

COMMIT;
