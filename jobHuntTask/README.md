# Job Hunt Task — Tracking & Reminder System

A production-grade Go application for planning, executing, and reviewing your
daily job-hunt routine. It combines a REST API, background scheduler, rule-based
coaching suggestions, and a server-rendered HTMX web UI — all backed by
PostgreSQL.

**Default URL:** [http://localhost:8082](http://localhost:8082) (task tracker + CRM at [/crm/](http://localhost:8082/crm/))

---

## Quick start (local dev)

One command after setup:

```bash
cp .env.example .env
make migrate-all   # first time only — creates tables in postgres
make run           # starts postgres (docker) if needed, builds CRM UI, runs server
```

Open **http://localhost:8082/tasks** (task tracker) or **http://localhost:8082/crm/** (Job Hunt CRM).

Skip the CRM rebuild on repeat runs:

```bash
make run-server    # same as make run, but no npm build
```

---

## AI Job Hunt CRM

A personal recruiter layer on top of the task tracker: automatic job collection,
AI/heuristic fit scoring, daily brief (1 apply + 2 outreach + 1 skill), application
pipeline, resume analysis, skill gaps, weekly review, and career coach.

See **[docs/CRM.md](docs/CRM.md)** for CRM-specific API and pipeline details.

```bash
make integrate       # postgres + kafka + migrate + unified server (no app container)
make integrate-up    # full docker stack (app + worker + postgres + kafka)
make migrate-all     # apply tasks + CRM tables to existing postgres
make crm-collect     # fetch & score jobs
make frontend-dev    # Next.js CRM UI on :3000 (dev only; production uses /crm/)
```

Both systems share **one PostgreSQL database** (`jobhunt`) with separate table groups.
One Go process (`cmd/server`) serves the task tracker UI, REST API, and CRM API.

Set `OPENAI_API_KEY` for GPT-powered matching and outreach (heuristics work without it).

---

## Architecture & code patterns

This is a **layered monolith**: one deployable binary, clear separation of concerns,
no business logic in `main` or HTTP handlers.

```
Browser / API client
        │
        ▼
┌───────────────────────────────────────────────────────┐
│  Gin router (internal/api)                            │
│  ├── /api/v1/*     REST JSON (integrations, CRM API)  │
│  ├── /dashboard, /tasks, …   HTMX pages (internal/web) │
│  ├── /crm/*        embedded Next.js static export     │
│  └── /healthz, /readyz                                │
└───────────────────────────────────────────────────────┘
        │
        ▼
┌──────────────────┐     ┌──────────────────┐
│  service/        │     │  crm/service/    │  ← business rules, orchestration
│  (tasks, reviews,│     │  (collect, score,│
│   metrics, …)    │     │   brief, AI)     │
└────────┬─────────┘     └────────┬─────────┘
         │                        │
         ▼                        ▼
┌──────────────────┐     ┌──────────────────┐
│  repository/     │     │  crm/repository/ │  ← SQL via pgx
└────────┬─────────┘     └────────┬─────────┘
         │                        │
         └────────────┬───────────┘
                      ▼
              PostgreSQL (jobhunt)
                      ▲
         ┌────────────┴───────────┐
         │  scheduler/ (cron)     │  ← reminders, rollover, CRM pipeline
         │  crm-worker (optional) │  ← Kafka consumer for async scoring
         └────────────────────────┘
```

**Layer rules**

| Layer | Package | Responsibility |
| ----- | ------- | ---------------- |
| Entrypoint | `cmd/server` | Load config, wire dependencies, start HTTP + scheduler, graceful shutdown — **no domain logic** |
| HTTP (JSON) | `internal/api` | Parse requests, call services, return JSON |
| HTTP (HTML) | `internal/web` | Server-rendered templates + HTMX partials + static assets |
| Business logic | `internal/service`, `internal/crm/service` | Rules, validation, orchestration |
| Data access | `internal/repository`, `internal/crm/repository` | PostgreSQL queries only |
| Domain | `internal/model`, `internal/crm/model` | Entities and enums |
| Infrastructure | `internal/config`, `internal/database`, `internal/scheduler` | Cross-cutting concerns |

**UI split**

| Surface | Tech | Served from |
| ------- | ---- | ----------- |
| Task tracker | `html/template` + HTMX + embedded CSS/JS | Go binary (`internal/web/`) |
| Job Hunt CRM | Next.js 15 static export | Built into `internal/web/static/crm/`, served at `/crm/` |

The CRM frontend is **not** a separate process in production — `make run` / `make build` compiles it into the Go binary.

---

## How `make run` works

`make run` is the default local dev workflow. It chains four steps:

```
make run
  │
  ├─ ensure-db
  │    ├─ Read DB_HOST / DB_PORT from .env
  │    ├─ pg_isready → already up? continue
  │    └─ else → make db-up  (docker compose up -d postgres)
  │              → make wait-db  (poll until postgres accepts connections)
  │
  ├─ check-port
  │    └─ Fail if HTTP_PORT (default 8082) is already in use
  │
  ├─ build-crm
  │    └─ npm run build in frontend/ → copy static files to internal/web/static/crm/
  │
  └─ go run ./cmd/server
       └─ Connect to postgres at localhost:DB_PORT, listen on :8082
```

**Important env vars for DB connectivity**

| Variable | Purpose |
| -------- | ------- |
| `DB_HOST` | Host the Go app connects to (`localhost` when app runs on your machine) |
| `DB_PORT` | Port the Go app connects to — must match the published docker port |
| `DB_HOST_PORT` | Port postgres is published on your machine (docker-compose `ports:` mapping) |

For `make run`, keep **`DB_PORT` = `DB_HOST_PORT`**. Example if another project uses 5432:

```env
DB_HOST_PORT=5433
DB_PORT=5433
```

Inside docker-compose, the `app` container always uses `DB_HOST=postgres` and `DB_PORT=5432` (internal network) — never `DB_HOST_PORT`.

**First-time database setup**

- **Brand-new docker volume:** postgres auto-runs `migrations/deploy.sql` on first container start (mounted into `/docker-entrypoint-initdb.d/`).
- **Existing volume or local postgres:** run `make migrate-all` once to apply (or update) the schema. The file is idempotent (`IF NOT EXISTS`).

---

## Docker

`docker-compose.yml` defines four services. You rarely need all of them for day-to-day dev.

| Service | Image / build | Role | Typical dev usage |
| ------- | ------------- | ---- | ----------------- |
| **postgres** | `postgres:16-alpine` | Primary database | Always — started by `make run` via `ensure-db` |
| **app** | `Dockerfile` (multi-stage Go + CRM) | Unified server in a container | `make up` / `make integrate-up` |
| **redpanda** | Kafka-compatible broker | Async CRM job scoring queue | `make integrate` / `make integrate-up` |
| **crm-worker** | `Dockerfile.crm-worker` | Consumes Kafka, scores jobs | `make integrate-up` only |

### How the Docker files relate

Four files work together — each has a distinct job:

| File | Role |
| ---- | ---- |
| **`docker-compose.yml`** | Orchestrator — defines which containers run, ports, env vars, volumes, and service dependencies |
| **`Dockerfile`** | Build recipe for the **`app`** service (`cmd/server` + embedded CRM UI at `/crm/`) |
| **`Dockerfile.crm-worker`** | Build recipe for the **`crm-worker`** service (`cmd/crm-worker` — Kafka consumer only) |
| **`Makefile`** | Shortcuts — wraps `docker compose` and local dev; picks which parts of the stack to start |

```
Makefile  ──calls──▶  docker compose  ──reads──▶  docker-compose.yml
                              │                        │
                              │                        ├── postgres, redpanda  (official images)
                              │                        ├── app                 (builds from Dockerfile)
                              │                        └── crm-worker          (builds from Dockerfile.crm-worker)
                              │
                              └── also runs go/npm on your host (make run) without building the app image
```

**Service wiring inside compose:**

```
postgres ──┬── app (Dockerfile)
           │      │  HTTP :8082, cron scheduler, publishes to Kafka
redpanda ──┤      │
           │      └──► crm.jobs.ingested (Kafka topic)
           │
           └── crm-worker (Dockerfile.crm-worker)
                  └── reads Kafka, scores jobs, writes to postgres
```

**`Dockerfile`** (main app) — three stages:

1. **crm** — `npm run build` in `frontend/`
2. **builder** — embed CRM static files, `go build ./cmd/server`
3. **runtime** — `distroless/static`, non-root, port 8082

**`Dockerfile.crm-worker`** — lighter image: builds only `./cmd/crm-worker`, no UI, no HTTP server.

**Which Makefile targets use which files:**

| Target | Compose services | Dockerfiles |
| ------ | ---------------- | ----------- |
| `make db-up` | `postgres` only | none |
| `make run` | `postgres` only (if not already up) | none — app runs via `go run` on host |
| `make integrate` | `postgres` + `redpanda` | none — app runs via `go run` on host |
| `make up` / `make integrate-up` | all four | **both** Dockerfiles |
| `make docker-build` | none (build image only) | `Dockerfile` only |

**Two dev paths:**

```
Path A — make run (hybrid, recommended)
────────────────────────────────────────
  Your machine                         Docker
  ┌──────────────────┐                ┌──────────────┐
  │ go run ./cmd/server │──localhost──▶│ postgres     │
  │ :8082            │   :DB_PORT     └──────────────┘
  └──────────────────┘
  Dockerfile not used for the app


Path B — make up (full docker)
──────────────────────────────
  Docker internal network
  ┌──────────┐     ┌─────────────┐     ┌────────────┐
  │ postgres │◀────│ app         │────▶│ redpanda   │
  └────▲─────┘     │ (Dockerfile)│     └─────┬──────┘
       │           └─────────────┘           │
       └──────── crm-worker (Dockerfile.crm-worker)
  Host :8082 → app container
```

### Postgres container

```yaml
postgres:
  ports: "${DB_HOST_PORT:-5432}:5432"          # host → container
  volumes:
    - jobhunt_pgdata:/var/lib/postgresql/data   # persistent data
    - ./migrations/deploy.sql → initdb script   # runs ONLY on first empty volume
```

Flow on **first** `docker compose up -d postgres`:

1. Docker creates the `jobhunt_pgdata` volume.
2. Postgres initializes the `jobhunt` database.
3. `deploy.sql` runs automatically — tasks + CRM tables, indexes, enums.
4. Postgres listens on container port 5432, published to your host at `DB_HOST_PORT`.

On **subsequent** starts, the volume already has data — init scripts are skipped. Use `make migrate-all` to apply schema changes.

Useful commands:

```bash
make db-up      # start postgres only
make wait-db    # block until postgres is ready
make psql       # interactive shell inside the container
make down       # stop all compose services
```

### App container (`make up`)

See **How the Docker files relate** above for the full build pipeline. The running `app` container connects to `postgres:5432` on the docker network (not `localhost`).

### Two ways to run the stack

| Mode | Command | What runs where |
| ---- | ------- | ---------------- |
| **Hybrid (recommended for dev)** | `make run` | Postgres in Docker; Go app + scheduler on your host (hot reload via `go run`) |
| **Full Docker** | `make up` or `make integrate-up` | Postgres + app (+ kafka + worker) all in containers |

```bash
# Hybrid — edit Go/templates, restart make run
make run

# Full docker — production-like, no local Go needed
make up
make health && make ready
```

If port 8082 is taken by the docker `app` container, either use it directly or run `make stop-app` before `make run`.

---

## Project layout

```
cmd/
  server/              Main entrypoint — wiring only
  crm-worker/          Kafka consumer for async job scoring
  dedupe-tasks/        CLI: remove duplicate tasks
  reschedule-tasks/    CLI: spread overdue tasks across future days
internal/
  api/                 REST handlers + /api/v1 routing (thin HTTP layer)
  web/                 HTMX pages, templates, embedded static + CRM assets
  service/             Task tracker business logic
  repository/          PostgreSQL access (tasks domain)
  model/               Domain entities
  crm/                 CRM subsystem (aggregator, AI, kafka, engines)
  scheduler/           Cron jobs (reminders, rollover, CRM pipeline)
  config/              Typed environment configuration
  database/            pgx pool lifecycle
frontend/              Next.js CRM — output copied into internal/web/static/crm/
migrations/
  deploy.sql           Single idempotent schema (tasks + CRM) — use for deploy
  *.sql                Incremental migrations (also folded into deploy.sql)
docker-compose.yml     postgres, redpanda, app, crm-worker
Dockerfile             Multi-stage build → app (cmd/server + CRM UI)
Dockerfile.crm-worker  Build → crm-worker (Kafka consumer)
Makefile               run, db-up, migrate-all, integrate, …
```

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

## Prerequisites

- Go 1.25+
- Docker Desktop (recommended — postgres starts automatically via `make run`)
- Node.js 22+ (for CRM UI build on first `make run`)
- `make`, `curl`, `jq` (optional, for health checks)

---

## Deployment

### Overview

| Target | Steps | Best for |
| ------ | ----- | -------- |
| **Local dev** | `cp .env.example .env` → `make migrate-all` → `make run` | Daily development |
| **Docker (single host)** | `make docker-build` → `docker compose up -d` | Staging / small VPS |
| **Binary + managed DB** | `make build` → ship `bin/server` → set `DATABASE_URL` | Production with RDS/Cloud SQL |

All paths use the same schema file: **`migrations/deploy.sql`** (idempotent).

### 1. Local development (hybrid)

Postgres runs in Docker; the Go app runs on your machine.

```bash
cp .env.example .env
# Optional: set DB_HOST_PORT if 5432 is taken (and DB_PORT to match)

make migrate-all   # first time, or after schema changes
make run           # ensure-db → build-crm → go run
```

Verify:

```bash
curl http://localhost:8082/healthz   # liveness
curl http://localhost:8082/readyz    # postgres reachable
```

### 2. Full Docker stack

Everything runs in containers — no local Go or Node required after the image is built.

```bash
cp .env.example .env
# Set APP_ENV=production, strong DB_PASSWORD, LOG_FORMAT=json for prod

make up            # build image + start postgres, redpanda, app, crm-worker
make migrate-all   # safe on existing volumes; required if volume predates a schema change
make health && make ready
```

Or use the integrated targets:

```bash
make integrate-up  # docker compose up -d --build + migrate-all
make integrate     # postgres + kafka only, then go run on host (no app container)
```

Open **http://localhost:8082/dashboard** (tasks) and **http://localhost:8082/crm/** (CRM).

```bash
make logs          # tail app container logs
make psql          # postgres shell
make down          # stop stack (volume persists)
```

### 3. Database schema

```bash
# Docker postgres (this project)
make migrate-all

# Any postgres — reads DATABASE_URL or DB_* from .env
make migrate-all-local

# Or pipe deploy.sql directly
export DATABASE_URL="postgres://user:pass@host:5432/jobhunt?sslmode=require"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/deploy.sql
```

**Fresh docker volume:** schema applied automatically on first postgres start (`deploy.sql` mounted into `docker-entrypoint-initdb.d/`).

**Existing database:** `deploy.sql` uses `IF NOT EXISTS` — safe to re-run. Incremental files under `migrations/*.sql` exist for targeted upgrades; `make migrate-task-notes-structured` etc. apply individual migrations when needed.

### 4. Production binary (no Docker for the app)

```bash
make build                     # builds CRM UI + compiles → bin/server
export DATABASE_URL=postgres://user:pass@db-host:5432/jobhunt?sslmode=require
export APP_ENV=production
./bin/server
```

The binary embeds the task tracker UI, static assets, and CRM at `/crm/` — no separate frontend server.

### Production checklist

| Item | Notes |
| ---- | ----- |
| `APP_ENV=production` | Enables Gin release mode |
| `DATABASE_URL` | Preferred over discrete `DB_*` fields |
| `DB_SSLMODE=require` | When connecting to managed Postgres |
| `SCHEDULER_TZ` / `APP_TIMEZONE` | Match your timezone for midnight rollover and cron |
| `OPENAI_API_KEY` | Optional; CRM heuristics work without it |
| Secrets | Never commit `.env`; rotate `DB_PASSWORD` |
| Health probes | `GET /healthz` (liveness), `GET /readyz` (DB readiness) |
| Port | App listens on **8082** by default (`HTTP_PORT`) |
| Kafka | Set `KAFKA_BROKERS` for async CRM scoring; leave empty to disable |

### Cron schedule (override via env)

| Variable | Default | Job |
| -------- | ------- | --- |
| `CRON_MORNING_REMINDER` | `0 9 * * *` | Enqueue daily planning reminder |
| `CRON_EVENING_REVIEW` | `0 21 * * *` | Enqueue evening review reminder |
| `CRON_WEEKLY_REVIEW` | `0 20 * * 0` | Enqueue Sunday weekly review reminder |
| `CRON_OVERDUE_SCANNER` | `*/15 * * * *` | Flag overdue tasks → reminder queue |
| `CRON_AUTO_CARRY_OVER` | `5 0 * * *` | Roll unfinished tasks to next day |
| `CRON_DAILY_ROLLOVER` | `0 0 * * *` | Yesterday pending → missed; today → pending |
| `CRON_REMINDER_DISPATCHER` | `* * * * *` | Deliver pending reminders |
| `CRON_CRM_DAILY_PIPELINE` | `0 7 * * *` | CRM collect → score → daily brief |
| `SCHEDULER_ENABLED` | `true` | Master switch for all cron jobs |

### Tests

```bash
make test      # race detector, all packages
make vet
```

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
make help              # all targets

# Local dev
make run               # ensure-db + build-crm + go run (recommended)
make run-server        # ensure-db + go run (skip CRM rebuild)
make db-up             # docker compose up -d postgres
make stop-app          # stop docker app container (free port 8082)

# Docker
make up / down         # full compose stack
make integrate         # postgres + kafka + migrate + go run on host
make integrate-up      # full docker stack + migrate

# Build & test
make build             # CRM UI + bin/server
make build-crm         # Next.js → internal/web/static/crm only
make test / vet

# Database
make migrate-all       # apply deploy.sql via docker postgres
make migrate-all-local # apply deploy.sql via host psql
make psql              # postgres shell in container
make wait-db           # poll until postgres is ready

# Ops
make health / ready    # probe /healthz and /readyz
make crm-collect       # POST /api/v1/crm/collect
make frontend-dev      # Next.js dev server on :3000
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
