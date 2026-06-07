# Twitter Leads Bot

A small Go service that searches Twitter/X for prospect tweets, scores them
with Google Gemini, and drafts reply suggestions in a simple HTMX dashboard.

## Stack

- Go 1.22+
- `gorilla/mux` HTTP router
- PostgreSQL via `database/sql` + `lib/pq`
- HTMX (loaded from CDN) + `html/template` server-rendered UI
- Google Gemini API via `google.golang.org/genai` (default model: `gemini-2.5-flash`)
- Tweet source: RapidAPI Twitter scraper (default) **or** Twitter/X API v2

## Layout

```
cmd/server/        program entry point + dependency wiring
cmd/rapidapi-test/ smoke test for the RapidAPI search client
cmd/pool-test/     smoke test for the multi-key/multi-provider pool
cmd/env-test/      verifies .env loading
internal/twitter/  Searcher interface + impls (RapidAPI, official v2, Pool)
internal/gemini/   relevance + reply generator (Gemini client, Analyzer interface)
internal/db/       Postgres repository (Repository interface)
internal/web/      Gorilla mux handlers + HTMX templates (embedded)
migrations/        SQL schema
```

Inner packages depend only on their own interfaces; `cmd/server/main.go` is the
only place that knows the concrete implementations exist. That's the
"clean architecture / DI" boundary kept deliberately small.

## Setup

1. Create a Postgres database and apply the schema:

   ```bash
   createdb twitter_leads
   psql twitter_leads -f migrations/001_init.sql
   ```

2. Copy `.env.example` to `.env` and fill in your credentials, then export them
   (or use `direnv` / `godotenv`-style loader of your choice):

   ```bash
   cp .env.example .env
   set -a && source .env && set +a
   ```

3. Run the server:

   ```bash
   go run ./cmd/server
   ```

4. Open <http://localhost:8080>.

## Workflow

1. Type a Twitter search query (e.g. `"need a crm" OR "looking for crm"`) and
   submit. Tweets are deduped by `tweet_id` and stored as leads.
2. Open a lead to see a 1–10 relevance score and one-line reasoning from Gemini.
3. Click **Draft reply** to generate a short conversational reply suggestion.
4. **Save** / **Dismiss** to triage; high-scoring leads bubble to the top.

## Tweet sources

`cmd/server` picks the search backend at startup based on which credential is
present in `.env`, in this priority order:

1. **`RAPIDAPI_POOL`** set → mixed-provider pool. Comma-separated `host|key`
   entries. Round-robins between them and parks any member that hits its
   quota (HTTP 429 / "exceeded") for `RAPIDAPI_COOLDOWN` (default 1h).
2. **`RAPIDAPI_KEYS`** set → same-provider key pool. Comma-separated keys
   that all target `RAPIDAPI_HOST`. Best way to stitch together several free
   tiers of the same scraper.
3. **`RAPIDAPI_KEY`** set → single RapidAPI client (no rotation).
4. **`TWITTER_BEARER_TOKEN`** set → official X v2 `recent search` endpoint
   (requires the paid Basic tier).

### Free RapidAPI Twitter scrapers worth pooling

Quotas and pricing change constantly — verify on RapidAPI before subscribing.
All of these expose a search endpoint with broadly similar JSON; the ones
marked ✅ are known to parse cleanly with the bundled `RapidAPIClient`. For
others you'll likely need a small `normalizeAuthor`/field-name tweak in
`internal/twitter/rapidapi.go`.

| Provider | Host | Free tier (approx) | Bundled parser? |
|---|---|---|---|
| Twitter API 4.5 | `twitter-api45.p.rapidapi.com` | 100 req/day | ✅ |
| Twitter154 | `twitter154.p.rapidapi.com` | 50 req/day | needs adapter |
| Twitter241 | `twitter241.p.rapidapi.com` | ~1k req/month | needs adapter |
| Twttr API | `twttrapi.p.rapidapi.com` | ~100 req/month | needs adapter |
| Real-Time Twitter Search | `real-time-twitter-search.p.rapidapi.com` | small monthly | needs adapter |

The cheap trick to multiply free quota: subscribe to the same provider
(say Twitter API 4.5) under several different email accounts, paste each
resulting API key into `RAPIDAPI_KEYS=k1,k2,k3,…` and you've effectively got
N × 100 req/day with automatic failover.

### Smoke tests

```bash
# Single key
go run ./cmd/rapidapi-test "polymarket" 5

# Pool (multiple keys / mixed providers)
go run ./cmd/pool-test "polymarket" 3
```

`pool-test` prints which member served each round, so you can watch the
rotation and verify that quota-exhausted members get parked.

## Notes

- Tweet search backends use plain `net/http`; the LLM path uses Google’s
  official `genai` SDK for structured JSON output from Gemini.
- For production: add migrations tooling (e.g. `golang-migrate`), structured
  logging, request auth, and rate limiting around the search endpoint.
