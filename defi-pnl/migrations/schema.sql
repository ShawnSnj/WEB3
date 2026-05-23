-- defi-pnl: consolidated greenfield schema.
--
-- Single source of truth for the database shape. Every object is dropped
-- before it is recreated, so this file is safe to re-run against any
-- database (it will wipe data, so only use it on dev / fresh installs).
--
-- Run:
--   psql "$DATABASE_URL" -f migrations/schema.sql

-- ---------------------------------------------------------------------------
-- daily_leaderboard
--   Per-day top wallets by USD volume.
--   tx_type: 'U' = Swap.origin aggregate, 'B' = Swap.sender aggregate.
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS daily_leaderboard CASCADE;
CREATE TABLE daily_leaderboard (
    date         date              NOT NULL,
    tx_address   text              NOT NULL,
    volume       double precision  NOT NULL,
    tx_type      char(1)           NOT NULL CHECK (tx_type IN ('U', 'B')),
    CONSTRAINT daily_leaderboard_date_tx_type_unique UNIQUE (date, tx_address, tx_type)
);

CREATE INDEX daily_leaderboard_date_idx
    ON daily_leaderboard (date);

CREATE INDEX daily_leaderboard_date_tx_type_idx
    ON daily_leaderboard (date, tx_type);

-- ---------------------------------------------------------------------------
-- trades_v2
--   Raw per-swap rows ingested from the Uniswap V3 subgraph. Keyed by
--   (tx_hash, log_index) because a single tx can contain multiple swaps.
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS trades_v2 CASCADE;
CREATE TABLE trades_v2 (
    tx_hash     TEXT    NOT NULL,
    log_index   INT     NOT NULL,

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

CREATE INDEX trades_v2_timestamp_idx
    ON trades_v2 (timestamp);

CREATE INDEX trades_v2_day_idx
    ON trades_v2 ((timestamp::date));

-- ---------------------------------------------------------------------------
-- trade_flows view
--   Flattens trades_v2 into per-trade in/out token + amount columns.
-- ---------------------------------------------------------------------------
DROP VIEW IF EXISTS trade_flows;
CREATE VIEW trade_flows AS
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

-- ---------------------------------------------------------------------------
-- pnl_leaderboard
--   Daily snapshot of top wallets+tokens by realized USD PnL over the
--   trailing 7 days. Populated by the daily PnL job:
--     roi   = (sell_usd - buy_usd) / NULLIF(buy_usd, 0)
--     score = pnl * LOG(1 + trade_count) * LEAST(roi, 3)
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS pnl_leaderboard CASCADE;
CREATE TABLE pnl_leaderboard (
    stat_date   DATE,
    wallet      TEXT,
    token       TEXT,
    roi         NUMERIC,
    pnl         NUMERIC,
    trade_count INTEGER,
    score       NUMERIC,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX pnl_leaderboard_stat_date_idx
    ON pnl_leaderboard (stat_date);

-- ---------------------------------------------------------------------------
-- subscribers
--   Telegram subscribers: one row per chat_id. The bot upserts on /start
--   and updates tx_hash on /paid; an out-of-band verifier flips status to
--   'active' once the on-chain transfer is confirmed.
--
--   payer_wallet  — the on-chain address the user actually paid from. Often
--                   different from wallet_address (e.g. a CEX withdrawal
--                   address). Populated by the payment verifier.
--   updated_at    — bumped whenever the row meaningfully changes (e.g. user
--                   submits a new /paid tx_hash); distinct from created_at.
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS subscribers CASCADE;
CREATE TABLE subscribers (
    id               SERIAL PRIMARY KEY,
    telegram_chat_id BIGINT UNIQUE,
    wallet_address   TEXT,
    payer_wallet     TEXT,
    tx_hash          TEXT,
    plan             TEXT      DEFAULT 'pro',
    amount_usd       NUMERIC,
    status           TEXT      DEFAULT 'inactive',
    start_at         TIMESTAMP,
    expire_at        TIMESTAMP,
    created_at       TIMESTAMP DEFAULT NOW(),
    updated_at       TIMESTAMP
);

-- ---------------------------------------------------------------------------
-- pushed_signals
--   Every swap we've already alerted subscribers about, keyed by
--   (tx_hash, log_index) so the same on-chain swap is never pushed twice.
--   created_at is set explicitly by the alerts job to the run-start
--   timestamp, which doubles as the per-wallet watermark for the next
--   subgraph query (timestamp_gt = MAX(created_at) WHERE wallet = …).
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS pushed_signals CASCADE;
CREATE TABLE pushed_signals (
    tx_hash     TEXT      NOT NULL,
    log_index   INT       NOT NULL,
    wallet      TEXT,
    token0      TEXT,
    token1      TEXT,
    side        TEXT,
    amount0_in  NUMERIC,
    amount0_out NUMERIC,
    amount1_in  NUMERIC,
    amount1_out NUMERIC,
    amount_usd  NUMERIC,
    timestamp   TIMESTAMP,
    created_at  TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (tx_hash, log_index)
);

CREATE INDEX pushed_signals_wallet_created_at_idx
    ON pushed_signals (wallet, created_at DESC);



INSERT INTO subscribers (telegram_chat_id,wallet_address,payer_wallet,tx_hash,plan,amount_usd,status,start_at,expire_at,created_at,updated_at) VALUES
	 (7005712018,'0x6239B188906f85bF63D0159E5F691389DA740c78',NULL,'dddddddddeeeeeeeeee','pro',10,'active','2026-05-01 00:00:00','2026-05-31 00:00:00','2026-05-01 09:49:39.133544','2026-05-01 17:53:41.453026');