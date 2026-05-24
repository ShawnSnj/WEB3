DROP TRIGGER IF EXISTS trg_weekly_reviews_set_updated_at ON weekly_reviews;
DROP FUNCTION IF EXISTS weekly_reviews_set_updated_at();
DROP INDEX IF EXISTS idx_weekly_reviews_week_start_desc;
DROP TABLE IF EXISTS weekly_reviews;
