-- 0005_review_distractions_notes.up.sql
--
-- Extend daily_reviews with distractions (TEXT[], like blockers/wins) and
-- a free-form notes field for end-of-day capture.

ALTER TABLE daily_reviews
    ADD COLUMN IF NOT EXISTS distractions TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS notes         TEXT    NOT NULL DEFAULT '';
