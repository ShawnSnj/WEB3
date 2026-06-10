-- defi-pnl: consolidated schema.
--
-- Idempotent deployment script. Edit this file when the database shape changes,
-- then re-apply with:
--   make integrate
--
-- - New databases: CREATE TABLE IF NOT EXISTS creates missing tables.
-- - Existing databases: the upgrades section adds any missing columns.
-- - Views: CREATE OR REPLACE updates the definition without dropping data.
-- - No DROP statements — existing rows are preserved.

-- ---------------------------------------------------------------------------
-- daily_leaderboard
--   Per-day top wallets by USD volume.
--   tx_type: 'U' = Swap.origin aggregate, 'B' = Swap.sender aggregate.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS daily_leaderboard (
    date         date              NOT NULL,
    tx_address   text              NOT NULL,
    volume       double precision  NOT NULL,
    tx_type      char(1)           NOT NULL CHECK (tx_type IN ('U', 'B')),
    CONSTRAINT daily_leaderboard_date_tx_type_unique UNIQUE (date, tx_address, tx_type)
);

CREATE INDEX IF NOT EXISTS daily_leaderboard_date_idx
    ON daily_leaderboard (date);

CREATE INDEX IF NOT EXISTS daily_leaderboard_date_tx_type_idx
    ON daily_leaderboard (date, tx_type);

-- ---------------------------------------------------------------------------
-- trades_v2
--   Raw per-swap rows ingested from the Uniswap V3 subgraph. Keyed by
--   (tx_hash, log_index) because a single tx can contain multiple swaps.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS trades_v2 (
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

CREATE INDEX IF NOT EXISTS trades_v2_timestamp_idx
    ON trades_v2 (timestamp);

CREATE INDEX IF NOT EXISTS trades_v2_day_idx
    ON trades_v2 ((timestamp::date));

-- ---------------------------------------------------------------------------
-- pnl_leaderboard
--   Daily snapshot of top wallets+tokens by realized USD PnL over the
--   trailing 7 days. Populated by the daily PnL job:
--     roi   = (sell_usd - buy_usd) / NULLIF(buy_usd, 0)
--     score = pnl * LOG(1 + trade_count) * LEAST(roi, 3)
-- ---------------------------------------------------------------------------
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

-- ---------------------------------------------------------------------------
-- subscribers
--   Telegram subscribers: one row per chat_id. The bot upserts on /start
--   and updates tx_hash on /paid; an out-of-band verifier flips status to
--   'active' once the on-chain transfer is confirmed.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS subscribers (
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
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pushed_signals (
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

CREATE INDEX IF NOT EXISTS pushed_signals_wallet_created_at_idx
    ON pushed_signals (wallet, created_at DESC);

-- ---------------------------------------------------------------------------
-- upgrades
--   Add new columns/indexes here when you extend an existing table, then run
--   make integrate. Safe to re-run (IF NOT EXISTS on every statement).
-- ---------------------------------------------------------------------------

ALTER TABLE daily_leaderboard ADD COLUMN IF NOT EXISTS tx_type char(1);

ALTER TABLE pnl_leaderboard ADD COLUMN IF NOT EXISTS roi         NUMERIC;
ALTER TABLE pnl_leaderboard ADD COLUMN IF NOT EXISTS trade_count INTEGER;
ALTER TABLE pnl_leaderboard ADD COLUMN IF NOT EXISTS score       NUMERIC;
ALTER TABLE pnl_leaderboard ADD COLUMN IF NOT EXISTS created_at  TIMESTAMP DEFAULT NOW();

ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS payer_wallet   TEXT;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS updated_at       TIMESTAMP;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS plan             TEXT      DEFAULT 'pro';
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS amount_usd       NUMERIC;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS status           TEXT      DEFAULT 'inactive';
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS start_at         TIMESTAMP;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS expire_at        TIMESTAMP;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS created_at       TIMESTAMP DEFAULT NOW();

ALTER TABLE pushed_signals ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();

-- ---------------------------------------------------------------------------
-- trade_flows view
--   Flattens trades_v2 into per-trade in/out token + amount columns.
--   CREATE OR REPLACE keeps the view in sync when the definition changes.
-- ---------------------------------------------------------------------------
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

-- ---------------------------------------------------------------------------
-- seed data (optional dev rows — skipped when already present)
-- ---------------------------------------------------------------------------
INSERT INTO subscribers (
    telegram_chat_id,
    wallet_address,
    payer_wallet,
    tx_hash,
    plan,
    amount_usd,
    status,
    start_at,
    expire_at,
    created_at,
    updated_at
) VALUES (
    7005712018,
    '0x6239B188906f85bF63D0159E5F691389DA740c78',
    NULL,
    'dddddddddeeeeeeeeee',
    'pro',
    10,
    'active',
    '2026-05-01 00:00:00',
    '2026-05-31 00:00:00',
    '2026-05-01 09:49:39.133544',
    '2026-05-01 17:53:41.453026'
)
ON CONFLICT (telegram_chat_id) DO NOTHING;
