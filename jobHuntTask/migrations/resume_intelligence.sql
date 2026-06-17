-- resume_intelligence.sql — Phase 1: Candidate Master Profile foundation
-- Run: make migrate-resume-intelligence

BEGIN;

-- ---------------------------------------------------------------------------
-- resume_documents — raw EN/ZH resume uploads
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS resume_documents (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    language     TEXT         NOT NULL CHECK (language IN ('en', 'zh')),
    filename     TEXT         NOT NULL DEFAULT '',
    raw_text     TEXT         NOT NULL DEFAULT '',
    content_type TEXT         NOT NULL DEFAULT 'text/plain',
    parsed_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_resume_documents_lang
    ON resume_documents (language, created_at DESC);

-- ---------------------------------------------------------------------------
-- candidate_profiles — merged structured profile (source of truth)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS candidate_profiles (
    id                              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_profile_id                 UUID         REFERENCES user_profile(id) ON DELETE SET NULL,
    full_name                       TEXT         NOT NULL DEFAULT '',
    location                        TEXT         NOT NULL DEFAULT '',
    email                           TEXT         NOT NULL DEFAULT '',
    years_of_experience             INTEGER      NOT NULL DEFAULT 0,
    seniority_level                 TEXT         NOT NULL DEFAULT 'Senior',
    target_roles                    TEXT[]       NOT NULL DEFAULT '{}',
    target_regions                  TEXT[]       NOT NULL DEFAULT '{}',
    target_compensation_min_usd     INTEGER      NOT NULL DEFAULT 150000,
    strongest_skills                TEXT[]       NOT NULL DEFAULT '{}',
    medium_skills                   TEXT[]       NOT NULL DEFAULT '{}',
    weak_skills                     TEXT[]       NOT NULL DEFAULT '{}',
    backend_skills                  TEXT[]       NOT NULL DEFAULT '{}',
    web3_skills                     TEXT[]       NOT NULL DEFAULT '{}',
    infra_skills                    TEXT[]       NOT NULL DEFAULT '{}',
    database_skills                 TEXT[]       NOT NULL DEFAULT '{}',
    messaging_skills                TEXT[]       NOT NULL DEFAULT '{}',
    leadership_experience           TEXT[]       NOT NULL DEFAULT '{}',
    payment_experience              TEXT[]       NOT NULL DEFAULT '{}',
    web3_experience                 TEXT[]       NOT NULL DEFAULT '{}',
    distributed_systems_experience    TEXT[]       NOT NULL DEFAULT '{}',
    major_achievements              TEXT[]       NOT NULL DEFAULT '{}',
    quantified_results              TEXT[]       NOT NULL DEFAULT '{}',
    resume_keywords                 TEXT[]       NOT NULL DEFAULT '{}',
    company_domain_experience       TEXT[]       NOT NULL DEFAULT '{}',
    preferred_job_types             TEXT[]       NOT NULL DEFAULT '{}',
    domains                         TEXT[]       NOT NULL DEFAULT '{}',
    structured_profile              JSONB        NOT NULL DEFAULT '{}'::jsonb,
    source_en_resume_id             UUID         REFERENCES resume_documents(id) ON DELETE SET NULL,
    source_zh_resume_id               UUID         REFERENCES resume_documents(id) ON DELETE SET NULL,
    last_parsed_at                    TIMESTAMPTZ,
    created_at                        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_candidate_profiles_singleton
    ON candidate_profiles ((true));

-- ---------------------------------------------------------------------------
-- candidate_skills — normalized skill graph linked to profile
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS candidate_skills (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id   UUID         NOT NULL REFERENCES candidate_profiles(id) ON DELETE CASCADE,
    skill_name   TEXT         NOT NULL,
    category     TEXT         NOT NULL DEFAULT 'general',
    level        SMALLINT     NOT NULL DEFAULT 5 CHECK (level >= 1 AND level <= 10),
    strength     TEXT         NOT NULL DEFAULT 'medium' CHECK (strength IN ('strong', 'medium', 'weak')),
    source       TEXT         NOT NULL DEFAULT 'manual',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT candidate_skills_unique UNIQUE (profile_id, skill_name)
);

CREATE INDEX IF NOT EXISTS idx_candidate_skills_profile
    ON candidate_skills (profile_id, strength);

-- Seed default candidate profile from existing user_profile if none exists
INSERT INTO candidate_profiles (
    user_profile_id, full_name, years_of_experience, seniority_level,
    target_roles, target_compensation_min_usd, strongest_skills, domains
)
SELECT
    up.id,
    up.display_name,
    15,
    'Senior',
    up.target_titles,
    GREATEST(up.min_salary_usd, 150000),
    ARRAY['Java', 'Go', 'Kafka', 'SQL', 'Payment Systems'],
    ARRAY['Payments', 'Web3', 'Backend Infrastructure']
FROM user_profile up
WHERE NOT EXISTS (SELECT 1 FROM candidate_profiles LIMIT 1)
LIMIT 1;

COMMIT;
