# /bin/bash exists on macOS and Linux; /usr/bin/bash does not exist on stock
# macOS, which would otherwise break every target there.
SHELL := /bin/bash

DATABASE_URL ?= postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable

.PHONY: help setup start check check-backend check-cli check-web doctor db-up db-down db-reset db-migrate db-seed db-psql api-server worker cli-skills-snapshot

help: ## list common targets
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-reset:
	docker compose down -v
	docker compose up -d postgres

db-migrate:
	./scripts/db/apply-goose-migrations.sh "$(DATABASE_URL)"

db-seed:
	psql "$(DATABASE_URL)" -f scripts/db/seed-dev.sql

db-psql:
	psql "$(DATABASE_URL)"

api-server:
	cd backend && go run ./cmd/api-server

worker:
	cd backend && go run ./cmd/worker

# Regenerate the embedded Agent Skills snapshot the CLI ships
# (cli/internal/skills/snapshot) from the canonical web/content/agent-skills.
# Run after changing any skill; CI should fail if the result is uncommitted:
#   make cli-skills-snapshot && git diff --exit-code cli/internal/skills/snapshot
cli-skills-snapshot:
	node scripts/sync-cli-skills-snapshot.mjs

# --- Contributor entry points ---------------------------------------------

setup: ## one-command dev bootstrap (Postgres + Redis + migrations + web deps)
	@./scripts/dev/bootstrap.sh

start: ## boot the full local stack (Postgres, Redis, Temporal, API, worker)
	@./scripts/dev/start-local-stack.sh

doctor: ## check that the running local stack is healthy
	@./scripts/dev/doctor.sh

check: check-backend check-cli check-web ## build + vet/lint + test every module
	@echo "==> all checks passed"

check-backend: ## build, vet, and test the Go backend
	cd backend && go build ./... && go vet ./... && go test -short -race -count=1 ./...

check-cli: ## build, vet, and test the Go CLI
	cd cli && go build ./... && go vet ./... && go test -short -race -count=1 ./...

check-web: ## install, lint, type-check, and test the web app
	cd web && pnpm install && pnpm lint && npx tsc --noEmit && pnpm test
