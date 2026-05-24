-- 0002_create_reviews_and_sessions.down.sql

DROP TRIGGER  IF EXISTS trg_task_sessions_set_updated_at ON task_execution_sessions;
DROP FUNCTION IF EXISTS task_sessions_set_updated_at();
DROP INDEX    IF EXISTS idx_sessions_running;
DROP INDEX    IF EXISTS idx_sessions_by_task;
DROP INDEX    IF EXISTS uniq_sessions_active_per_task;
DROP TABLE    IF EXISTS task_execution_sessions;

DROP TRIGGER  IF EXISTS trg_daily_reviews_set_updated_at ON daily_reviews;
DROP FUNCTION IF EXISTS daily_reviews_set_updated_at();
DROP INDEX    IF EXISTS idx_daily_reviews_date_desc;
DROP TABLE    IF EXISTS daily_reviews;
