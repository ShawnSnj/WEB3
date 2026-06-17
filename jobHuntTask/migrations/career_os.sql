-- career_os.sql — Career OS intelligence layer schema (idempotent)
-- Apply: make migrate-career-os

BEGIN;

-- ---------------------------------------------------------------------------
-- user_skills — skill graph with proficiency levels (1-10)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS user_skills (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    skill       TEXT         NOT NULL,
    level       SMALLINT     NOT NULL DEFAULT 5,
    category    TEXT         NOT NULL DEFAULT 'general',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT user_skills_skill_unique UNIQUE (skill),
    CONSTRAINT user_skills_level_range CHECK (level >= 1 AND level <= 10)
);

-- Seed default skill graph for senior backend engineer
INSERT INTO user_skills (skill, level, category) VALUES
    ('Go', 8, 'language'),
    ('Java', 7, 'language'),
    ('Kafka', 7, 'messaging'),
    ('SQL', 8, 'data'),
    ('PostgreSQL', 8, 'data'),
    ('Distributed Systems', 8, 'architecture'),
    ('Cloud', 6, 'infra'),
    ('AWS', 4, 'infra'),
    ('Docker', 7, 'infra'),
    ('Kubernetes', 3, 'infra'),
    ('Terraform', 1, 'infra'),
    ('gRPC', 7, 'architecture'),
    ('Redis', 6, 'data'),
    ('Web3', 2, 'domain'),
    ('Blockchain', 2, 'domain')
ON CONFLICT (skill) DO NOTHING;

-- ---------------------------------------------------------------------------
-- market_snapshots — daily career intelligence (Layer 2)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS market_snapshots (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_date       DATE         NOT NULL UNIQUE,
    skill_demand        JSONB        NOT NULL DEFAULT '[]'::jsonb,
    trending_tech       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    interview_topics    JSONB        NOT NULL DEFAULT '[]'::jsonb,
    salary_combos       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    jobs_analyzed       INTEGER      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- company_profiles — enriched company intelligence
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS company_profiles (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID         REFERENCES companies(id) ON DELETE CASCADE,
    company_name        TEXT         NOT NULL,
    quality_score       SMALLINT     NOT NULL DEFAULT 50,
    growth_score        SMALLINT     NOT NULL DEFAULT 50,
    interview_topics    TEXT[]       NOT NULL DEFAULT '{}',
    tech_stack          TEXT[]       NOT NULL DEFAULT '{}',
    is_target           BOOLEAN      NOT NULL DEFAULT FALSE,
    notes               TEXT         NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT company_profiles_name_unique UNIQUE (company_name)
);

INSERT INTO company_profiles (company_name, quality_score, growth_score, interview_topics, tech_stack, is_target) VALUES
    ('Grafana Labs', 90, 85, ARRAY['Prometheus','Observability','Go concurrency','Distributed tracing'], ARRAY['Go','Kubernetes','Prometheus'], TRUE),
    ('Confluent', 92, 88, ARRAY['Kafka internals','Stream processing','Consumer groups','Exactly-once'], ARRAY['Kafka','Java','Flink'], TRUE),
    ('GitLab', 88, 80, ARRAY['Rails/Go hybrid','CI/CD at scale','PostgreSQL tuning','Kubernetes'], ARRAY['Go','Ruby','PostgreSQL','K8s'], TRUE),
    ('Cloudflare', 95, 90, ARRAY['Edge computing','Rust/Go systems','DDoS mitigation','Global load balancing'], ARRAY['Go','Rust','eBPF'], TRUE),
    ('Chainlink', 85, 82, ARRAY['Oracle networks','Solidity basics','Node operators','Decentralization'], ARRAY['Go','Solidity','Web3'], TRUE),
    ('Alchemy', 87, 84, ARRAY['Blockchain infra','RPC scaling','Web3 APIs','Node management'], ARRAY['Go','Node.js','Web3'], TRUE),
    ('Nethermind', 83, 78, ARRAY['Ethereum client','EVM internals','Sync performance','P2P networking'], ARRAY['C#','Go','Ethereum'], TRUE)
ON CONFLICT (company_name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- interview_readiness — per-company readiness (Layer 4)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS interview_readiness (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_name        TEXT         NOT NULL,
    readiness_score     SMALLINT     NOT NULL DEFAULT 0,
    target_score        SMALLINT     NOT NULL DEFAULT 80,
    missing_topics      TEXT[]       NOT NULL DEFAULT '{}',
    study_topics        TEXT[]       NOT NULL DEFAULT '{}',
    analyzed_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT interview_readiness_company_unique UNIQUE (company_name),
    CONSTRAINT interview_readiness_score_range CHECK (readiness_score >= 0 AND readiness_score <= 100)
);

-- ---------------------------------------------------------------------------
-- offer_predictions — funnel analysis (Layer 5)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS offer_predictions (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    prediction_date     DATE         NOT NULL UNIQUE,
    interview_prob      NUMERIC(5,2) NOT NULL DEFAULT 0,
    offer_prob          NUMERIC(5,2) NOT NULL DEFAULT 0,
    bottleneck          TEXT         NOT NULL DEFAULT '',
    recommendations     TEXT[]       NOT NULL DEFAULT '{}',
    funnel_stats        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Extend job_matches — company quality + growth scores (Layer 1)
-- ---------------------------------------------------------------------------

ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS company_score SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE job_matches ADD COLUMN IF NOT EXISTS growth_score SMALLINT NOT NULL DEFAULT 0;

-- ---------------------------------------------------------------------------
-- Extend daily_briefs — mission completeness (Layer 6)
-- ---------------------------------------------------------------------------

ALTER TABLE daily_briefs ADD COLUMN IF NOT EXISTS interview_topic TEXT NOT NULL DEFAULT '';
ALTER TABLE daily_briefs ADD COLUMN IF NOT EXISTS interview_context TEXT NOT NULL DEFAULT '';
ALTER TABLE daily_briefs ADD COLUMN IF NOT EXISTS estimated_minutes SMALLINT NOT NULL DEFAULT 30;
ALTER TABLE daily_briefs ADD COLUMN IF NOT EXISTS learning_topic TEXT NOT NULL DEFAULT '';
ALTER TABLE daily_briefs ADD COLUMN IF NOT EXISTS mission_payload JSONB NOT NULL DEFAULT '{}'::jsonb;

-- ---------------------------------------------------------------------------
-- Extend skill_analyses — gap scores with ROI
-- ---------------------------------------------------------------------------

ALTER TABLE skill_analyses ADD COLUMN IF NOT EXISTS skill_gaps JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMIT;
