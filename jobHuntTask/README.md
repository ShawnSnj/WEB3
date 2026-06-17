# Job Hunt Task — Tracking & Reminder System

A production-grade Go application for planning, executing, and reviewing your
daily job-hunt routine. It combines a REST API, background scheduler, rule-based
coaching suggestions, and a server-rendered HTMX web UI — all backed by
PostgreSQL.

**Default URL:** [http://localhost:8082](http://localhost:8082) (task tracker + CRM at [/crm/](http://localhost:8082/crm/))

---

## AI Job Hunt CRM

A personal recruiter layer on top of the task tracker: automatic job collection,
AI/heuristic fit scoring, daily brief (1 apply + 2 outreach + 1 skill), application
pipeline, resume analysis, skill gaps, weekly review, and career coach.

See **[docs/CRM.md](docs/CRM.md)** for full architecture and API.

```bash
make integrate       # ONE command: postgres + kafka + migrate + unified server
make integrate-up    # full docker stack (API + worker + CRM UI)
make migrate-all     # apply tasks + CRM tables to existing postgres
make crm-collect     # fetch & score jobs
make frontend-dev    # Next.js CRM UI (API already on :8082)
```

Both systems share **one PostgreSQL database** (`jobhunt`) with separate table groups.
One Go process (`cmd/server`) serves the task tracker UI, REST API, and CRM API.

Set `OPENAI_API_KEY` for GPT-powered matching and outreach (heuristics work without it).

---

## Stack

| Concern        | Choice                                      |
| -------------- | ------------------------------------------- |
| Language       | Go 1.25                                     |
| HTTP           | [gin-gonic/gin](https://github.com/gin-gonic/gin) |
| Database       | PostgreSQL 16 via [jackc/pgx](https://github.com/jackc/pgx) v5 |
| Config         | [caarlos0/env](https://github.com/caarlos0/env) + godotenv |
| Logging        | `log/slog` (structured JSON)                |
| Scheduler      | cron (morning/evening reminders, carry-over) |
| Web UI         | `html/template` + HTMX + Chart.js (embedded) |
| CRM UI         | Next.js 15 + TypeScript + Tailwind (`frontend/`) |
| Queue          | Kafka (Redpanda in docker-compose)          |
| AI             | OpenAI API (optional)                         |
| Container      | Multi-stage → `distroless/static`           |

---

## Project layout

```
cmd/server/              Entrypoint — wires config, DB, HTTP, scheduler, web UI
internal/
  api/                   REST handlers + `/api/v1` routing
  config/                Typed environment configuration
  database/              pgx pool lifecycle
  model/                 Domain entities and invariants
  repository/            PostgreSQL data access
  service/               Business logic (tasks, reviews, metrics, reminders…)
  scheduler/             Cron jobs (reminders, overdue scan, auto carry-over)
  suggestion/            Rule-based coaching engine
  web/                   Server-rendered pages, static assets, HTMX handlers
migrations/
  deploy.sql             Single idempotent schema (use this to deploy)
  versions/              Historical incremental migrations (reference only)
```

---

## Quick start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose (recommended)
- `make`, `curl`, `jq` (optional, for health checks)

### 1. Configure

```bash
cp .env.example .env
# Edit DB_HOST_PORT if 5432 is already taken locally
```

### 2. Run with Docker (recommended)

```bash
make up        # build app image, start postgres + app
make health    # → {"status":"ok"}
make ready     # → {"status":"ready"} (checks DB)
```

Postgres runs the schema automatically on **first** container init via
`migrations/deploy.sql`. Open the UI at **http://localhost:8082/dashboard**.

```bash
make logs      # tail application logs
make psql      # interactive postgres shell
make down      # stop stack
```

### 3. Run locally (app on host, DB in Docker)

```bash
docker compose up -d postgres
make migrate   # apply deploy.sql if the volume already existed without schema
make run
```

Verify:

```bash
curl http://localhost:8082/healthz
curl http://localhost:8082/readyz
```

### 4. Tests

```bash
make test      # race detector, all packages
make vet
```

---

## Deployment

### Database schema

Use the **single deploy file** — idempotent, safe to re-run:

```bash
# Dockerised postgres (this project)
make migrate

# Any postgres (production, managed DB, local psql)
export DATABASE_URL="postgres://user:pass@host:5432/jobhunt?sslmode=require"
make migrate-local
# or:
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/deploy.sql
```

**Fresh Docker volume:** schema is applied automatically when postgres starts
for the first time (`docker-compose.yml` mounts `deploy.sql` into initdb).

**Existing database** that was created from old split migrations: `deploy.sql`
uses `IF NOT EXISTS` — running it is safe and will add any missing objects.

Historical step-by-step SQL files are archived under `migrations/versions/`.

### Application (Docker)

```bash
cp .env.example .env
# Set APP_ENV=production, strong DB_PASSWORD, LOG_FORMAT=json

make docker-build
docker compose up -d
```

Production checklist:

| Item | Notes |
| ---- | ----- |
| `APP_ENV=production` | Enables Gin release mode |
| `DB_SSLMODE=require` | When connecting to managed Postgres |
| `DATABASE_URL` | Preferred over discrete DB_* fields |
| `SCHEDULER_TZ` | Match your local timezone for cron jobs |
| Secrets | Never commit `.env`; rotate `DB_PASSWORD` |
| Health probes | `GET /healthz` (liveness), `GET /readyz` (DB readiness) |
| Port | App listens on **8082** by default |

### Application (binary)

```bash
make build                     # → bin/server
export DATABASE_URL=postgres://...
./bin/server
```

The binary embeds the web UI and static assets — no separate frontend build.

### Cron schedule (override via env)

| Variable | Default | Job |
| -------- | ------- | --- |
| `CRON_MORNING_REMINDER` | `0 9 * * *` | Enqueue daily planning reminder |
| `CRON_EVENING_REVIEW` | `0 21 * * *` | Enqueue evening review reminder |
| `CRON_WEEKLY_REVIEW` | `0 20 * * 0` | Enqueue Sunday weekly review reminder |
| `CRON_OVERDUE_SCANNER` | `*/15 * * * *` | Flag overdue tasks → reminder queue |
| `CRON_AUTO_CARRY_OVER` | `5 0 * * *` | Roll unfinished tasks to next day |
| `CRON_REMINDER_DISPATCHER` | `* * * * *` | Deliver pending reminders |
| `SCHEDULER_ENABLED` | `true` | Master switch for all cron jobs |

---

## Operation summary

### Web UI (`/` — HTMX pages)

Server-rendered pages with partial refresh. No SPA framework.

| Page | Path | What it does |
| ---- | ---- | ------------ |
| **Dashboard** | `/dashboard` | Focus-first home: today's completion hero, streak, 7-day trend, recent activity, rule-based suggestions. Cards refresh independently via HTMX. |
| **Tasks** | `/tasks` | Full task CRUD, **CSV import** for daily plans, filters (today/overdue/completed/carried/all), sortable table + mobile cards, bulk complete/delete, modal create/edit, optimistic row updates. |
| **Daily review** | `/reviews/daily` | End-of-day journal: reflection, wins, blockers, distractions, notes, energy/productivity scores. Autosaves per section; sidebar snapshot of completed/unfinished/overdue tasks. |
| **Weekly review** | `/reviews/weekly` | 7-day report: stats, streak, category breakdown, carry-over charts, suggestions, editable wins/bottlenecks/priorities. Week navigation + autosave notes. |
| **Analytics** | `/analytics` | Chart.js dashboards: completion trend, category ROI, weekly productivity, carry-over/overdue rates, execution time, streak history. Filterable 7/30/90-day ranges with HTMX panel refresh. |

**UX features (global):** loading indicators, styled confirm dialogs, toast notifications, keyboard shortcuts (`?`, `/`, `n`, `Esc`), inline errors, retry banners.

### REST API (`/api/v1`)

For integrations, scripts, or future mobile clients.

#### Tasks

| Method | Path | Description |
| ------ | ---- | ----------- |
| `POST` | `/tasks` | Create a task (title, category, priority, due date, estimates) |
| `GET` | `/tasks` | List with filters (`status`, `category`, `priority`, date range) |
| `GET` | `/tasks/overdue` | All overdue pending/in-progress tasks |
| `GET` | `/tasks/:id` | Fetch one task |
| `PATCH` | `/tasks/:id` | Update fields |
| `DELETE` | `/tasks/:id` | Delete |
| `POST` | `/tasks/:id/start` | Mark in progress |
| `POST` | `/tasks/:id/complete` | Mark completed (optional `actual_minutes`) |
| `POST` | `/tasks/:id/miss` | Mark missed |
| `POST` | `/tasks/:id/carry-over` | Carry to next day |

#### Task execution sessions (focus mode)

| Method | Path | Description |
| ------ | ---- | ----------- |
| `POST` | `/tasks/:id/sessions/start` | Start a focus session on a task |
| `GET` | `/tasks/:id/sessions/current` | Active or paused session, if any |
| `GET` | `/tasks/:id/sessions` | All sessions for a task |
| `GET` | `/sessions` | List sessions (filter by `task_id`, `status`) |
| `GET` | `/sessions/:id` | Fetch one session |
| `POST` | `/sessions/:id/pause` | Pause active session |
| `POST` | `/sessions/:id/resume` | Resume paused session |
| `POST` | `/sessions/:id/stop` | End without completing task |
| `POST` | `/sessions/:id/complete` | End session and mark task completed |
| `DELETE` | `/sessions/:id` | Delete session record |

#### Daily reviews

| Method | Path | Description |
| ------ | ---- | ----------- |
| `GET` | `/reviews` | List reviews (paginated) |
| `GET` | `/reviews/today` | Today's review |
| `PUT` | `/reviews/today` | Upsert today's review |
| `GET` | `/reviews/:date` | Review for `YYYY-MM-DD` |
| `PUT` | `/reviews/:date` | Upsert review for date |
| `DELETE` | `/reviews/:date` | Delete review |

#### Metrics

| Method | Path | Description |
| ------ | ---- | ----------- |
| `GET` | `/metrics/dashboard` | Combined dashboard payload |
| `GET` | `/metrics/today` | Today's status breakdown |
| `GET` | `/metrics/weekly` | Rolling 7-day stats |
| `GET` | `/metrics/trend` | Daily completion counts |
| `GET` | `/metrics/streak` | Current/longest streak |
| `GET` | `/metrics/categories` | Completion rate by category |

#### Suggestions

| Method | Path | Description |
| ------ | ---- | ----------- |
| `GET` | `/suggestions` | Active coaching suggestions |
| `POST` | `/suggestions/refresh` | Re-run rule engine |
| `GET` | `/suggestions/:id` | Fetch one |
| `POST` | `/suggestions/:id/dismiss` | Dismiss for the week |
| `DELETE` | `/suggestions/:id` | Delete |

### Background jobs

| Job | Function |
| --- | -------- |
| **Morning reminder** | Schedules a deduplicated “plan your day” reminder each morning. |
| **Evening review reminder** | Nudges you to complete the daily review. |
| **Weekly review reminder** | Sunday evening prompt for weekly reflection. |
| **Reminder dispatcher** | Delivers pending/failed reminders with retry limits (`REMINDER_MAX_ATTEMPTS`). |
| **Overdue scanner** | Every 15 minutes, finds overdue tasks and enqueues overdue reminders. |
| **Auto carry-over** | At 00:05 UTC (configurable), rolls eligible unfinished tasks to the next day. |

### Suggestion rules

The coaching engine evaluates metrics weekly and surfaces at most one active
suggestion per rule per ISO week:

| Rule | Triggers when… |
| ---- | -------------- |
| `reduce_workload` | Too many tasks missed or carried over |
| `smaller_tasks` | Average estimated time is high vs completion rate |
| `easier_wins` | Streak is low — recommends quick wins |
| `focus_shift` | One category dominates misses — suggests rebalancing |

---

## Health & ops endpoints

| Method | Path | Purpose |
| ------ | ---- | ------- |
| `GET` | `/healthz` | Liveness — process is running |
| `GET` | `/readyz` | Readiness — can reach PostgreSQL |

---

## Makefile reference

```bash
make help           # all targets
make up / down      # docker compose stack
make run            # local go run
make build          # compile bin/server
make test           # unit tests
make migrate        # apply deploy.sql (docker postgres)
make migrate-local  # apply deploy.sql (host psql)
make health / ready # probe endpoints
make psql           # postgres shell in container
```

---

## Task categories

`job_apply` · `recruiter_outreach` · `github` · `twitter` · `networking` ·
`learning` · `interview` · `misc`

Statuses: `pending` → `in_progress` → `completed` | `missed` (with carry-over support).

---

## Import daily tasks (CSV)

Plan your day in a spreadsheet (Notion, Google Sheets, Excel), export as CSV, then import on the **Tasks** page via **Import**.

**Tasks views:** **Today** shows items due now or overdue; **Upcoming** shows the next 7 days (use this after importing a multi-day plan).

### Template

Download from the app: **Tasks → Import → Download template**, or copy
[`docs/daily_tasks_template.csv`](docs/daily_tasks_template.csv).

```csv
title,description,category,priority,estimated_minutes,due_date
Apply to Acme Corp,Submit via Greenhouse,job_apply,high,45,
Reach out to recruiter,,recruiter_outreach,medium,20,
Post on LinkedIn,,twitter,low,15,
```

| Column | Required | Notes |
| ------ | -------- | ----- |
| `title` | yes* | Task name — or provide `task_id` as label when title is empty |
| `task_id` | no | External code (e.g. `W1D1-01`); ignored except as title fallback |
| `description` | no | Free text |
| `category` | no | App category or common alias — see mapping below |
| `notes` | no | Alias for `description` |
| `priority` | no | `low` / `medium` / `high` / `urgent` (case-insensitive) |
| `estimated_minutes` | no | Integer or decimal (`45`, `45.0`, `60 min`) |
| `due_date` | no | `YYYY-MM-DD`; **blank = today** (ideal for daily imports) |

**Category aliases** (import maps these to the app's fixed categories):

| Your label | Stored as |
| ---------- | --------- |
| `application`, `apply` | `job_apply` |
| `outreach`, `recruiter` | `recruiter_outreach` |
| `portfolio`, `readme` | `github` |
| `visibility`, `social`, `content` | `twitter` |
| `branding`, `linkedin` | `networking` |
| `study`, `practice`, `prep` | `learning` |
| `interview`, `mock_interview`, `system_design` | `interview` |
| `career`, `research`, `review`, `planning` | `misc` |
| anything else unrecognized | `misc` |

The import modal also lets you set a **default due date** for rows with blank dates. Rows with errors are skipped and listed in the result summary.

---

## License

Private / project use — adjust as needed for your deployment.
