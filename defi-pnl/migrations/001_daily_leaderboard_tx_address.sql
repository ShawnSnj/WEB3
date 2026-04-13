-- Run once against your database (e.g. psql defi_pnl < migrations/001_daily_leaderboard_tx_address.sql)
-- Renames user_address -> tx_address and adds tx_type (U = origin aggregate, B = sender aggregate).

ALTER TABLE daily_leaderboard RENAME COLUMN user_address TO tx_address;

ALTER TABLE daily_leaderboard ADD COLUMN tx_type CHAR(1) NOT NULL DEFAULT 'U';

-- Drop legacy unique constraint on (date, tx_address) if present (name may differ — adjust in psql \d daily_leaderboard)
ALTER TABLE daily_leaderboard DROP CONSTRAINT IF EXISTS daily_leaderboard_date_user_address_key;
ALTER TABLE daily_leaderboard DROP CONSTRAINT IF EXISTS daily_leaderboard_date_tx_address_key;

ALTER TABLE daily_leaderboard ADD CONSTRAINT daily_leaderboard_date_tx_type_unique UNIQUE (date, tx_address, tx_type);
