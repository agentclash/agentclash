# /bin/bash exists on macOS and Linux; /usr/bin/bash does not exist on stock
# macOS, which would otherwise break every target there.
SHELL := /bin/bash

DATABASE_URL ?= postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable

.PHONY: help setup start status logs stop restart doctor check check-dev check-backend check-cli check-runtime check-web db-up db-down db-reset db-migrate db-seed db-psql api-server worker cli-skills-snapshot

help: ## list common targets
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

db-up: ## start the Postgres container
	docker compose up -d postgres

db-down: ## remove Docker Compose containers and network (keeps volumes)
	docker compose down

db-reset: ## destroy and recreate the database (drops volumes)
	docker compose down -v
	docker compose up -d postgres

db-migrate: export DATABASE_URL := $(DATABASE_URL)
db-migrate: ## apply filename-ledger migrations to the dev database
	./scripts/db/apply-goose-migrations.sh

db-seed: ## load base dev rows (needs a psql client)
	psql "$(DATABASE_URL)" -f scripts/db/seed-dev.sql

db-psql: ## open a psql shell against the dev database
	psql "$(DATABASE_URL)"

api-server: ## run the API server on http://localhost:8080
	cd backend && go run ./cmd/api-server

worker: ## run the Temporal worker
	cd backend && go run ./cmd/worker

# Regenerate the embedded Agent Skills snapshot the CLI ships
# (cli/internal/skills/snapshot) from the canonical web/content/agent-skills.
# Run after changing any skill; CI should fail if the result is uncommitted:
#   make cli-skills-snapshot && git diff --exit-code cli/internal/skills/snapshot
cli-skills-snapshot: ## regenerate the CLI's embedded Agent Skills snapshot
	node scripts/sync-cli-skills-snapshot.mjs

# --- Contributor entry points ---------------------------------------------

setup: ## one-command dev bootstrap (Postgres + Redis + migrations + web deps)
	@./scripts/dev/bootstrap.sh

start: ## boot the full local stack (Postgres, Redis, Temporal, API, worker)
	@./scripts/dev/local-stack.sh start

status: ## show local-stack ownership, process state, and health
	@./scripts/dev/local-stack.sh status

logs: ## follow all local-stack logs (use FOLLOW=0 for a snapshot)
	@FOLLOW="$(FOLLOW)" TAIL="$(TAIL)" ./scripts/dev/local-stack.sh logs

stop: ## stop the local stack while preserving containers, volumes, and logs
	@./scripts/dev/local-stack.sh stop

restart: ## stop and start the complete local stack
	@./scripts/dev/local-stack.sh restart

doctor: status ## compatibility alias for local-stack status and health

check: check-dev check-backend check-cli check-runtime check-web ## build + vet/lint + test every module
	@echo "==> all checks passed"

check-dev: ## syntax-check and test contributor lifecycle scripts
	@bash -n scripts/dev/*.sh
	@./scripts/dev/local-stack-test.sh

check-backend: ## build, vet, and test the Go backend
	cd backend && go build ./... && go vet ./... && go test -short -race -count=1 ./...

check-cli: ## build, vet, and test the Go CLI
	cd cli && go build ./... && go vet ./... && go test -short -race -count=1 ./...

check-runtime: ## build, vet, and test the Go runtime module
	cd runtime && go build ./... && go vet ./... && go test -short -race -count=1 ./...

check-web: ## install, lint, type-check, and test the web app
	cd web && pnpm install && pnpm lint && pnpm exec tsc --noEmit && pnpm test
