-- View that flattens trades_v2 into per-trade in/out token + amount columns.
-- Re-run safe (CREATE OR REPLACE).
CREATE OR REPLACE VIEW trade_flows AS
SELECT
    wallet,

    CASE
        WHEN amount0_in > 0 THEN token0
        ELSE token1
    END AS token_in,

    CASE
        WHEN amount0_in > 0 THEN token1
        ELSE token0
    END AS token_out,

    CASE
        WHEN amount0_in > 0 THEN amount0_in
        ELSE amount1_in
    END AS amount_in,

    CASE
        WHEN amount0_in > 0 THEN amount1_out
        ELSE amount0_out
    END AS amount_out,

    amount_usd,
    side,
    timestamp
FROM trades_v2;

-- Daily snapshot of the top wallets+tokens by realized USD PnL over the trailing 7 days.
CREATE TABLE IF NOT EXISTS pnl_leaderboard (
    stat_date   DATE,
    wallet      TEXT,
    token       TEXT,
    roi         NUMERIC,
    pnl         NUMERIC,
    trade_count INTEGER,
    score       NUMERIC,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS pnl_leaderboard_stat_date_idx
    ON pnl_leaderboard (stat_date);
