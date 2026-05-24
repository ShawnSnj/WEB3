-- 0002_create_reviews_and_sessions.up.sql
--
-- Daily-review + task-execution-tracking tables.
--
-- Design:
--   * daily_reviews has UNIQUE(review_date) — exactly one review per day,
--     upserted via ON CONFLICT in the repository.
--   * blockers and wins are TEXT[] so each item stands alone and remains
--     queryable later (e.g. `WHERE 'rejection' = ANY(blockers)`).
--   * task_execution_sessions models a state machine
--         active <-> paused -> stopped | completed
--     A partial unique index guarantees AT MOST ONE active/paused session
--     per task — even under racing requests.
--   * total_paused_seconds is accumulated on resume/stop/complete so that
--     effective work time is (ended_at - started_at) - total_paused_seconds.
--   * Each table gets its own updated_at trigger function to avoid touching
--     the trigger created by migration 0001.

-- ---------------------------------------------------------------------------
-- daily_reviews
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS daily_reviews (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    review_date        DATE         NOT NULL UNIQUE,
    reflection         TEXT         NOT NULL DEFAULT '',
    blockers           TEXT[]       NOT NULL DEFAULT '{}',
    wins               TEXT[]       NOT NULL DEFAULT '{}',
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

-- At most one active/paused session per task — partial unique index makes
-- this enforceable at the database level under concurrent requests.
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
