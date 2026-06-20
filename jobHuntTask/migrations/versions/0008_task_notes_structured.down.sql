DROP INDEX IF EXISTS idx_task_notes_sent_at;
DROP INDEX IF EXISTS idx_task_notes_applied_at;
DROP INDEX IF EXISTS idx_task_notes_company;
DROP INDEX IF EXISTS idx_task_notes_note_type;

ALTER TABLE task_notes DROP COLUMN IF EXISTS notes;
ALTER TABLE task_notes DROP COLUMN IF EXISTS source;
ALTER TABLE task_notes DROP COLUMN IF EXISTS fit_score;
ALTER TABLE task_notes DROP COLUMN IF EXISTS resume_version;
ALTER TABLE task_notes DROP COLUMN IF EXISTS applied_at;
ALTER TABLE task_notes DROP COLUMN IF EXISTS application_status;
ALTER TABLE task_notes DROP COLUMN IF EXISTS job_url;
ALTER TABLE task_notes DROP COLUMN IF EXISTS job_title;
ALTER TABLE task_notes DROP COLUMN IF EXISTS reply_at;
ALTER TABLE task_notes DROP COLUMN IF EXISTS reply_status;
ALTER TABLE task_notes DROP COLUMN IF EXISTS sent_at;
ALTER TABLE task_notes DROP COLUMN IF EXISTS message_content;
ALTER TABLE task_notes DROP COLUMN IF EXISTS profile_url;
ALTER TABLE task_notes DROP COLUMN IF EXISTS platform;
ALTER TABLE task_notes DROP COLUMN IF EXISTS role_title;
ALTER TABLE task_notes DROP COLUMN IF EXISTS company;
ALTER TABLE task_notes DROP COLUMN IF EXISTS person_name;
ALTER TABLE task_notes DROP COLUMN IF EXISTS note_type;

ALTER TABLE task_notes ADD CONSTRAINT task_notes_title_not_blank
    CHECK (length(btrim(title)) > 0);
