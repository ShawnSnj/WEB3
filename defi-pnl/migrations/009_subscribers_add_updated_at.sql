-- updated_at: bumped whenever the subscriber row meaningfully changes
-- (e.g. user submits a new /paid tx_hash). Distinct from created_at so we
-- can tell when a subscriber last did something even after their initial
-- /start signup.
ALTER TABLE subscribers
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;
