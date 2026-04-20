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
export CHALLENGE_INPUT_SET_ID="${CHALLENGE_INPUT_SET_ID:-abababab-abab-abab-abab-abababababab}"
export AGENT_BUILD_VERSION_ID="${AGENT_BUILD_VERSION_ID:-dddddddd-dddd-dddd-dddd-dddddddddddd}"

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

echo "==> Seeding local eval-session fixture"
"${ROOT_DIR}/scripts/dev/seed-local-run-fixture.sh" >/dev/null

echo "==> Creating eval session"
create_response="$(curl -fsS \
  -X POST "${API_BASE_URL}/v1/eval-sessions" \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  -d '{
    "workspace_id": "'"${WORKSPACE_ID}"'",
    "challenge_pack_version_id": "'"${CHALLENGE_PACK_VERSION_ID}"'",
    "challenge_input_set_id": "'"${CHALLENGE_INPUT_SET_ID}"'",
    "participants": [
      {
        "agent_build_version_id": "'"${AGENT_BUILD_VERSION_ID}"'",
        "label": "Primary"
      }
    ],
    "execution_mode": "single_agent",
    "name": "Smoke Eval Session",
    "eval_session": {
      "repetitions": 2,
      "aggregation": {
        "method": "mean",
        "report_variance": true,
        "confidence_interval": 0.95
      },
      "routing_task_snapshot": {
        "routing": { "mode": "single_agent" },
        "task": { "pack_version": "v1", "input_set": "default" }
      },
      "schema_version": 1
    }
  }')"

session_id="$(jq -r '.eval_session.id' <<<"${create_response}")"
run_count="$(jq -r '.run_ids | length' <<<"${create_response}")"
status="$(jq -r '.eval_session.status' <<<"${create_response}")"

if [[ -z "${session_id}" || "${session_id}" == "null" ]]; then
  echo "failed to parse eval session id from response" >&2
  echo "${create_response}" >&2
  exit 1
fi

if [[ "${run_count}" -lt 1 ]]; then
  echo "expected at least one child run id" >&2
  echo "${create_response}" >&2
  exit 1
fi

if [[ "${status}" != "queued" ]]; then
  echo "eval session status = ${status}, want queued" >&2
  echo "${create_response}" >&2
  exit 1
fi

echo "==> Verifying weighted_mean validation"
validation_status="$(curl -sS -o /tmp/agentclash-eval-session-validation.json -w '%{http_code}' \
  -X POST "${API_BASE_URL}/v1/eval-sessions" \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  -d '{
    "workspace_id": "'"${WORKSPACE_ID}"'",
    "challenge_pack_version_id": "'"${CHALLENGE_PACK_VERSION_ID}"'",
    "challenge_input_set_id": "'"${CHALLENGE_INPUT_SET_ID}"'",
    "participants": [
      {
        "agent_build_version_id": "'"${AGENT_BUILD_VERSION_ID}"'",
        "label": "Primary"
      }
    ],
    "execution_mode": "single_agent",
    "eval_session": {
      "repetitions": 2,
      "aggregation": {
        "method": "weighted_mean",
        "report_variance": true,
        "confidence_interval": 0.95
      },
      "routing_task_snapshot": {
        "routing": { "mode": "single_agent" },
        "task": { "pack_version": "v1", "input_set": "default" }
      },
      "schema_version": 1
    }
  }')"

if [[ "${validation_status}" != "422" ]]; then
  echo "validation status = ${validation_status}, want 422" >&2
  cat /tmp/agentclash-eval-session-validation.json >&2
  exit 1
fi

validation_code="$(jq -r '.errors[0].code' /tmp/agentclash-eval-session-validation.json)"
if [[ "${validation_code}" != "eval_session.reliability_weights.required" ]]; then
  echo "validation code = ${validation_code}, want eval_session.reliability_weights.required" >&2
  cat /tmp/agentclash-eval-session-validation.json >&2
  exit 1
fi

echo "==> Eval session smoke passed"
echo "    session_id: ${session_id}"
echo "    run_count: ${run_count}"
