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

### Prerequisites

- Go 1.25+
- PostgreSQL
- [The Graph](https://thegraph.com/) API key for subgraph queries

### 1. Configure

```bash
cp .env.example .env
# Set GRAPH_API_KEY, DB_URL, and optional TELEGRAM_BOT_TOKEN / PAYMENT_WALLET
```

### 2. Apply schema

```bash
psql "$DB_URL" -f migrations/schema.sql
```

> **Note:** `schema.sql` drops and recreates tables — use only on fresh/dev databases.

### 3. Run

```bash
go run ./cmd/server              # local mode (loads .env)
go run ./cmd/server -env=render  # production (process env only)
```

Default listen port: **8080** (override with `PORT`).

On startup the server:
- Recalculates today's PnL leaderboard snapshot
- Starts daily job schedulers
- Runs a background 365-day backfill (skips days already present)
- Starts the Telegram bot if `TELEGRAM_BOT_TOKEN` is set

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
