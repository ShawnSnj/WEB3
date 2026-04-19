-- Export every row in daily_leaderboard as INSERT ... ON CONFLICT DO NOTHING.
-- Run in psql and save stdout, e.g.:
--   psql "$DATABASE_URL" -f migrations/daily_leaderboard_export_inserts.sql -t -A > seed_from_db.sql
--
-- Or in psql: \o seed_from_db.sql  then run the SELECT below, then \o

\o migrations/seed_from_db.sql 
SELECT format(
    'INSERT INTO daily_leaderboard (date, tx_address, volume, tx_type) VALUES (%L, %L, %s, %L) ON CONFLICT (date, tx_address, tx_type) DO NOTHING;',
    date,
    tx_address,
    volume,
    tx_type
) AS insert_statement
FROM daily_leaderboard
ORDER BY date, tx_type, volume DESC;
