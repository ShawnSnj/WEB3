-- defi-pnl deployment entry point.
--
-- Whenever you change the database, update migrations/schema.sql (and add
-- any new columns to the upgrades section), then redeploy:
--
--   make integrate    # apply deploy.sql to Postgres
--   make run          # rebuild and restart the app
--
-- Idempotent: safe to run on every deploy. Existing tables and data are kept.

\ir schema.sql
