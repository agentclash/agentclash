SHELL := /usr/bin/bash

DATABASE_URL ?= postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable

.PHONY: db-up db-down db-reset db-migrate db-seed db-psql api-server worker reasoning-service reasoning-test test-go test-python test-all

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

reasoning-service:
	cd reasoning && python -m uvicorn reasoning.app:app --host 0.0.0.0 --port 8000

reasoning-up:
	docker compose up -d reasoning

reasoning-test:
	cd reasoning && python -m pytest tests/ -v

test-go:
	cd backend && go test -short -race -count=1 ./...

test-python:
	cd reasoning && python -m pytest tests/ -v

test-all: test-go test-python

test-e2e:
	cd backend && go test -race -count=1 -tags=e2e ./internal/reasoning/...
