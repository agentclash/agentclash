SHELL := /usr/bin/bash

DATABASE_URL ?= postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable

.PHONY: db-up db-down db-reset db-migrate db-psql api-server

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-reset:
	docker compose down -v
	docker compose up -d postgres

db-migrate:
	./scripts/db/apply-goose-migrations.sh "$(DATABASE_URL)"

db-psql:
	psql "$(DATABASE_URL)"

api-server:
	cd backend && go run ./cmd/api-server
