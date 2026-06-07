CREATE TABLE IF NOT EXISTS leads (
    id              BIGSERIAL PRIMARY KEY,
    tweet_id        TEXT        NOT NULL UNIQUE,
    author          TEXT        NOT NULL,
    content         TEXT        NOT NULL,
    url             TEXT        NOT NULL,
    keyword         TEXT        NOT NULL,
    relevance_score INT         NOT NULL DEFAULT 0,
    reasoning       TEXT        NOT NULL DEFAULT '',
    suggested_reply TEXT        NOT NULL DEFAULT '',
    status          TEXT        NOT NULL DEFAULT 'new',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_leads_status     ON leads(status);
CREATE INDEX IF NOT EXISTS idx_leads_created_at ON leads(created_at DESC);
