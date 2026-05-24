ALTER TABLE daily_reviews
    DROP COLUMN IF EXISTS distractions,
    DROP COLUMN IF EXISTS notes;
