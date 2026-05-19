-- Telegram subscribers: one row per chat_id. The bot upserts on /start and
-- updates tx_hash on /paid; an out-of-band verifier flips status to 'active'
-- once the on-chain transfer is confirmed.
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
