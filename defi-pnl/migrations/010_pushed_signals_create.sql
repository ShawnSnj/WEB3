-- pushed_signals: every swap we've already alerted subscribers about, keyed
-- by (tx_hash, log_index) so the same on-chain swap is never pushed twice.
-- Mirrors trades_v2's swap shape; `side` is computed at insert time using the
-- same BUY/SELL/SWAP rules so consumers can read it directly without joining.
--
-- created_at is set explicitly by the alerts job to the run-start timestamp,
-- which doubles as the per-wallet watermark for the next subgraph query
-- (`timestamp_gt = MAX(created_at) WHERE wallet=…`).
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

-- Speeds up the per-wallet watermark lookup
-- (SELECT MAX(created_at) FROM pushed_signals WHERE wallet = $1).
CREATE INDEX IF NOT EXISTS pushed_signals_wallet_created_at_idx
    ON pushed_signals (wallet, created_at DESC);
