#!/usr/bin/env sh
# Wipe all data in Docker Postgres, then import from the source DB (DB_URL / SOURCE_DB_URL).
set -eu

cd "$(dirname "$0")/.."

read_env_var() {
	var_name="$1"
	# Read KEY=value from .env without sourcing (safe for special characters in values).
	line=$(grep -E "^${var_name}=" .env 2>/dev/null | head -n 1 || true)
	if [ -z "$line" ]; then
		return 1
	fi
	printf '%s' "$line" | sed "s/^${var_name}=//"
}

if [ ! -f .env ]; then
	echo "error: .env not found (copy from .env.example)" >&2
	exit 1
fi

SOURCE_DB_URL=$(read_env_var SOURCE_DB_URL || true)
if [ -z "$SOURCE_DB_URL" ]; then
	SOURCE_DB_URL=$(read_env_var DB_URL || true)
fi
if [ -z "$SOURCE_DB_URL" ]; then
	echo "error: set DB_URL or SOURCE_DB_URL in .env" >&2
	exit 1
fi

# Application tables to import (must exist in both source and Docker DB).
TABLES="daily_leaderboard trades_v2 pnl_leaderboard subscribers pushed_signals"

# From inside the compose Postgres container, reach a host-local DB via host.docker.internal.
IMPORT_SOURCE_URL=$(printf '%s' "$SOURCE_DB_URL" | sed \
	's/@localhost:/@host.docker.internal:/;s/@127\.0\.0\.1:/@host.docker.internal:/')

echo "==> Source: ${SOURCE_DB_URL%%\?*} (credentials hidden)"
echo "==> Target: Docker Postgres (defi-pnl)"

echo "==> Stopping API server to avoid writes during import..."
docker compose stop server 2>/dev/null || true

echo "==> Starting Docker Postgres (if not already running)..."
docker compose up -d postgres

echo "==> Waiting for Docker Postgres..."
until docker compose exec -T postgres pg_isready -U postgres -d defi_pnl >/dev/null 2>&1; do
	sleep 1
done

echo "==> Verifying source database connection..."
if ! docker compose exec -T postgres pg_dump "$IMPORT_SOURCE_URL" --schema-only -t daily_leaderboard >/dev/null 2>&1; then
	echo "error: cannot reach source database." >&2
	echo "       Check DB_URL / SOURCE_DB_URL in .env (your old DB on localhost:5432)." >&2
	exit 1
fi

echo "==> Deleting all data from Docker Postgres..."
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d defi_pnl <<'SQL'
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT quote_ident(schemaname) AS schemaname, quote_ident(tablename) AS tablename
        FROM pg_tables
        WHERE schemaname = 'public'
    LOOP
        EXECUTE format('TRUNCATE TABLE %s.%s RESTART IDENTITY CASCADE', r.schemaname, r.tablename);
    END LOOP;
END $$;
SQL

echo "==> Importing data from source database..."
dump_tables=""
for table in $TABLES; do
	dump_tables="$dump_tables -t $table"
done

# shellcheck disable=SC2086
docker compose exec -T postgres sh -c "
	set -eu
	pg_dump '$IMPORT_SOURCE_URL' --data-only --no-owner --no-acl $dump_tables \
	| psql -v ON_ERROR_STOP=1 -U postgres -d defi_pnl -q
"

echo ""
echo "==> Import complete. Row counts in Docker Postgres:"
docker compose exec -T postgres psql -U postgres -d defi_pnl -c \
	"SELECT 'daily_leaderboard' AS tbl, COUNT(*)::bigint FROM daily_leaderboard
	 UNION ALL SELECT 'trades_v2', COUNT(*)::bigint FROM trades_v2
	 UNION ALL SELECT 'pnl_leaderboard', COUNT(*)::bigint FROM pnl_leaderboard
	 UNION ALL SELECT 'subscribers', COUNT(*)::bigint FROM subscribers
	 UNION ALL SELECT 'pushed_signals', COUNT(*)::bigint FROM pushed_signals
	 ORDER BY tbl;"

echo ""
echo "Done. Start the stack again with: make run"
