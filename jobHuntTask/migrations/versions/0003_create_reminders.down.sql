-- 0003_create_reminders.down.sql

DROP TRIGGER  IF EXISTS trg_reminders_set_updated_at ON reminders;
DROP FUNCTION IF EXISTS reminders_set_updated_at();
DROP INDEX    IF EXISTS idx_reminders_kind_status;
DROP INDEX    IF EXISTS idx_reminders_due;
DROP INDEX    IF EXISTS uniq_reminders_dedup_key;
DROP TABLE    IF EXISTS reminders;
