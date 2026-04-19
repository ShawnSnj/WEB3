-- Return all rows from daily_leaderboard (newest days first).
-- Run: psql "$DATABASE_URL" -f migrations/daily_leaderboard_select_all.sql

SELECT
    date,
    tx_address,
    volume,
    tx_type
FROM daily_leaderboard
ORDER BY date DESC, tx_type ASC, volume DESC;
