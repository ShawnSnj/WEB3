-- 0001_create_tasks.up.sql
--
-- Core task table for the job-hunt tracker.
--
-- Design notes:
--   * UUID primary key (gen_random_uuid) — externally safe.
--   * Status / priority / category are stored as TEXT with CHECK constraints.
--     Native PG enums are hard to evolve; CHECKs are trivial to ALTER.
--   * updated_at is auto-maintained by a trigger so service code never
--     forgets to bump it.
--   * Indexes target the actual query patterns of the service:
--       - list by status + due_date (today / dashboard)
--       - list overdue (due_date < now AND status IN (pending, in_progress))

CREATE EXTENSION IF NOT EXISTS pgcrypto;

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
