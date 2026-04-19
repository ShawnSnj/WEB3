-- Greenfield: create daily_leaderboard (matches defi-pnl storage layer).
-- tx_type: U = Swap.origin aggregate, B = Swap.sender aggregate.
-- Run: psql "$DATABASE_URL" -f migrations/000_daily_leaderboard_create.sql

CREATE TABLE IF NOT EXISTS daily_leaderboard (
    date         date        NOT NULL,
    tx_address   text        NOT NULL,
    volume       double precision NOT NULL,
    tx_type      char(1)     NOT NULL CHECK (tx_type IN ('U', 'B')),
    CONSTRAINT daily_leaderboard_date_tx_type_unique UNIQUE (date, tx_address, tx_type)
);

CREATE INDEX IF NOT EXISTS daily_leaderboard_date_idx
    ON daily_leaderboard (date);

CREATE INDEX IF NOT EXISTS daily_leaderboard_date_tx_type_idx
    ON daily_leaderboard (date, tx_type);

-- ---------------------------------------------------------------------------
-- Sample seed: one INSERT, all 20 rows (10 × U + 10 × B). Remove in production.
-- ---------------------------------------------------------------------------

INSERT INTO daily_leaderboard (date, tx_address, volume, tx_type) VALUES
    ('2026-04-11', '0x1111111111111111111111111111111111111111', 1250000.50, 'U'),
    ('2026-04-11', '0x2222222222222222222222222222222222222222', 980000.25,  'U'),
    ('2026-04-11', '0x3333333333333333333333333333333333333333', 875432.10,  'U'),
    ('2026-04-11', '0x4444444444444444444444444444444444444444', 654321.00,  'U'),
    ('2026-04-11', '0x5555555555555555555555555555555555555555', 543210.99,  'U'),
    ('2026-04-11', '0x6666666666666666666666666666666666666666', 432100.00,  'U'),
    ('2026-04-11', '0x7777777777777777777777777777777777777777', 321000.50,  'U'),
    ('2026-04-11', '0x8888888888888888888888888888888888888888', 210987.65,  'U'),
    ('2026-04-11', '0x9999999999999999999999999999999999999999', 109876.54,  'U'),
    ('2026-04-11', '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 50000.00,   'U'),
    ('2026-04-11', '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 2500000.00, 'B'),
    ('2026-04-11', '0xcccccccccccccccccccccccccccccccccccccccc', 1800000.75, 'B'),
    ('2026-04-11', '0xdddddddddddddddddddddddddddddddddddddddd', 1200000.00, 'B'),
    ('2026-04-11', '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 950000.50,  'B'),
    ('2026-04-11', '0xffffffffffffffffffffffffffffffffffffffff', 800000.00,  'B'),
    ('2026-04-11', '0x1010101010101010101010101010101010101010', 600000.25,  'B'),
    ('2026-04-11', '0x2020202020202020202020202020202020202020', 450000.00,  'B'),
    ('2026-04-11', '0x3030303030303030303030303030303030303030', 300000.00,  'B'),
    ('2026-04-11', '0x4040404040404040404040404040404040404040', 150000.50,  'B'),
    ('2026-04-11', '0x5050505050505050505050505050505050505050', 75000.25,   'B')
ON CONFLICT (date, tx_address, tx_type) DO NOTHING;
