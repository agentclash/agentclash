#!/usr/bin/env bash
#
# AgentClash one-command dev bootstrap (Tier 2 / full stack).
#
# Idempotent and safe to re-run. Brings up Postgres + Redis, applies database
# migrations, and installs web dependencies so `make start` (or the individual
# api-server / worker / web processes) can run.
#
# Most contributors do NOT need this. Docs/web work needs only Node + pnpm, and
# CLI work needs only Go against the hosted API. See "Run AgentClash locally" in
# CONTRIBUTING.md for the tiered paths.
set -euo pipefail
cd "$(dirname "$0")/../.."   # repo root

note(){ printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn(){ printf '\033[1;33m!!\033[0m %s\n' "$*"; }
ok(){   printf '\033[1;32m✓\033[0m %s\n' "$*"; }

missing=0
need(){ if ! command -v "$1" >/dev/null 2>&1; then warn "Missing $1 — $2"; missing=1; fi; }

note "Checking prerequisites"
need go "Go 1.25+ (https://go.dev/dl/)"
need node "Node 22 (https://nodejs.org — see .nvmrc / .tool-versions)"
need pnpm "pnpm (corepack enable, or: npm i -g pnpm)"
need docker "Docker (https://docs.docker.com/get-docker/)"
need psql "psql / libpq client — needed for migrations & seeding (brew install libpq · apt install postgresql-client)"
need curl "curl — used for API readiness and local smoke checks"
# Temporal CLI is optional: docker compose now ships a temporal dev-server
# container, so the host CLI is only a fallback for 'make start'.
command -v temporal >/dev/null 2>&1 || note "temporal CLI not found — fine, 'make start' uses the docker container"

# Stop here rather than pressing on: continuing with a missing prerequisite
# either half-completes setup or dies later with an unrelated-looking error.
if [ "$missing" -ne 0 ]; then
  warn "Install the missing prerequisites above, then re-run 'make setup'."
  exit 1
fi

# Dev-safe env: backend/.env.example already encodes the zero-key dev profile
# (APP_ENV=development, AUTH_MODE=dev, SANDBOX_PROVIDER=unconfigured, ephemeral
# secrets key). Copying it means the backend boots with NO external API keys.
if [ -f backend/.env.example ] && [ ! -f backend/.env ]; then
  cp backend/.env.example backend/.env && ok "Created backend/.env (dev profile — boots with zero API keys)"
elif [ -f backend/.env ]; then
  note "backend/.env already exists — leaving it untouched"
fi

if ! command -v docker >/dev/null 2>&1; then
  warn "Docker not available — skipping Postgres/Redis startup and migrations."
  warn "Install Docker and re-run 'make setup' to finish backend setup."
else
  note "Starting Postgres + Redis"
  docker compose up -d postgres redis

  note "Waiting for Postgres to accept connections"
  pg_ready=0
  for _ in $(seq 1 30); do
    if docker compose exec -T postgres pg_isready -U agentclash -d agentclash >/dev/null 2>&1; then
      ok "Postgres is ready"; pg_ready=1; break
    fi
    sleep 1
  done

  # Without this branch the script fell through to 'make db-migrate' and died
  # with a connection error. The usual cause is Docker not running.
  if [ "$pg_ready" -ne 1 ]; then
    warn "Postgres did not accept connections after 30s."
    warn "Is Docker running? Start Docker Desktop (or your daemon), then re-run 'make setup'."
    warn "Inspect the container with: docker compose logs postgres"
    exit 1
  fi

  if command -v go >/dev/null 2>&1; then
    note "Running migrations"
    make db-migrate
  else
    warn "Skipping migrations — Go not found. Install Go, then run 'make db-migrate'."
  fi
fi

if command -v pnpm >/dev/null 2>&1; then
  note "Installing web dependencies"
  ( cd web && pnpm install )
else
  warn "Skipping web deps — pnpm not found. Run 'corepack enable' then 'cd web && pnpm install'."
fi

note "Done."
echo
echo "  Next:"
echo "    make start        # full local stack (Postgres, Redis, Temporal, API, worker)"
echo "    make status       # ownership, process state, health, and log sources"
echo "    make logs         # follow Docker and host logs together"
echo "    make stop         # stop the stack and preserve containers/data/logs"
echo "  Or run pieces individually:"
echo "    make api-server   # API on http://localhost:8080"
echo "    cd web && pnpm dev   # web on http://localhost:3000"
echo
echo "  Environment files:"
echo "    backend/.env      # created from backend/.env.example by this setup"
echo "    web/.env.local    # copy web/.env.local.example yourself before pnpm dev"
echo "  The web development server is not managed by make start/stop."
