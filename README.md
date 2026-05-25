# Shawn · Web3 & Backend Engineer

> I build **Go backends** and **on-chain systems** that turn DeFi data into products — analytics pipelines, trading bots, lending logic, and AI-assisted tooling.  
> This repo is my living portfolio: coursework foundations, production-style side projects, and the habits I use to improve every day.

[![GitHub](https://img.shields.io/badge/GitHub-ShawnSnj-181717?logo=github)](https://github.com/ShawnSnj)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Solidity](https://img.shields.io/badge/Solidity-0.8.x-363636?logo=solidity&logoColor=white)](https://soliditylang.org/)

---

## At a glance

| | |
|---|---|
| **Focus** | DeFi infrastructure · Go services · Smart contracts · MEV-aware automation |
| **Strength** | End-to-end delivery — from subgraph ingestion and Postgres to Telegram bots, HTMX dashboards, and Flashbots bundles |
| **Mindset** | Learn in public, ship in private repos first, iterate with metrics and daily reviews |
| **Contact** | [shawn_snj@163.com](mailto:shawn_snj@163.com) · [github.com/ShawnSnj](https://github.com/ShawnSnj) |

---

## What I bring to a team

**1. I ship full systems, not isolated scripts.**  
Several projects here run as long-lived services: scheduled jobs, REST APIs, database migrations, Docker deploys, and operator-facing UIs — not one-off homework snippets.

**2. I understand DeFi mechanics, not just SDK calls.**  
Lending (collateral, interest accrual, liquidation), AMMs (constant product, optimal sizing), flash loans, PnL accounting, and MEV-aware submission paths show up across multiple repos.

**3. I write testable Go.**  
[`jobHuntTask/`](jobHuntTask/) alone has 30+ test files covering handlers, services, and import flows. I treat regressions as unacceptable in code I expect others to read.

**4. I document and structure for maintainability.**  
[`twitter-leads-bot/`](twitter-leads-bot/) uses explicit interface boundaries and a single composition root — the kind of layout that scales when a second engineer joins.

**5. I learn systematically, then build originals.**  
MetaNode Academy coursework (`Golong/`, `Phase4-BackendPractice/`, parts of `solidity/`) gave me the base; projects like `defi-pnl`, `arb-bot`, and `jobHuntTask` are where I applied it.

---

## Featured projects

### [`jobHuntTask/`](jobHuntTask/) — Job Hunt OS (Go + Postgres + HTMX)

A production-grade task tracker I built to **run my own job search like a product**:

- Dashboard, daily/weekly reviews, analytics, CSV import, focus sessions, cron reminders
- REST API + server-rendered HTMX UI (no SPA overhead)
- Docker Compose, idempotent migrations, health/readiness probes, structured logging

**Why it matters to a hiring manager:** it proves I can own a feature from schema design → API → UI → ops, and that I hold myself accountable with data (streaks, carry-over rates, coaching rules).

→ [Full docs](jobHuntTask/README.md)

---

### [`defi-pnl/`](defi-pnl/) — DeFi PnL Analytics & Smart-Money Signals

On-chain analytics service that ingests **Uniswap V3** swap data via **The Graph**, computes wallet PnL and leaderboards, and pushes alerts through a **monetized Telegram bot**.

- Subgraph → Postgres pipeline with backfill and daily schedulers
- Average-cost PnL engine; user vs bot leaderboard separation
- REST endpoints for PnL and rankings; Render.com deployment mode
- Paid subscription flow (USDT on Arbitrum) for signal access

**Stack:** Go · PostgreSQL · The Graph · Telegram Bot API

---

### [`twitter-leads-bot/`](twitter-leads-bot/) — AI Lead Generation (Go + Gemini + HTMX)

Searches Twitter/X for prospect tweets, scores relevance with **Google Gemini**, drafts reply suggestions, and triages leads in an HTMX dashboard.

- Multi-provider RapidAPI pool with quota-aware rotation
- Clean architecture: interfaces in `internal/`, wiring only in `cmd/server`
- Structured JSON from LLM for scoring + reply drafting

→ [Full docs](twitter-leads-bot/README.md)

---

### [`arb-bot/`](arb-bot/) — Cross-DEX Arbitrage (Go + Flashbots)

Monitors two constant-product AMM pools, computes optimal trade size off-chain, **simulates via Flashbots `CallBundle`**, and submits bundles only when profitable.

- Go bot: reserve polling, `OptimalDx` sizing, EIP-1559 tx building
- Solidity: `SimpleDEX` (toy AMM) + `FlashArb` executor
- Config-driven deployment (`config/config.json`)

**Stack:** Go · go-ethereum · Hardhat · Flashbots

---

### [`solidity/`](solidity/) — DeFi Bot Suite

Collection of on-chain automation projects:

| Project | What it does |
|---------|--------------|
| [`uniswap-bot/`](solidity/uniswap-bot/) | Real-time Uniswap price monitoring and optional auto-swap |
| [`aave-bot/`](solidity/aave-bot/) | Aave V3 health-factor monitoring and liquidation automation |
| [`uniswapFlashloan/`](solidity/uniswapFlashloan/) | Aave V3 flash loan + Uniswap V2/V3 arbitrage contract |
| [`flashloan-demo/`](solidity/flashloan-demo/) | Flash-loan liquidation demo with Hardhat mainnet fork |

Each subproject has its own README with setup, addresses, and security notes.

---

## More in this repo

| Directory | Description |
|-----------|-------------|
| [`defi-security-notes/`](defi-security-notes/) | 7-day DeFi lending mini-course I wrote — notes + annotated `SimpleLending.sol` |
| [`Phase4-BackendPractice/`](Phase4-BackendPractice/) | DApp backend practice: ethclient, abigen, upgradeable NFT auction, pledge/NFT market study |
| [`Golong/`](Golong/) | Go fundamentals: algorithms, concurrency, GORM, JWT blog API |
| [`uniswap-learning/`](uniswap-learning/) | Uniswap V2 core/periphery study workspace |

---

## Tech stack

```
Languages     Go · Solidity · SQL
Web3          go-ethereum · abigen · Hardhat · Foundry · Flashbots · The Graph
DeFi          Uniswap V2/V3 · Aave V3 · Chainlink · lending/liquidation · MEV
Backend       Gin · GORM · PostgreSQL · MySQL · Redis · REST · cron schedulers
Frontend/Ops  HTMX · html/template · Chart.js · Docker · structured logging (slog)
AI            Google Gemini · structured LLM output
```

---

## How I improve day by day

I treat growth like a sprint board — the same mindset behind [`jobHuntTask/`](jobHuntTask/). Roughly:

| Day focus | What I do | Repo evidence |
|-----------|-----------|---------------|
| **Build** | Ship one concrete increment (endpoint, job, contract, test) | Recent commits across `defi-pnl`, `jobHuntTask` |
| **Learn** | Read protocol docs / walk through reference implementations | `uniswap-learning/`, `defi-security-notes/`, Compound study in `Phase4-BackendPractice/` |
| **Practice** | Algorithms & Go idioms | `Golong/task1/` LeetCode-style drills |
| **Review** | Weekly retrospective — what shipped, what blocked, what to cut | `jobHuntTask` weekly review + analytics |
| **Share** | READMEs, notes, diagrams for the next reader (including future me) | This file; per-project docs |

### Strengths (today)

- **Execution:** Multiple runnable services with real persistence, scheduling, and deployment paths
- **DeFi breadth:** Trading, lending, PnL, flash loans, and bot automation — not a single-niche tutorial fork
- **Backend craft:** Layered packages, migrations, tests, Makefile/Docker ergonomics
- **Ownership:** I built tools to manage my own job hunt — evidence of initiative, not just assignment completion

### Active growth areas (honest)

These are the edges I'm deliberately pushing — the kind of engineer I am *becoming*, not claiming to have finished:

| Area | Current state | Next step |
|------|---------------|-----------|
| **Algorithms** | Solid on fundamentals; some drills still use brute-force / naive sort | Replace with canonical patterns (hash maps, heaps, merge sort) and benchmark |
| **Production hardening** | Side projects run locally / on Render; auth & rate limits called out as TODOs | Add auth middleware, request limits, and migration tooling (`golang-migrate`) |
| **Smart contract security** | Educational contracts with disclaimers; no formal audit trail | More Foundry invariant/fuzz tests; study past exploits; contribute to audit write-ups |
| **Public product surface** | Strong code, uneven top-level docs on a few repos | README + demo GIF for `defi-pnl` and `arb-bot` (in progress via this portfolio) |
| **Mainnet operations** | Fork tests and testnet flows; cautious with unaudited deploys | Documented testnet deployments with tx hashes and runbooks |

> **To a potential manager:** I'm not claiming seniority I haven't earned. I *am* showing consistent output, clear self-assessment, and a system for getting better — which is what I'd bring to your team on week one.

---

## Learning journey

Structured training via [MetaNode Academy](https://github.com/MetaNodeAcademy) (Go base, Solidity advance, DApp backends) forms the curriculum layer under this repo. On top of that, every **original project** here is something I designed because I wanted the skill, not because a PDF told me to.

---

## Repository map

```
WEB3/
├── jobHuntTask/           ← flagship: job search OS (Go, Postgres, HTMX)
├── defi-pnl/              ← PnL analytics + Telegram signals
├── twitter-leads-bot/     ← AI lead gen (Gemini + HTMX)
├── arb-bot/               ← Flashbots arbitrage bot
├── solidity/              ← Uniswap/Aave bots + flash loan contracts
├── defi-security-notes/   ← 7-day lending course (my notes + contract)
├── Phase4-BackendPractice/← DApp backend + contract coursework
├── Golong/                ← Go fundamentals + blog API
└── uniswap-learning/      ← Uniswap V2 source study
```

---

## Get in touch

Interested in Web3 backend, DeFi infrastructure, or Go platform work? I'd love to talk.

- **Email:** [shawn_snj@163.com](mailto:shawn_snj@163.com)
- **GitHub:** [@ShawnSnj](https://github.com/ShawnSnj)

<!-- Optional — uncomment and fill in when ready:
- **LinkedIn:** https://linkedin.com/in/your-handle
- **Location:** Your city / timezone
- **Open to:** Remote · Relocation · Contract · Full-time
-->

---

*Last updated: May 2025 · Built and maintained as a public record of what I can do and what I'm working on next.*
