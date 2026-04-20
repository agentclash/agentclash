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
export REPETITIONS="${REPETITIONS:-3}"
export SECOND_REPETITIONS="${SECOND_REPETITIONS:-}"
export EXPECT_STATUS="${EXPECT_STATUS:-queued}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq

create_session() {
  local repetitions="$1"
  local name="$2"

  curl -fsS \
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
      "name": "'"${name}"'",
      "eval_session": {
        "repetitions": '"${repetitions}"',
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
    }'
}

inspect_session() {
  local session_id="$1"
  local expected_repetitions="$2"
  local detail_file="/tmp/agentclash-eval-session-${session_id}.json"

  curl -fsS \
    -H "X-Agentclash-User-Id: ${USER_ID}" \
    -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
    "${API_BASE_URL}/v1/eval-sessions/${session_id}" >"${detail_file}"

  local status
  status="$(jq -r '.eval_session.status' "${detail_file}")"
  local total_runs
  total_runs="$(jq -r '.summary.run_counts.total' "${detail_file}")"
  local queued_runs
  queued_runs="$(jq -r '.summary.run_counts.queued' "${detail_file}")"
  local listed_runs
  listed_runs="$(jq -r '.runs | length' "${detail_file}")"
  local aggregate_is_null
  aggregate_is_null="$(jq -r '.aggregate_result == null' "${detail_file}")"
  local warning_present
  warning_present="$(jq -r '[.evidence_warnings[]? | contains("aggregate result unavailable")] | any' "${detail_file}")"

  if [[ "${status}" != "${EXPECT_STATUS}" ]]; then
    echo "eval session ${session_id} status = ${status}, want ${EXPECT_STATUS}" >&2
    cat "${detail_file}" >&2
    exit 1
  fi

  if [[ "${total_runs}" != "${expected_repetitions}" ]]; then
    echo "eval session ${session_id} total runs = ${total_runs}, want ${expected_repetitions}" >&2
    cat "${detail_file}" >&2
    exit 1
  fi

  if [[ "${queued_runs}" != "${expected_repetitions}" && "${EXPECT_STATUS}" == "queued" ]]; then
    echo "eval session ${session_id} queued runs = ${queued_runs}, want ${expected_repetitions}" >&2
    cat "${detail_file}" >&2
    exit 1
  fi

  if [[ "${listed_runs}" != "${expected_repetitions}" ]]; then
    echo "eval session ${session_id} run list length = ${listed_runs}, want ${expected_repetitions}" >&2
    cat "${detail_file}" >&2
    exit 1
  fi

  if [[ "${aggregate_is_null}" != "true" ]]; then
    echo "eval session ${session_id} aggregate_result should be null" >&2
    cat "${detail_file}" >&2
    exit 1
  fi

  if [[ "${warning_present}" != "true" ]]; then
    echo "eval session ${session_id} missing aggregate-result evidence warning" >&2
    cat "${detail_file}" >&2
    exit 1
  fi
}

echo "==> Checking API health"
curl -fsS "${API_BASE_URL}/healthz" >/dev/null

echo "==> Seeding local eval-session fixture"
"${ROOT_DIR}/scripts/dev/seed-local-run-fixture.sh" >/dev/null

echo "==> Creating primary eval session"
first_response="$(create_session "${REPETITIONS}" "Scale verification primary")"
first_session_id="$(jq -r '.eval_session.id' <<<"${first_response}")"
if [[ -z "${first_session_id}" || "${first_session_id}" == "null" ]]; then
  echo "failed to parse primary eval session id" >&2
  echo "${first_response}" >&2
  exit 1
fi

inspect_session "${first_session_id}" "${REPETITIONS}"

second_session_id=""
if [[ -n "${SECOND_REPETITIONS}" ]]; then
  echo "==> Creating secondary eval session"
  second_response="$(create_session "${SECOND_REPETITIONS}" "Scale verification secondary")"
  second_session_id="$(jq -r '.eval_session.id' <<<"${second_response}")"
  if [[ -z "${second_session_id}" || "${second_session_id}" == "null" ]]; then
    echo "failed to parse secondary eval session id" >&2
    echo "${second_response}" >&2
    exit 1
  fi
  inspect_session "${second_session_id}" "${SECOND_REPETITIONS}"
fi

echo "==> Listing eval sessions"
list_file="/tmp/agentclash-eval-session-list.json"
curl -fsS \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  "${API_BASE_URL}/v1/eval-sessions?workspace_id=${WORKSPACE_ID}&limit=10" >"${list_file}"

first_present="$(jq --arg id "${first_session_id}" '[.items[].eval_session.id == $id] | any' "${list_file}")"
if [[ "${first_present}" != "true" ]]; then
  echo "primary eval session ${first_session_id} missing from list response" >&2
  cat "${list_file}" >&2
  exit 1
fi

if [[ -n "${second_session_id}" ]]; then
  second_present="$(jq --arg id "${second_session_id}" '[.items[].eval_session.id == $id] | any' "${list_file}")"
  if [[ "${second_present}" != "true" ]]; then
    echo "secondary eval session ${second_session_id} missing from list response" >&2
    cat "${list_file}" >&2
    exit 1
  fi
fi

echo "==> Eval session read smoke passed"
echo "    first_session_id: ${first_session_id}"
if [[ -n "${second_session_id}" ]]; then
  echo "    second_session_id: ${second_session_id}"
fi
