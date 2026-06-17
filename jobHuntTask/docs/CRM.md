# AI Job Hunt CRM

Personal recruiter for senior backend engineers targeting remote Web3 and infrastructure roles.

## Architecture

```
Sources (RemoteOK, Greenhouse, Lever, Ashby, Web3…) 
    → Go aggregator (cron + manual)
    → PostgreSQL (job_postings)
    → Kafka (crm.jobs.ingested) → crm-worker scores jobs
    → job_matches + daily_briefs
    → Next.js dashboard (localhost:3000)
```

## Quick start

```bash
# Recommended — one database, one server process
make integrate

# Full docker (API + kafka worker + Next.js UI)
make integrate-up

# Existing postgres volume — apply all tables
make migrate-all
```

**Unified layout:** `jobhunt` database holds task tables (`tasks`, `daily_reviews`, …)
and CRM tables (`job_postings`, `applications`, …). Schema is in `migrations/deploy.sql`.

**Unified UI:** CRM is a static Next.js export **embedded in the Go binary** and served at
`/crm/` on the same port as the task tracker (8082). No separate `:3000` process in production.

Set `OPENAI_API_KEY` in `.env` for AI matching, resume analysis, outreach, and coaching. Without it, heuristic scoring still works.

## Daily automation

| Cron | Default | Action |
|------|---------|--------|
| `CRON_CRM_DAILY_PIPELINE` | `0 7 * * *` | Collect → score → skill gaps → daily brief |

Manual trigger: `POST /api/v1/crm/pipeline/run` or **Sync jobs** in the UI.

## API (`/api/v1/crm`)

| Endpoint | Description |
|----------|-------------|
| `GET /dashboard` | Today's apply + outreach + learning |
| `POST /pipeline/run` | Full daily pipeline |
| `POST /collect` | Fetch jobs from all sources |
| `GET /jobs` | Ranked listings (`?min_fit=80`) |
| `POST /jobs/:id/apply` | Track application |
| `POST /jobs/:id/resume` | Resume vs JD analysis |
| `GET /applications` | Pipeline tracker |
| `GET /skills` | Skill gap analysis |
| `GET /weekly` | Weekly CRM report |
| `GET /coach` | AI career coach summary |

## Job sources

| Source | Status |
|--------|--------|
| RemoteOK | Public API |
| Greenhouse | Board API (GitLab, HashiCorp, etc.) |
| Lever | Board API |
| Ashby | Board API |
| Web3 Career | API (when available) |
| CryptoJobsList | API (when available) |
| Wellfound | Seed + integration hook |
| LinkedIn | Stub (requires partnership API) |

## User profile

Seeded in `migrations/crm.sql` with Go/Java/Kafka/Web3 skills. Update via `PUT /api/v1/crm/profile`.
