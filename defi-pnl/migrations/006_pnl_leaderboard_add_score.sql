-- Add trade_count and score columns to pnl_leaderboard.
-- score = pnl * LOG(1 + trade_count) * LEAST(roi, 3), populated by the daily job.
ALTER TABLE pnl_leaderboard
    ADD COLUMN IF NOT EXISTS trade_count INTEGER,
    ADD COLUMN IF NOT EXISTS score       NUMERIC;
