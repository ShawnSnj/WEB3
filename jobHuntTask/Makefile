.DEFAULT_GOAL := help

APP_NAME := jobhunt-task
BIN_DIR  := bin
BIN      := $(BIN_DIR)/server

GO       := go
GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

## ---- dev ------------------------------------------------------------------

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: db-up
db-up: ## Start postgres container (uses DB_HOST_PORT from .env)
	@if ! docker info >/dev/null 2>&1; then \
		echo "Docker is not running. Start Docker Desktop, then run: make db-up"; \
		exit 1; \
	fi
	docker compose up -d postgres

# ensure-db: use existing postgres on DB_HOST:DB_PORT, or start docker postgres.
.PHONY: ensure-db
ensure-db:
	@set -a && [ -f .env ] && . ./.env && set +a; \
	host="$${DB_HOST:-localhost}"; port="$${DB_PORT:-5432}"; user="$${DB_USER:-jobhunt}"; \
	if command -v pg_isready >/dev/null 2>&1 && pg_isready -h "$$host" -p "$$port" -U "$$user" >/dev/null 2>&1; then \
		echo "PostgreSQL ready at $$host:$$port"; \
	elif docker info >/dev/null 2>&1; then \
		$(MAKE) db-up wait-db; \
	else \
		echo ""; \
		echo "Cannot connect to PostgreSQL at $$host:$$port and Docker is not running."; \
		echo ""; \
		echo "  Option A — Docker (recommended):"; \
		echo "    1. Open Docker Desktop and wait until it is running"; \
		echo "    2. make migrate-all   # first time only"; \
		echo "    3. make run"; \
		echo ""; \
		echo "  Option B — Local Postgres (no Docker):"; \
		echo "    Install/start Postgres, set DB_HOST/DB_PORT in .env, then make run-server"; \
		echo ""; \
		exit 1; \
	fi

.PHONY: stop-app
stop-app: ## Stop docker app container (frees port 8082 for make run)
	docker compose stop app

.PHONY: check-port
check-port: ## Fail if HTTP port (8082) is already in use
	@port=$$(grep '^HTTP_PORT=' .env 2>/dev/null | cut -d= -f2 | tr -d ' "' || echo 8082); \
	if lsof -nP -iTCP:$$port -sTCP:LISTEN >/dev/null 2>&1; then \
		echo ""; \
		echo "Port $$port is already in use."; \
		if docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^jobhunt-app$$'; then \
			echo "  Docker jobhunt-app is running → http://localhost:$$port"; \
			echo "  Stop it:  docker compose stop app"; \
			echo "  Or use it as-is (no need for make run)."; \
		else \
			echo "  Free the port:  kill \$$(lsof -t -i:$$port)"; \
		fi; \
		echo ""; \
		exit 1; \
	fi

.PHONY: run-server
run-server: ensure-db check-port ## Run Go server only (skip CRM rebuild)
	$(GO) run ./cmd/server

.PHONY: run
run: ensure-db check-port build-crm ## Build CRM + start server (starts postgres via Docker if needed)
	$(GO) run ./cmd/server

.PHONY: dedupe-tasks
dedupe-tasks: ## Remove duplicate tasks (same title, same or different due days)
	$(GO) run ./cmd/dedupe-tasks

.PHONY: reschedule-tasks
reschedule-tasks: ## Spread today+overdue tasks to future days (2/day default)
	$(GO) run ./cmd/reschedule-tasks

.PHONY: build-crm
build-crm: ## Build Next.js CRM UI into internal/web/static/crm (embedded in Go binary)
	cd frontend && npm ci 2>/dev/null || npm install
	cd frontend && NEXT_PUBLIC_API_URL= npm run build
	find internal/web/static/crm -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true
	cp -R frontend/out/. internal/web/static/crm/
	@echo "CRM UI embedded at /crm"

.PHONY: build
build: build-crm ## Build CRM UI + local binary into ./bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/server
	@echo "built: $(BIN)"

.PHONY: vet
vet: ## go vet ./...
	$(GO) vet ./...

.PHONY: fmt
fmt: ## gofmt -s -w .
	gofmt -s -w .

.PHONY: test
test: ## Run unit tests
	$(GO) test -race -count=1 ./...

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

## ---- docker ---------------------------------------------------------------

.PHONY: docker-build
docker-build: ## Build the docker image
	docker build -t $(APP_NAME):latest .

.PHONY: up
up: ## Start the full stack (postgres + app)
	docker compose up -d --build

.PHONY: down
down: ## Stop the stack
	docker compose down

.PHONY: logs
logs: ## Tail app logs
	docker compose logs -f app

.PHONY: psql
psql: ## Open a psql shell in the postgres container
	docker compose exec postgres psql -U jobhunt -d jobhunt

.PHONY: migrate
migrate: migrate-all ## Alias: apply full unified schema (tasks + CRM)

.PHONY: migrate-all
migrate-all: wait-db ## Apply unified schema (tasks + CRM) to docker postgres
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/deploy.sql

.PHONY: migrate-crm
migrate-crm: wait-db ## Add CRM tables only (if DB predates CRM merge)
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/crm.sql

.PHONY: migrate-career-os
migrate-career-os: wait-db ## Add Career OS intelligence layer tables
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/career_os.sql

.PHONY: migrate-resume-intelligence
migrate-resume-intelligence: wait-db ## Phase 1: candidate profile + resume documents
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/resume_intelligence.sql

.PHONY: migrate-job-fit
migrate-job-fit: wait-db ## Phase 2: job fit analysis columns on job_matches
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/job_fit.sql

.PHONY: migrate-job-fetch
migrate-job-fetch: wait-db ## Job fetch audit log + URL dedupe index
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/job_fetch_runs.sql

.PHONY: migrate-job-fit-engine
migrate-job-fit-engine: wait-db ## Personalized job fit engine columns
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/job_fit_engine.sql

.PHONY: migrate-task-notes-structured
migrate-task-notes-structured: wait-db ## Structured task note types for job hunt dashboard
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/task_notes_structured.sql

.PHONY: migrate-task-notes-marked
migrate-task-notes-marked: wait-db ## Pin task note to tasks page header
	docker compose exec -T postgres psql -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} -v ON_ERROR_STOP=1 \
		< migrations/task_notes_marked.sql

.PHONY: migrate-local
migrate-local: migrate-all-local ## Alias: full schema via host psql

.PHONY: migrate-all-local
migrate-all-local: ## Apply unified schema via host psql (reads .env)
	@set -a && [ -f .env ] && . ./.env && set +a; \
	psql "$${DATABASE_URL:-postgres://$${DB_USER:-jobhunt}:$${DB_PASSWORD:-jobhunt}@$${DB_HOST:-localhost}:$${DB_PORT:-5432}/$${DB_NAME:-jobhunt}?sslmode=$${DB_SSLMODE:-disable}}" \
		-v ON_ERROR_STOP=1 -f migrations/deploy.sql

.PHONY: wait-db
wait-db: ## Wait until postgres in docker-compose is ready
	@if ! docker info >/dev/null 2>&1; then \
		echo "Docker is not running. Start Docker Desktop, then: make db-up"; \
		exit 1; \
	fi
	@echo "Waiting for PostgreSQL..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do \
		docker compose exec -T postgres pg_isready -U $${DB_USER:-jobhunt} -d $${DB_NAME:-jobhunt} >/dev/null 2>&1 && exit 0; \
		sleep 1; \
	done; \
	echo "PostgreSQL not ready. Run: make db-up  (port $${DB_HOST_PORT:-5432} in .env)"; exit 1

.PHONY: integrate
integrate: ## One DB + one server: start postgres/kafka, migrate, run app
	@if ! docker info >/dev/null 2>&1; then \
		echo "Docker is not running. Start Docker Desktop first."; exit 1; \
	fi
	docker compose up -d postgres redpanda
	$(MAKE) migrate-all
	@echo ""
	@echo "==> Unified server (tasks + CRM API): http://localhost:8082"
	@echo "==> Task tracker UI:                  http://localhost:8082/dashboard"
	@echo "==> CRM API:                          http://localhost:8082/api/v1/crm/dashboard"
	@echo "==> Job CRM UI:                       http://localhost:8082/crm/"
	@echo ""
	$(GO) run ./cmd/server

.PHONY: integrate-up
integrate-up: ## Full docker stack: DB + unified API + kafka worker
	docker compose up -d --build
	$(MAKE) migrate-all
	@echo ""
	@echo "Task tracker: http://localhost:8082/dashboard"
	@echo "Job CRM:      http://localhost:8082/crm/"
	@echo "CRM API:      http://localhost:8082/api/v1/crm/dashboard"

.PHONY: migrate-down
migrate-down: ## Roll back using the latest versioned down.sql (see migrations/versions/)
	@f=$$(ls migrations/versions/*.down.sql 2>/dev/null | sort | tail -n 1); \
	if [ -z "$$f" ]; then echo "no down migration found"; exit 1; fi; \
	echo "rolling back $$f"; \
	docker compose exec -T postgres psql -U jobhunt -d jobhunt -v ON_ERROR_STOP=1 < "$$f"

.PHONY: frontend-dev
frontend-dev: ## Dev-only: Next.js CRM on :3000 (production uses embedded /crm)
	cd frontend && npm install && NEXT_PUBLIC_API_URL=http://localhost:8082 npm run dev

.PHONY: crm-collect
crm-collect: ## Trigger job collection + scoring via API
	curl -fsS -X POST http://localhost:8082/api/v1/crm/collect | jq .

.PHONY: crm-dashboard
crm-dashboard: ## Fetch today's CRM brief
	curl -fsS http://localhost:8082/api/v1/crm/dashboard | jq .

## ---- ops ------------------------------------------------------------------

.PHONY: health
health: ## curl the health endpoint
	curl -fsS http://localhost:8082/healthz | jq .

.PHONY: ready
ready: ## curl the readiness endpoint
	curl -fsS http://localhost:8082/readyz | jq .
