DROP INDEX IF EXISTS idx_task_notes_is_marked;
ALTER TABLE task_notes DROP COLUMN IF EXISTS is_marked;
