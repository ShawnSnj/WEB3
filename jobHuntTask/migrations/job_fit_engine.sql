-- job_fit_engine.sql — personalized fit scoring + master resume fields
-- Run: make migrate-job-fit-engine

BEGIN;

-- Master resume aliases on candidate profile
ALTER TABLE candidate_profiles ADD COLUMN IF NOT EXISTS core_skills TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE candidate_profiles ADD COLUMN IF NOT EXISTS secondary_skills TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE candidate_profiles ADD COLUMN IF NOT EXISTS avoid_roles TEXT[] NOT NULL DEFAULT '{}';

UPDATE candidate_profiles SET core_skills = strongest_skills
WHERE cardinality(core_skills) = 0 AND cardinality(strongest_skills) > 0;
UPDATE candidate_profiles SET secondary_skills = medium_skills
WHERE cardinality(secondary_skills) = 0 AND cardinality(medium_skills) > 0;
UPDATE candidate_profiles SET avoid_roles = ARRAY[
  'Frontend Engineer', 'Junior Engineer', 'Intern', 'Marketing',
  'Content', 'Growth', 'Recruiter', 'Sales', 'Designer'
]::TEXT[] WHERE cardinality(avoid_roles) = 0;

-- Detailed fit scores + explanations
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS title_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS backend_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS infra_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS negative_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS final_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS fit_tier TEXT NOT NULL DEFAULT 'C';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS why_this_matches_me TEXT NOT NULL DEFAULT '';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS missing_skills TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS resume_keywords_to_add TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS suggested_resume_angle TEXT NOT NULL DEFAULT '';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS application_priority TEXT NOT NULL DEFAULT 'low';
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS user_action TEXT NOT NULL DEFAULT '';

DO $$ BEGIN
  ALTER TABLE job_matches ADD CONSTRAINT job_matches_fit_tier_check
    CHECK (fit_tier IN ('A', 'B', 'C'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_job_matches_final_tier
  ON job_matches (fit_tier, final_score DESC);

COMMIT;
