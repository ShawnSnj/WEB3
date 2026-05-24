-- 0006_create_weekly_reviews.up.sql
--
-- One weekly review per rolling-window start date (Monday-normalised in
-- application code, stored as DATE at UTC midnight).

CREATE TABLE IF NOT EXISTS weekly_reviews (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    week_start           DATE         NOT NULL,
    wins                 TEXT         NOT NULL DEFAULT '',
    bottlenecks          TEXT         NOT NULL DEFAULT '',
    improvement_notes    TEXT         NOT NULL DEFAULT '',
    next_week_priorities TEXT         NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT weekly_reviews_week_start_unique UNIQUE (week_start)
);

CREATE INDEX IF NOT EXISTS idx_weekly_reviews_week_start_desc
    ON weekly_reviews (week_start DESC);

CREATE OR REPLACE FUNCTION weekly_reviews_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_weekly_reviews_set_updated_at ON weekly_reviews;
CREATE TRIGGER trg_weekly_reviews_set_updated_at
    BEFORE UPDATE ON weekly_reviews
    FOR EACH ROW
    EXECUTE FUNCTION weekly_reviews_set_updated_at();
