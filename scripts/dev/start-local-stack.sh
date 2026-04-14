#!/usr/bin/env bash
set -euo pipefail

# Starts the local AgentClash stack required for manual curl checks:
# - Postgres via docker-compose
# - schema migrations
# - Temporal dev server (requires the temporal CLI)
# - API server
# - worker
#
# Important:
# - This starts the API and worker in the background and writes logs/PIDs under
#   /tmp/agentclash-local-stack.
# - A successful curl flow does not automatically mean the OpenAI adapter ran.
#   Native execution still requires a real sandbox provider such as E2B.
#
# Optional:
#   OPENAI_API_KEY         exported into AGENTCLASH_SECRET_OPENAI for seeded native deployments
#   OPENAI_MODEL           defaults to gpt-4.1-mini for the seed fixture
#   DATABASE_URL           defaults from backend/.env.example if not set
#   TEMPORAL_HOST_PORT     defaults to localhost:7233
#   TEMPORAL_NAMESPACE     defaults to default

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
STATE_DIR="${STATE_DIR:-/tmp/agentclash-local-stack}"
API_LOG="${STATE_DIR}/api-server.log"
WORKER_LOG="${STATE_DIR}/worker.log"
TEMPORAL_LOG="${STATE_DIR}/temporal.log"
API_PID_FILE="${STATE_DIR}/api-server.pid"
WORKER_PID_FILE="${STATE_DIR}/worker.pid"
TEMPORAL_PID_FILE="${STATE_DIR}/temporal.pid"

mkdir -p "${STATE_DIR}"

if [[ -f "${BACKEND_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${BACKEND_DIR}/.env"
  set +a
elif [[ -f "${BACKEND_DIR}/.env.example" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${BACKEND_DIR}/.env.example"
  set +a
fi

export OPENAI_MODEL="${OPENAI_MODEL:-gpt-4.1-mini}"
export TEMPORAL_HOST_PORT="${TEMPORAL_HOST_PORT:-localhost:7233}"
export TEMPORAL_NAMESPACE="${TEMPORAL_NAMESPACE:-default}"
export GOCACHE="${GOCACHE:-/tmp/go-build}"

if [[ -n "${OPENAI_API_KEY:-}" && -z "${AGENTCLASH_SECRET_OPENAI:-}" ]]; then
  export AGENTCLASH_SECRET_OPENAI="${OPENAI_API_KEY}"
fi

port_open() {
  local host="$1"
  local port="$2"
  timeout 1 bash -lc ">/dev/tcp/${host}/${port}" >/dev/null 2>&1
}

wait_for_http() {
  local url="$1"
  local attempts="$2"
  local sleep_seconds="$3"

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${sleep_seconds}"
  done

  return 1
}

echo "==> Starting Postgres and Redis"
cd "${ROOT_DIR}"
make db-up
docker compose up -d redis

export REDIS_URL="${REDIS_URL:-redis://localhost:6379}"

echo "==> Applying migrations"
make db-migrate

if ! port_open "127.0.0.1" "7233"; then
  if ! command -v temporal >/dev/null 2>&1; then
    echo "Temporal is not reachable on localhost:7233 and the 'temporal' CLI is not installed." >&2
    echo "Install the Temporal CLI or start your own dev server, then rerun this script." >&2
    exit 1
  fi

  echo "==> Starting Temporal dev server"
  nohup temporal server start-dev \
    --ip 127.0.0.1 \
    --port 7233 \
    --namespace "${TEMPORAL_NAMESPACE}" \
    >"${TEMPORAL_LOG}" 2>&1 &
  echo $! >"${TEMPORAL_PID_FILE}"

  for ((i = 1; i <= 30; i++)); do
    if port_open "127.0.0.1" "7233"; then
      break
    fi
    sleep 1
  done

  if ! port_open "127.0.0.1" "7233"; then
    echo "Temporal dev server did not become ready. See ${TEMPORAL_LOG}" >&2
    exit 1
  fi
else
  echo "==> Temporal already reachable on localhost:7233"
fi

echo "==> Starting API server"
cd "${BACKEND_DIR}"
nohup go run ./cmd/api-server >"${API_LOG}" 2>&1 &
echo $! >"${API_PID_FILE}"

echo "==> Starting worker"
nohup go run ./cmd/worker >"${WORKER_LOG}" 2>&1 &
echo $! >"${WORKER_PID_FILE}"

echo "==> Waiting for API health"
if ! wait_for_http "http://localhost:8080/healthz" 30 1; then
  echo "API server did not become healthy. See ${API_LOG}" >&2
  exit 1
fi

echo
echo "Local stack is up."
echo "  API log:      ${API_LOG}"
echo "  Worker log:   ${WORKER_LOG}"
echo "  Temporal log: ${TEMPORAL_LOG}"
echo
echo "Next steps:"
echo "  1. scripts/dev/seed-local-run-fixture.sh"
echo "  2. scripts/dev/curl-create-run.sh"
echo
echo "Note:"
echo "  Without a real sandbox provider (for example E2B), a native run created"
echo "  through curl can still be queued and started, but it will not successfully"
echo "  execute the OpenAI-backed native path."
