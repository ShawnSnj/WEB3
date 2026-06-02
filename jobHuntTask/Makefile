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

.PHONY: run
run: ## Run the server locally (uses .env if present)
	$(GO) run ./cmd/server

.PHONY: dedupe-tasks
dedupe-tasks: ## Remove duplicate tasks (same title + due day)
	$(GO) run ./cmd/dedupe-tasks

.PHONY: build
build: ## Build a local binary into ./bin/
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
migrate: ## Apply migrations/deploy.sql against the running postgres container
	docker compose exec -T postgres psql -U jobhunt -d jobhunt -v ON_ERROR_STOP=1 \
		< migrations/deploy.sql

.PHONY: migrate-local
migrate-local: ## Apply deploy.sql using psql on the host (set DATABASE_URL or .env)
	psql "$${DATABASE_URL:-postgres://$${DB_USER:-jobhunt}:$${DB_PASSWORD:-jobhunt}@$${DB_HOST:-localhost}:$${DB_PORT:-5432}/$${DB_NAME:-jobhunt}?sslmode=$${DB_SSLMODE:-disable}}" \
		-v ON_ERROR_STOP=1 -f migrations/deploy.sql

.PHONY: migrate-down
migrate-down: ## Roll back using the latest versioned down.sql (see migrations/versions/)
	@f=$$(ls migrations/versions/*.down.sql 2>/dev/null | sort | tail -n 1); \
	if [ -z "$$f" ]; then echo "no down migration found"; exit 1; fi; \
	echo "rolling back $$f"; \
	docker compose exec -T postgres psql -U jobhunt -d jobhunt -v ON_ERROR_STOP=1 < "$$f"

## ---- ops ------------------------------------------------------------------

.PHONY: health
health: ## curl the health endpoint
	curl -fsS http://localhost:8082/healthz | jq .

.PHONY: ready
ready: ## curl the readiness endpoint
	curl -fsS http://localhost:8082/readyz | jq .
