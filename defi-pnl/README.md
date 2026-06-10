# DeFi PnL — Wallet Analytics & Smart-Money Signals

Go service that ingests **Uniswap V3** swap data from **The Graph**, stores trades in **PostgreSQL**, computes wallet PnL and leaderboards, and delivers **smart-money alerts** through a monetized **Telegram bot**.

Part of the [WEB3](../README.md) portfolio.

---

## What it does

| Layer | Description |
| ----- | ----------- |
| **Ingestion** | Fetches swap events from a Uniswap V3 subgraph; backfills up to 365 days on startup |
| **Storage** | `trades_v2` (raw swaps), `daily_leaderboard` (volume rankings), `pnl_leaderboard` (PnL snapshots) |
| **Analytics** | Average-cost PnL engine with realized/unrealized breakdown by protocol |
| **API** | REST endpoints for wallet PnL and daily/user/bot leaderboards |
| **Signals** | Monitors top PnL wallets for large swaps; pushes alerts to paid Telegram subscribers |
| **Scheduler** | Daily trade fetch, signal broadcast, and alert jobs (configurable local hours) |

---

## Stack

| Concern | Choice |
| ------- | ------ |
| Language | Go 1.25 |
| Database | PostgreSQL (`lib/pq`) |
| Data source | The Graph (GraphQL subgraph) |
| Bot | Telegram Bot API (long-polling) |
| Config | `.env` locally · process env on Render.com |
| Deploy | `-env=render` mode for production |

---

## Project layout

```
cmd/server/           HTTP server, schedulers, Telegram bot startup
internal/
  api/                REST handlers (/user/pnl, /leaderboard/*)
  jobs/               Backfill, daily trades, signals, alerts
  pnl/                PnL calculation engine
  storage/            Postgres repositories
  telegram/           Subscription bot (/start, /paid verification)
migrations/
  schema.sql          Consolidated schema (single source of truth)
```

---

## Quick start

### Option A — Docker (recommended)

**Prerequisites:** Docker, Docker Compose, [The Graph](https://thegraph.com/) API key

```bash
cp .env.example .env
# Set GRAPH_API_KEY (and optional TELEGRAM_BOT_TOKEN / PAYMENT_WALLET)

make run       # start Postgres + API in the foreground (logs in terminal; Ctrl+C to stop)
make run-detached  # same, but in the background
make import-db # wipe Docker Postgres, then import data from DB_URL in .env
make integrate # apply migrations/deploy.sql (run after any database change)
make redeploy  # make integrate + rebuild/restart the app
make logs      # follow logs
make stop      # stop containers
make reset-db  # wipe DB volume and re-apply schema on next `make run`
```

- API: `http://localhost:8081` (override with `HOST_PORT` in `.env`)
- Postgres: `localhost:15432` — user `postgres`, password `postgres`, database `defi_pnl` (override with `POSTGRES_PORT` in `.env`; or use `make psql` without a host port)
- `docker-compose.yml` overrides `DB_URL` to point at the `postgres` service
- Docker Postgres is a **separate** database from a local `go run` setup — wipe Docker data and copy your old DB with `make import-db`

### Option B — Local Go

**Prerequisites:** Go 1.25+, PostgreSQL, [The Graph](https://thegraph.com/) API key

```bash
cp .env.example .env
# Set GRAPH_API_KEY, DB_URL, and optional TELEGRAM_BOT_TOKEN / PAYMENT_WALLET

psql "$DB_URL" -f migrations/deploy.sql
go run ./cmd/server              # local mode (loads .env)
go run ./cmd/server -env=render  # production (process env only)
```

Default listen port: **8080** (override with `PORT`).

On startup the server:
- Recalculates today's PnL leaderboard snapshot
- Starts daily job schedulers
- Runs a background 365-day backfill (skips days already present)
- Starts the Telegram bot if `TELEGRAM_BOT_TOKEN` is set

### Database changes

`migrations/deploy.sql` is the deployment entry point; it runs `schema.sql`. Whenever you change the database:

1. Edit `migrations/schema.sql` (add tables/indexes in the main sections; add new columns in the **upgrades** section).
2. Run `make integrate` to apply changes to Postgres (idempotent — no drops, data preserved).
3. Run `make run` to restart the app, or use `make redeploy` to do steps 2 and 3 together.

---

## Environment variables

| Variable | Required | Description |
| -------- | -------- | ----------- |
| `GRAPH_API_KEY` | yes | The Graph Studio API key |
| `DB_URL` | yes | Postgres connection string |
| `PORT` | no | HTTP port (default `8080`) |
| `DAILY_JOB_HOUR` | no | Local hour for daily trade fetch (default `2`) |
| `SIGNAL_HOUR` | no | Local hour for signal broadcast (default `3`) |
| `TELEGRAM_BOT_TOKEN` | no | Enables Telegram bot; empty = disabled |
| `PAYMENT_WALLET` | no | USDT (Arbitrum) wallet shown in `/start` message |
| `SUBGRAPH_FETCH_LOG` | no | Set to `1` to log subgraph fetches to JSONL |
| `GRAPH_ENDPOINT` | no | Override full GraphQL endpoint URL |

See [`.env.example`](.env.example) for the full list.

---

## API

| Method | Path | Description |
| ------ | ---- | ----------- |
| `GET` | `/user/pnl?address=0x…` | PnL breakdown for a wallet |
| `GET` | `/leaderboard/daily` | Daily volume leaderboard |
| `GET` | `/leaderboard/daily/users` | Daily leaderboard — user-origin txs |
| `GET` | `/leaderboard/daily/bots` | Daily leaderboard — contract-origin txs |
| `GET` | `/leaderboard` | Overall leaderboard |
| `GET` | `/leaderboard/users` | User leaderboard |
| `GET` | `/leaderboard/bots` | Bot leaderboard |

---

## Telegram bot

When configured, the bot offers a paid subscription flow:

1. `/start` — shows pricing ($10/mo) and payment wallet (USDT on Arbitrum)
2. `/paid <tx_hash>` — verifies on-chain payment and activates subscription
3. Subscribers receive smart-money signal broadcasts from the daily scheduler

---

## Deployment (Render.com)

Set environment variables in the Render dashboard (no `.env` file). The server auto-detects Render via the `RENDER` env var, or pass `-env=render` explicitly.

---

## Disclaimer

Educational / portfolio project. Not financial advice. Subgraph data and PnL calculations are approximate — verify before making trading decisions.

---

## License

Private / portfolio use.
