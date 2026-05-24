-- 0001_create_tasks.down.sql

DROP TRIGGER  IF EXISTS trg_tasks_set_updated_at ON tasks;
DROP FUNCTION IF EXISTS tasks_set_updated_at();
DROP INDEX    IF EXISTS idx_tasks_category;
DROP INDEX    IF EXISTS idx_tasks_due_date;
DROP INDEX    IF EXISTS idx_tasks_status_due;
DROP TABLE    IF EXISTS tasks;
