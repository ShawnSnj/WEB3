.PHONY: run run-detached stop logs down reset-db build psql integrate redeploy status import-db

# Start Postgres + API server in the foreground (logs stream in this terminal).
# Press Ctrl+C to stop.
run: .env
	@echo "Starting defi-pnl (foreground — logs below; Ctrl+C to stop)"
	@echo "  API:      http://localhost:$${HOST_PORT:-8081}"
	@echo "  Postgres: localhost:$${POSTGRES_PORT:-15432} (user/pass/db: postgres/postgres/defi_pnl)"
	@echo ""
	docker compose up --build

# Start in the background (detached).
run-detached: .env
	docker compose up --build -d
	@echo ""
	@echo "defi-pnl is up (background)"
	@echo "  API:      http://localhost:$${HOST_PORT:-8081}"
	@echo "  Postgres: localhost:$${POSTGRES_PORT:-15432}"
	@echo ""
	@echo "Follow logs: make logs"
	@echo "Stop:        make stop"

# Stop containers (keeps database volume).
stop down:
	docker compose down

# Follow service logs.
logs:
	docker compose logs -f

# Show container status and probe the API.
status:
	@docker compose ps
	@echo ""
	@code=$$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$${HOST_PORT:-8081}/leaderboard/daily?date=2025-06-14" 2>/dev/null || echo "000"); \
	if [ "$$code" = "200" ]; then \
		echo "API is up: http://localhost:$${HOST_PORT:-8081}"; \
	else \
		echo "API not reachable on http://localhost:$${HOST_PORT:-8081} (HTTP $$code)"; \
		echo "Try: make run   then: make logs"; \
	fi

# Rebuild the server image only.
build:
	docker compose build server

# Open a psql shell against the Docker Postgres instance.
psql:
	docker compose exec postgres psql -U postgres -d defi_pnl

# Apply migrations/deploy.sql to Postgres (run after any schema.sql change).
integrate:
	@test -f migrations/deploy.sql || (echo "migrations/deploy.sql not found" && exit 1)
	docker compose up -d postgres
	@echo "Waiting for Postgres..."
	@until docker compose exec -T postgres pg_isready -U postgres -d defi_pnl >/dev/null 2>&1; do sleep 1; done
	@echo "Applying migrations/deploy.sql..."
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d defi_pnl -f /migrations/deploy.sql
	@echo "Database integration complete."

# Apply database changes and restart the full stack (typical redeploy flow).
redeploy: integrate
	docker compose up --build -d
	@echo ""
	@echo "Redeploy complete — database updated and services restarted."

# Wipe Docker Postgres data, then import from DB_URL / SOURCE_DB_URL in .env.
import-db: .env
	@chmod +x scripts/import-db.sh
	@./scripts/import-db.sh

# Wipe the database volume and re-apply schema.sql on next start.
reset-db:
	docker compose down -v
	@echo "Database volume removed. Run 'make run' to recreate Postgres and schema."

# Create .env from the example when missing.
.env:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env from .env.example — set GRAPH_API_KEY before fetching subgraph data."; \
	fi
