-- Create trades table populated from Uniswap V3 subgraph swaps.
-- Run: psql "$DATABASE_URL" -f migrations/002_trades_create.sql

CREATE TABLE IF NOT EXISTS trades (
    id          SERIAL PRIMARY KEY,
    wallet      TEXT,
    token       TEXT,
    side        TEXT, -- BUY / SELL
    amount      NUMERIC,
    token0In    NUMERIC,
    token1Out   NUMERIC,
    value_usd   NUMERIC,
    timestamp   TIMESTAMP
);

CREATE INDEX IF NOT EXISTS trades_timestamp_idx
    ON trades (timestamp);

CREATE INDEX IF NOT EXISTS trades_day_idx
    ON trades ((timestamp::date));
