-- Add roi column to pnl_leaderboard for existing databases.
-- roi = (sell_usd - buy_usd) / NULLIF(buy_usd, 0), populated by the daily job.
ALTER TABLE pnl_leaderboard
    ADD COLUMN IF NOT EXISTS roi NUMERIC;
