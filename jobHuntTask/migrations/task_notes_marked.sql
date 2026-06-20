-- Pin one task note to the tasks page header.
-- Run: make migrate-task-notes-marked

ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS is_marked BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_task_notes_is_marked
    ON task_notes (is_marked)
    WHERE is_marked = TRUE;
