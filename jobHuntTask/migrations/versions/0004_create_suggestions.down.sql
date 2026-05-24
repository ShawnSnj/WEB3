-- 0004_create_suggestions.down.sql

DROP TRIGGER  IF EXISTS trg_suggestions_set_updated_at ON suggestions;
DROP FUNCTION IF EXISTS suggestions_set_updated_at();
DROP INDEX    IF EXISTS idx_suggestions_kind;
DROP INDEX    IF EXISTS idx_suggestions_status_generated;
DROP INDEX    IF EXISTS uniq_suggestions_dedup;
DROP TABLE    IF EXISTS suggestions;
