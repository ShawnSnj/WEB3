-- deploy.sql — full PostgreSQL schema for jobhunt-task
--
-- Idempotent: safe to re-run on an existing database (uses IF NOT EXISTS).
-- Fresh installs: mounted by docker-compose into initdb, or applied via
--   make migrate
--
-- Tables: tasks, daily_reviews, task_execution_sessions, reminders,
--         suggestions, weekly_reviews

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
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id    UUID         NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    title      TEXT         NOT NULL DEFAULT '',
    content    TEXT         NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT task_notes_title_not_blank
        CHECK (length(btrim(title)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_task_notes_task_id
    ON task_notes (task_id, updated_at DESC);

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

COMMIT;
