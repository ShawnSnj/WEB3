-- deploy.sql — unified PostgreSQL schema (jobhunt-task + AI Job Hunt CRM)
--
-- Idempotent: safe to re-run on an existing database (uses IF NOT EXISTS).
-- Fresh installs: mounted by docker-compose into initdb, or applied via
--   make migrate / make migrate-all / make integrate
--
-- Task tracker tables: tasks, daily_reviews, task_execution_sessions,
--   reminders, suggestions, weekly_reviews, task_notes
-- CRM tables: user_profile, companies, job_postings, job_matches,
--   applications, contacts, outreach_messages, skill_analyses,
--   daily_briefs, weekly_crm_reports, daily_automation_log, resume_analyses

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- tasks
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS tasks (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    title             TEXT         NOT NULL,
    description       TEXT         NOT NULL DEFAULT '',
    priority          TEXT         NOT NULL DEFAULT 'medium',
    category          TEXT         NOT NULL DEFAULT 'misc',
    status            TEXT         NOT NULL DEFAULT 'pending',
    estimated_minutes INTEGER      NOT NULL DEFAULT 0,
    actual_minutes    INTEGER      NOT NULL DEFAULT 0,
    due_date          TIMESTAMPTZ,
    carry_over_count  INTEGER      NOT NULL DEFAULT 0,
    completed_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT tasks_title_not_blank
        CHECK (length(btrim(title)) > 0),
    CONSTRAINT tasks_priority_valid
        CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    CONSTRAINT tasks_status_valid
        CHECK (status IN ('pending', 'in_progress', 'completed', 'missed')),
    CONSTRAINT tasks_category_valid
        CHECK (category IN (
            'job_apply',
            'recruiter_outreach',
            'github',
            'twitter',
            'networking',
            'learning',
            'interview',
            'misc'
        )),
    CONSTRAINT tasks_estimated_nonneg CHECK (estimated_minutes >= 0),
    CONSTRAINT tasks_actual_nonneg    CHECK (actual_minutes    >= 0),
    CONSTRAINT tasks_carry_over_nonneg CHECK (carry_over_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_tasks_status_due
    ON tasks (status, due_date);

CREATE INDEX IF NOT EXISTS idx_tasks_due_date
    ON tasks (due_date)
    WHERE status IN ('pending', 'in_progress');

CREATE INDEX IF NOT EXISTS idx_tasks_category
    ON tasks (category);

CREATE OR REPLACE FUNCTION tasks_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_tasks_set_updated_at ON tasks;
CREATE TRIGGER trg_tasks_set_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION tasks_set_updated_at();

-- ---------------------------------------------------------------------------
-- daily_reviews
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS daily_reviews (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    review_date        DATE         NOT NULL UNIQUE,
    reflection         TEXT         NOT NULL DEFAULT '',
    blockers           TEXT[]       NOT NULL DEFAULT '{}',
    wins               TEXT[]       NOT NULL DEFAULT '{}',
    distractions       TEXT[]       NOT NULL DEFAULT '{}',
    notes              TEXT         NOT NULL DEFAULT '',
    energy_level       INTEGER      NOT NULL DEFAULT 0,
    productivity_score INTEGER      NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT daily_reviews_energy_range
        CHECK (energy_level >= 0 AND energy_level <= 10),
    CONSTRAINT daily_reviews_productivity_range
        CHECK (productivity_score >= 0 AND productivity_score <= 10)
);

CREATE INDEX IF NOT EXISTS idx_daily_reviews_date_desc
    ON daily_reviews (review_date DESC);

CREATE OR REPLACE FUNCTION daily_reviews_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_daily_reviews_set_updated_at ON daily_reviews;
CREATE TRIGGER trg_daily_reviews_set_updated_at
    BEFORE UPDATE ON daily_reviews
    FOR EACH ROW
    EXECUTE FUNCTION daily_reviews_set_updated_at();

-- Backfill columns added after initial deploy (0005_review_distractions_notes).
ALTER TABLE daily_reviews
    ADD COLUMN IF NOT EXISTS distractions TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS notes         TEXT    NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------------
-- task_execution_sessions
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS task_execution_sessions (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id              UUID         NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    status               TEXT         NOT NULL DEFAULT 'active',
    started_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    ended_at             TIMESTAMPTZ,
    paused_at            TIMESTAMPTZ,
    total_paused_seconds INTEGER      NOT NULL DEFAULT 0,
    interruptions        INTEGER      NOT NULL DEFAULT 0,
    completion_quality   SMALLINT     NOT NULL DEFAULT 0,
    notes                TEXT         NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT sessions_status_valid
        CHECK (status IN ('active', 'paused', 'stopped', 'completed')),
    CONSTRAINT sessions_quality_range
        CHECK (completion_quality >= 0 AND completion_quality <= 5),
    CONSTRAINT sessions_interruptions_nonneg
        CHECK (interruptions >= 0),
    CONSTRAINT sessions_paused_nonneg
        CHECK (total_paused_seconds >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_sessions_active_per_task
    ON task_execution_sessions (task_id)
    WHERE status IN ('active', 'paused');

CREATE INDEX IF NOT EXISTS idx_sessions_by_task
    ON task_execution_sessions (task_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_running
    ON task_execution_sessions (status)
    WHERE status IN ('active', 'paused');

CREATE OR REPLACE FUNCTION task_sessions_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_task_sessions_set_updated_at ON task_execution_sessions;
CREATE TRIGGER trg_task_sessions_set_updated_at
    BEFORE UPDATE ON task_execution_sessions
    FOR EACH ROW
    EXECUTE FUNCTION task_sessions_set_updated_at();

-- ---------------------------------------------------------------------------
-- reminders
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS reminders (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    kind            TEXT         NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'pending',
    dedup_key       TEXT,
    scheduled_for   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    payload         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    attempts        INTEGER      NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    last_error      TEXT,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT reminders_kind_valid
        CHECK (kind IN (
            'morning',
            'evening_review',
            'weekly_review',
            'overdue',
            'custom'
        )),
    CONSTRAINT reminders_status_valid
        CHECK (status IN ('pending', 'sent', 'failed', 'cancelled')),
    CONSTRAINT reminders_attempts_nonneg
        CHECK (attempts >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_reminders_dedup_key
    ON reminders (dedup_key)
    WHERE dedup_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_reminders_due
    ON reminders (scheduled_for)
    WHERE status IN ('pending', 'failed');

CREATE INDEX IF NOT EXISTS idx_reminders_kind_status
    ON reminders (kind, status);

CREATE OR REPLACE FUNCTION reminders_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_reminders_set_updated_at ON reminders;
CREATE TRIGGER trg_reminders_set_updated_at
    BEFORE UPDATE ON reminders
    FOR EACH ROW
    EXECUTE FUNCTION reminders_set_updated_at();

-- ---------------------------------------------------------------------------
-- suggestions
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS suggestions (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    kind          TEXT         NOT NULL,
    severity      TEXT         NOT NULL DEFAULT 'warning',
    status        TEXT         NOT NULL DEFAULT 'active',
    title         TEXT         NOT NULL,
    message       TEXT         NOT NULL,
    payload       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    dedup_key     TEXT         NOT NULL,
    generated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ,
    dismissed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT suggestions_kind_valid
        CHECK (kind IN (
            'reduce_workload',
            'smaller_tasks',
            'easier_wins',
            'focus_shift'
        )),
    CONSTRAINT suggestions_severity_valid
        CHECK (severity IN ('info', 'warning', 'critical')),
    CONSTRAINT suggestions_status_valid
        CHECK (status IN ('active', 'dismissed', 'expired'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_suggestions_dedup
    ON suggestions (dedup_key);

CREATE INDEX IF NOT EXISTS idx_suggestions_status_generated
    ON suggestions (status, generated_at DESC);

CREATE INDEX IF NOT EXISTS idx_suggestions_kind
    ON suggestions (kind);

CREATE OR REPLACE FUNCTION suggestions_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_suggestions_set_updated_at ON suggestions;
CREATE TRIGGER trg_suggestions_set_updated_at
    BEFORE UPDATE ON suggestions
    FOR EACH ROW
    EXECUTE FUNCTION suggestions_set_updated_at();

-- ---------------------------------------------------------------------------
-- weekly_reviews
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS weekly_reviews (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    week_start           DATE         NOT NULL,
    wins                 TEXT         NOT NULL DEFAULT '',
    bottlenecks          TEXT         NOT NULL DEFAULT '',
    improvement_notes    TEXT         NOT NULL DEFAULT '',
    next_week_priorities TEXT         NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT weekly_reviews_week_start_unique UNIQUE (week_start)
);

CREATE INDEX IF NOT EXISTS idx_weekly_reviews_week_start_desc
    ON weekly_reviews (week_start DESC);

CREATE OR REPLACE FUNCTION weekly_reviews_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_weekly_reviews_set_updated_at ON weekly_reviews;
CREATE TRIGGER trg_weekly_reviews_set_updated_at
    BEFORE UPDATE ON weekly_reviews
    FOR EACH ROW
    EXECUTE FUNCTION weekly_reviews_set_updated_at();

-- ---------------------------------------------------------------------------
-- task_notes
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS task_notes (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           UUID         NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    note_type         TEXT         NOT NULL DEFAULT 'GENERAL_NOTE',
    title             TEXT         NOT NULL DEFAULT '',
    content           TEXT         NOT NULL DEFAULT '',
    person_name       TEXT,
    company           TEXT,
    role_title        TEXT,
    platform          TEXT,
    profile_url       TEXT,
    message_content   TEXT,
    sent_at           TIMESTAMPTZ,
    reply_status      TEXT,
    reply_at          TIMESTAMPTZ,
    job_title         TEXT,
    job_url           TEXT,
    application_status TEXT,
    applied_at        TIMESTAMPTZ,
    resume_version    TEXT,
    fit_score         INTEGER,
    source            TEXT,
    notes             TEXT,
    is_marked         BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_notes_task_id
    ON task_notes (task_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_notes_note_type
    ON task_notes (note_type);

CREATE INDEX IF NOT EXISTS idx_task_notes_company
    ON task_notes (company);

CREATE INDEX IF NOT EXISTS idx_task_notes_applied_at
    ON task_notes (applied_at);

CREATE INDEX IF NOT EXISTS idx_task_notes_sent_at
    ON task_notes (sent_at);

CREATE INDEX IF NOT EXISTS idx_task_notes_is_marked
    ON task_notes (is_marked)
    WHERE is_marked = TRUE;

CREATE OR REPLACE FUNCTION task_notes_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_task_notes_set_updated_at ON task_notes;
CREATE TRIGGER trg_task_notes_set_updated_at
    BEFORE UPDATE ON task_notes
    FOR EACH ROW
    EXECUTE FUNCTION task_notes_set_updated_at();

-- ---------------------------------------------------------------------------
-- CRM — AI Job Hunt (same database, separate tables)
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
