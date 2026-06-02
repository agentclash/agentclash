SHELL := /usr/bin/bash

DATABASE_URL ?= postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable

.PHONY: db-up db-down db-reset db-migrate db-seed db-psql api-server worker cli-skills-snapshot

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
