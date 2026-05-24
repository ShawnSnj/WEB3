-- 0003_create_reminders.up.sql
--
-- Reminders table used by the scheduler + reminder service.
--
-- Design:
--   * `kind` is constrained to a known set via CHECK so the DB rejects typos.
--   * `status` is the lifecycle: pending -> sent | failed | cancelled.
--     `failed` may transition back to `pending` (the retry queue), so it is
--     NOT a terminal state.
--   * `dedup_key` is the application-defined string used to prevent duplicate
--     reminders for the same logical slot (e.g. "morning:2026-05-24"). A
--     partial unique index enforces uniqueness only where dedup_key IS NOT NULL
--     so manually-created reminders without a key can coexist.
--   * `payload` is jsonb to give us schema flexibility per kind.

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

-- Partial unique index: at most one reminder per dedup_key.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_reminders_dedup_key
    ON reminders (dedup_key)
    WHERE dedup_key IS NOT NULL;

-- Accelerates the dispatcher's "what's due?" query.
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
