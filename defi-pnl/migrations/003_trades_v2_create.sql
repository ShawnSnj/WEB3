-- Drop the old trades table (schema superseded by trades_v2).
-- Comment out the DROP if you need to keep historical rows.
DROP TABLE IF EXISTS trades;

CREATE TABLE IF NOT EXISTS trades_v2 (
    tx_hash     TEXT    NOT NULL,
    log_index   INT     NOT NULL, -- same tx can contain multiple swaps

    wallet      TEXT,

    token0      TEXT,
    token1      TEXT,
    side        TEXT,             -- BUY / SELL / UNKNOWN
    amount0_in  NUMERIC,
    amount0_out NUMERIC,
    amount1_in  NUMERIC,
    amount1_out NUMERIC,

    amount_usd  NUMERIC,
    timestamp   TIMESTAMP,

    PRIMARY KEY (tx_hash, log_index)
);

CREATE INDEX IF NOT EXISTS trades_v2_timestamp_idx
    ON trades_v2 (timestamp);

CREATE INDEX IF NOT EXISTS trades_v2_day_idx
    ON trades_v2 ((timestamp::date));
