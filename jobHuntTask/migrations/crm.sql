-- crm.sql — CRM-only schema (idempotent)
-- NOTE: CRM tables are also included in deploy.sql (unified database).
-- Use `make migrate-all` for the full schema, or this file to add CRM
-- tables to an existing task-tracker database: make migrate-crm

BEGIN;

-- ---------------------------------------------------------------------------
-- user_profile — single-user career preferences & resume
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS user_profile (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name          TEXT         NOT NULL DEFAULT 'Engineer',
    headline              TEXT         NOT NULL DEFAULT '',
    skills                TEXT[]       NOT NULL DEFAULT '{}',
    target_titles         TEXT[]       NOT NULL DEFAULT '{}',
    target_industries     TEXT[]       NOT NULL DEFAULT '{}',
    resume_text           TEXT         NOT NULL DEFAULT '',
    min_salary_usd        INTEGER      NOT NULL DEFAULT 0,
    remote_only           BOOLEAN      NOT NULL DEFAULT TRUE,
    web3_preferred        BOOLEAN      NOT NULL DEFAULT TRUE,
    daily_applications    INTEGER      NOT NULL DEFAULT 1,
    daily_outreach        INTEGER      NOT NULL DEFAULT 2,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO user_profile (display_name, headline, skills, target_titles, target_industries, resume_text)
SELECT
    'Senior Backend Engineer',
    'Backend / Platform Engineer — Go, Java, Kafka, Distributed Systems',
    ARRAY['Go','Java','Kafka','SQL','PostgreSQL','Distributed Systems','Cloud','AWS','Docker','Kubernetes','gRPC','Redis','Web3','Blockchain'],
    ARRAY['Backend Engineer','Staff Backend Engineer','Platform Engineer','Infrastructure Engineer','Web3 Backend Engineer'],
    ARRAY['Web3','Crypto','Fintech','Infrastructure','Developer Tools'],
    ''
WHERE NOT EXISTS (SELECT 1 FROM user_profile LIMIT 1);

-- ---------------------------------------------------------------------------
-- companies
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS companies (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT         NOT NULL,
    website     TEXT         NOT NULL DEFAULT '',
    industry    TEXT         NOT NULL DEFAULT '',
    size_band   TEXT         NOT NULL DEFAULT '',
    web3        BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT companies_name_unique UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS idx_companies_web3 ON companies (web3);

-- ---------------------------------------------------------------------------
-- job_postings — normalized listings from all sources
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS job_postings (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id      TEXT         NOT NULL,
    source           TEXT         NOT NULL,
    title            TEXT         NOT NULL,
    company_id       UUID         REFERENCES companies(id) ON DELETE SET NULL,
    company_name     TEXT         NOT NULL,
    salary_min_usd   INTEGER,
    salary_max_usd   INTEGER,
    salary_raw       TEXT         NOT NULL DEFAULT '',
    location         TEXT         NOT NULL DEFAULT '',
    remote           BOOLEAN      NOT NULL DEFAULT FALSE,
    description      TEXT         NOT NULL DEFAULT '',
    required_skills  TEXT[]       NOT NULL DEFAULT '{}',
    application_url  TEXT         NOT NULL DEFAULT '',
    posted_at        TIMESTAMPTZ,
    seniority        TEXT         NOT NULL DEFAULT '',
    web3             BOOLEAN      NOT NULL DEFAULT FALSE,
    raw_payload      JSONB        NOT NULL DEFAULT '{}'::jsonb,
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT job_postings_source_external_unique UNIQUE (source, external_id)
);

CREATE INDEX IF NOT EXISTS idx_job_postings_active_posted
    ON job_postings (is_active, posted_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_job_postings_remote ON job_postings (remote) WHERE is_active;
CREATE INDEX IF NOT EXISTS idx_job_postings_web3 ON job_postings (web3) WHERE is_active;

-- ---------------------------------------------------------------------------
-- job_matches — AI / heuristic fit scores
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS job_matches (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID         NOT NULL REFERENCES job_postings(id) ON DELETE CASCADE,
    fit_score     SMALLINT     NOT NULL DEFAULT 0,
    skill_score   SMALLINT     NOT NULL DEFAULT 0,
    remote_score  SMALLINT     NOT NULL DEFAULT 0,
    salary_score  SMALLINT     NOT NULL DEFAULT 0,
    seniority_score SMALLINT   NOT NULL DEFAULT 0,
    web3_score    SMALLINT     NOT NULL DEFAULT 0,
    pros          TEXT[]       NOT NULL DEFAULT '{}',
    risks         TEXT[]       NOT NULL DEFAULT '{}',
    summary       TEXT         NOT NULL DEFAULT '',
    model         TEXT         NOT NULL DEFAULT 'heuristic',
    scored_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT job_matches_job_unique UNIQUE (job_id),
    CONSTRAINT job_matches_fit_range CHECK (fit_score >= 0 AND fit_score <= 100)
);

CREATE INDEX IF NOT EXISTS idx_job_matches_fit ON job_matches (fit_score DESC);

-- ---------------------------------------------------------------------------
-- applications — pipeline tracker
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS applications (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID         REFERENCES job_postings(id) ON DELETE SET NULL,
    company_name  TEXT         NOT NULL DEFAULT '',
    role_title    TEXT         NOT NULL,
    status        TEXT         NOT NULL DEFAULT 'saved',
    applied_at    TIMESTAMPTZ,
    notes         TEXT         NOT NULL DEFAULT '',
    resume_score  SMALLINT,
    match_score   SMALLINT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT applications_status_valid CHECK (status IN (
        'saved','applied','interview','technical','final_round','offer','rejected'
    ))
);

CREATE INDEX IF NOT EXISTS idx_applications_status ON applications (status, updated_at DESC);

-- ---------------------------------------------------------------------------
-- contacts — outreach targets
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS contacts (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id    UUID         REFERENCES companies(id) ON DELETE SET NULL,
    company_name  TEXT         NOT NULL DEFAULT '',
    full_name     TEXT         NOT NULL,
    title         TEXT         NOT NULL DEFAULT '',
    role_type     TEXT         NOT NULL DEFAULT 'recruiter',
    linkedin_url  TEXT         NOT NULL DEFAULT '',
    email         TEXT         NOT NULL DEFAULT '',
    notes         TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT contacts_role_type_valid CHECK (role_type IN (
        'recruiter','engineering_manager','hiring_manager','founder','other'
    ))
);

CREATE INDEX IF NOT EXISTS idx_contacts_company ON contacts (company_name);

-- ---------------------------------------------------------------------------
-- outreach_messages — generated DMs
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS outreach_messages (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id    UUID         REFERENCES contacts(id) ON DELETE CASCADE,
    job_id        UUID         REFERENCES job_postings(id) ON DELETE SET NULL,
    subject       TEXT         NOT NULL DEFAULT '',
    body          TEXT         NOT NULL,
    status        TEXT         NOT NULL DEFAULT 'draft',
    sent_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT outreach_status_valid CHECK (status IN ('draft','sent','replied','ignored'))
);

-- ---------------------------------------------------------------------------
-- skill_analyses — periodic skill gap snapshots
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS skill_analyses (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_date     DATE         NOT NULL DEFAULT CURRENT_DATE,
    top_demanded      JSONB        NOT NULL DEFAULT '[]'::jsonb,
    missing_skills    JSONB        NOT NULL DEFAULT '[]'::jsonb,
    learning_priority JSONB        NOT NULL DEFAULT '[]'::jsonb,
    jobs_analyzed     INTEGER      NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT skill_analyses_date_unique UNIQUE (analysis_date)
);

-- ---------------------------------------------------------------------------
-- daily_briefs — cached "what to do today"
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS daily_briefs (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    brief_date          DATE         NOT NULL UNIQUE,
    apply_job_id        UUID         REFERENCES job_postings(id) ON DELETE SET NULL,
    apply_summary       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    outreach_targets    JSONB        NOT NULL DEFAULT '[]'::jsonb,
    learning_skill      TEXT         NOT NULL DEFAULT '',
    learning_reason     TEXT         NOT NULL DEFAULT '',
    automation_tasks    JSONB        NOT NULL DEFAULT '[]'::jsonb,
    generated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- weekly_crm_reports
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS weekly_crm_reports (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    week_start          DATE         NOT NULL UNIQUE,
    jobs_found          INTEGER      NOT NULL DEFAULT 0,
    applications_sent   INTEGER      NOT NULL DEFAULT 0,
    response_rate       NUMERIC(5,2) NOT NULL DEFAULT 0,
    interview_rate      NUMERIC(5,2) NOT NULL DEFAULT 0,
    skill_gap_changes   JSONB        NOT NULL DEFAULT '[]'::jsonb,
    coach_summary       TEXT         NOT NULL DEFAULT '',
    payload             JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- daily_automation_log — track daily goals
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS daily_automation_log (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    log_date        DATE         NOT NULL,
    kind            TEXT         NOT NULL,
    target_count    INTEGER      NOT NULL DEFAULT 1,
    completed_count INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT daily_automation_kind_valid CHECK (kind IN ('application','outreach')),
    CONSTRAINT daily_automation_date_kind_unique UNIQUE (log_date, kind)
);

-- ---------------------------------------------------------------------------
-- resume_analyses
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS resume_analyses (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID         REFERENCES job_postings(id) ON DELETE CASCADE,
    match_score   SMALLINT     NOT NULL DEFAULT 0,
    missing_keywords TEXT[]    NOT NULL DEFAULT '{}',
    suggestions   TEXT[]       NOT NULL DEFAULT '{}',
    model         TEXT         NOT NULL DEFAULT 'heuristic',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_resume_analyses_job ON resume_analyses (job_id, created_at DESC);

COMMIT;
