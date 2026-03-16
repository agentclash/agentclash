#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"

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

export API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
export WORKSPACE_ID="${WORKSPACE_ID:-22222222-2222-2222-2222-222222222222}"
export USER_ID="${USER_ID:-33333333-3333-3333-3333-333333333333}"
export CHALLENGE_PACK_VERSION_ID="${CHALLENGE_PACK_VERSION_ID:-55555555-5555-5555-5555-555555555555}"
export AGENT_DEPLOYMENT_ID="${AGENT_DEPLOYMENT_ID:-eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee}"
export SCORECARD_TIMEOUT_SECONDS="${SCORECARD_TIMEOUT_SECONDS:-240}"
export POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-2}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq

echo "==> Checking API health"
curl -fsS "${API_BASE_URL}/healthz" >/dev/null

echo "==> Seeding local run fixture"
"${ROOT_DIR}/scripts/dev/seed-local-run-fixture.sh" >/dev/null

echo "==> Creating run"
create_response="$(curl -fsS \
  -X POST "${API_BASE_URL}/v1/runs" \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  -d '{
    "workspace_id": "'"${WORKSPACE_ID}"'",
    "challenge_pack_version_id": "'"${CHALLENGE_PACK_VERSION_ID}"'",
    "agent_deployment_ids": ["'"${AGENT_DEPLOYMENT_ID}"'"]
  }')"

run_id="$(jq -r '.id' <<<"${create_response}")"
if [[ -z "${run_id}" || "${run_id}" == "null" ]]; then
  echo "failed to parse run id from create response" >&2
  echo "${create_response}" >&2
  exit 1
fi
echo "    run_id: ${run_id}"

echo "==> Waiting for run-agent to materialize"
run_agent_id=""
for _ in $(seq 1 30); do
  agents_response="$(curl -fsS \
    -H "X-Agentclash-User-Id: ${USER_ID}" \
    -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
    "${API_BASE_URL}/v1/runs/${run_id}/agents")"
  run_agent_id="$(jq -r '.items[0].id // empty' <<<"${agents_response}")"
  if [[ -n "${run_agent_id}" ]]; then
    break
  fi
  sleep 1
done

if [[ -z "${run_agent_id}" ]]; then
  echo "run agent did not materialize for run ${run_id}" >&2
  exit 1
fi
echo "    run_agent_id: ${run_agent_id}"

echo "==> Polling scorecard endpoint until ready"
deadline=$((SECONDS + SCORECARD_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  http_code="$(curl -sS -o /tmp/agentclash-scorecard-smoke.json -w '%{http_code}' \
    -H "X-Agentclash-User-Id: ${USER_ID}" \
    -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
    "${API_BASE_URL}/v1/scorecards/${run_agent_id}")"

  case "${http_code}" in
    200)
      state="$(jq -r '.state' /tmp/agentclash-scorecard-smoke.json)"
      correctness_score="$(jq -r '.correctness_score // "null"' /tmp/agentclash-scorecard-smoke.json)"
      reliability_score="$(jq -r '.reliability_score // "null"' /tmp/agentclash-scorecard-smoke.json)"
      echo "==> Scorecard ready"
      echo "    state: ${state}"
      echo "    correctness_score: ${correctness_score}"
      echo "    reliability_score: ${reliability_score}"
      exit 0
      ;;
    202)
      state="$(jq -r '.state' /tmp/agentclash-scorecard-smoke.json)"
      message="$(jq -r '.message // ""' /tmp/agentclash-scorecard-smoke.json)"
      echo "    pending: state=${state} message=${message}"
      ;;
    409)
      echo "scorecard endpoint returned 409 for run_agent ${run_agent_id}" >&2
      cat /tmp/agentclash-scorecard-smoke.json >&2
      exit 1
      ;;
    *)
      echo "unexpected status from scorecard endpoint: ${http_code}" >&2
      cat /tmp/agentclash-scorecard-smoke.json >&2
      exit 1
      ;;
  esac

  sleep "${POLL_INTERVAL_SECONDS}"
done

echo "timed out waiting for scorecard readiness for run_agent ${run_agent_id}" >&2
exit 1
