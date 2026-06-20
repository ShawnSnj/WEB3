-- Structured task note types for job hunt progress tracking.
-- Run: make migrate-task-notes-structured

ALTER TABLE task_notes DROP CONSTRAINT IF EXISTS task_notes_title_not_blank;

ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS note_type TEXT NOT NULL DEFAULT 'GENERAL_NOTE';
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS person_name TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS company TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS role_title TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS platform TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS profile_url TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS message_content TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS reply_status TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS reply_at TIMESTAMPTZ;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS job_title TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS job_url TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS application_status TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS applied_at TIMESTAMPTZ;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS resume_version TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS fit_score INTEGER;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS source TEXT;
ALTER TABLE task_notes ADD COLUMN IF NOT EXISTS notes TEXT;

UPDATE task_notes
SET note_type = 'GENERAL_NOTE',
    notes = COALESCE(NULLIF(btrim(notes), ''), content)
WHERE btrim(COALESCE(notes, '')) = '' AND btrim(COALESCE(content, '')) <> '';

CREATE INDEX IF NOT EXISTS idx_task_notes_note_type ON task_notes (note_type);
CREATE INDEX IF NOT EXISTS idx_task_notes_company ON task_notes (company);
CREATE INDEX IF NOT EXISTS idx_task_notes_applied_at ON task_notes (applied_at);
CREATE INDEX IF NOT EXISTS idx_task_notes_sent_at ON task_notes (sent_at);
