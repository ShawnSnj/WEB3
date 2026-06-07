-- 0007_create_task_notes.up.sql
-- Per-task notes: multiple free-form notes linked to a task.

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
