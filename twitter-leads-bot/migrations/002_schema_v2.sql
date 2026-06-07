-- Drop the v1 single-table model in favor of the spec's three-table layout.
DROP TABLE IF EXISTS leads;

CREATE TABLE IF NOT EXISTS search_keywords (
    id               BIGSERIAL PRIMARY KEY,
    keyword          TEXT        NOT NULL UNIQUE,
    enabled          BOOLEAN     NOT NULL DEFAULT TRUE,
    last_searched_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS processed_tweets (
    tweet_id   TEXT        PRIMARY KEY,
    keyword    TEXT        NOT NULL,
    username   TEXT        NOT NULL,
    text       TEXT        NOT NULL,
    likes      INT         NOT NULL DEFAULT 0,
    -- created_at = the tweet's own timestamp (from the source).
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pt_keyword ON processed_tweets(keyword);
CREATE INDEX IF NOT EXISTS idx_pt_seen_at ON processed_tweets(seen_at DESC);

CREATE TABLE IF NOT EXISTS lead_analysis (
    tweet_id         TEXT        PRIMARY KEY REFERENCES processed_tweets(tweet_id) ON DELETE CASCADE,
    score            INT         NOT NULL,
    reason           TEXT        NOT NULL DEFAULT '',
    reply_suggestion TEXT        NOT NULL DEFAULT '',
    -- pending | approved | skipped | sent
    status           TEXT        NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_la_status ON lead_analysis(status);
CREATE INDEX IF NOT EXISTS idx_la_score  ON lead_analysis(score DESC);

-- Seed a few sensible defaults so the dashboard isn't empty on first boot.
INSERT INTO search_keywords (keyword) VALUES
    ('polymarket'),
    ('hyperliquid'),
    ('smart money'),
    ('copy trading'),
    ('whale alerts')
ON CONFLICT (keyword) DO NOTHING;
