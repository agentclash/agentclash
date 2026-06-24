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
export CHALLENGE_PACK_VERSION_ID="${CHALLENGE_PACK_VERSION_ID:-53535353-5555-5555-5555-555555555555}"
export CHALLENGE_INPUT_SET_ID="${CHALLENGE_INPUT_SET_ID:-53535353-abab-abab-abab-abababababab}"
export AGENT_DEPLOYMENT_IDS="${AGENT_DEPLOYMENT_IDS:-53535353-e001-e001-e001-e001e001e001,53535353-e002-e002-e002-e002e002e002,53535353-e003-e003-e003-e003e003e003,53535353-e004-e004-e004-e004e004e004,53535353-e005-e005-e005-e005e005e005}"
export RUN_TIMEOUT_SECONDS="${RUN_TIMEOUT_SECONDS:-420}"
export SCORECARD_TIMEOUT_SECONDS="${SCORECARD_TIMEOUT_SECONDS:-420}"
export POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-3}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq

headers=(
  -H "X-Agentclash-User-Id: ${USER_ID}"
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin"
)

json_array_from_csv() {
  local csv="$1"
  jq -Rn --arg csv "${csv}" '$csv | split(",") | map(select(length > 0))'
}

echo "==> Checking API health"
curl -fsS "${API_BASE_URL}/healthz" >/dev/null

echo "==> Seeding SWE-bench mini fixture"
bash "${ROOT_DIR}/scripts/dev/seed-swebench-pack-fixture.sh" >/dev/null

deployment_ids_json="$(json_array_from_csv "${AGENT_DEPLOYMENT_IDS}")"

echo "==> Creating run"
create_response="$(jq -nc \
  --arg workspace_id "${WORKSPACE_ID}" \
  --arg challenge_pack_version_id "${CHALLENGE_PACK_VERSION_ID}" \
  --arg challenge_input_set_id "${CHALLENGE_INPUT_SET_ID}" \
  --argjson agent_deployment_ids "${deployment_ids_json}" \
  '{
    workspace_id: $workspace_id,
    challenge_pack_version_id: $challenge_pack_version_id,
    challenge_input_set_id: $challenge_input_set_id,
    agent_deployment_ids: $agent_deployment_ids
  }' | curl -fsS \
    -X POST "${API_BASE_URL}/v1/runs" \
    -H "Content-Type: application/json" \
    "${headers[@]}" \
    --data-binary @-)"

run_id="$(jq -r '.id' <<<"${create_response}")"
if [[ -z "${run_id}" || "${run_id}" == "null" ]]; then
  echo "failed to parse run id from create response" >&2
  echo "${create_response}" >&2
  exit 1
fi
echo "    run_id: ${run_id}"

echo "==> Waiting for all run-agents"
deadline=$((SECONDS + RUN_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  agents_response="$(curl -fsS "${headers[@]}" "${API_BASE_URL}/v1/runs/${run_id}/agents")"
  agent_count="$(jq '.items | length' <<<"${agents_response}")"
  if [[ "${agent_count}" -eq 5 ]]; then
    unfinished="$(jq '[.items[] | select(.status != "completed" and .status != "failed" and .status != "cancelled")] | length' <<<"${agents_response}")"
    echo "    agents=${agent_count} unfinished=${unfinished}"
    if [[ "${unfinished}" -eq 0 ]]; then
      break
    fi
  else
    echo "    agents=${agent_count} waiting_for_materialization=true"
  fi
  sleep "${POLL_INTERVAL_SECONDS}"
done

agents_response="$(curl -fsS "${headers[@]}" "${API_BASE_URL}/v1/runs/${run_id}/agents")"
agent_count="$(jq '.items | length' <<<"${agents_response}")"
if [[ "${agent_count}" -ne 5 ]]; then
  echo "expected 5 run agents, got ${agent_count}" >&2
  echo "${agents_response}" >&2
  exit 1
fi

echo "==> Run-agent statuses"
jq -r '.items[] | "\(.lane_index)\t\(.label)\t\(.status)\t\(.id)"' <<<"${agents_response}"

run_agent_ids=()
while IFS= read -r value; do
  run_agent_ids+=("${value}")
done < <(jq -r '.items[].id' <<<"${agents_response}")

declare -A scorecards
declare -A replays

echo "==> Waiting for scorecards/replays"
deadline=$((SECONDS + SCORECARD_TIMEOUT_SECONDS))
for run_agent_id in "${run_agent_ids[@]}"; do
  while (( SECONDS < deadline )); do
    scorecard_code="$(curl -sS -o /tmp/agentclash-scorecard-${run_agent_id}.json -w '%{http_code}' \
      "${headers[@]}" \
      "${API_BASE_URL}/v1/scorecards/${run_agent_id}")"
    replay_code="$(curl -sS -o /tmp/agentclash-replay-${run_agent_id}.json -w '%{http_code}' \
      "${headers[@]}" \
      "${API_BASE_URL}/v1/replays/${run_agent_id}")"

    if [[ "${scorecard_code}" =~ ^(200|409)$ ]] && [[ "${replay_code}" =~ ^(200|409)$ ]]; then
      scorecards["${run_agent_id}"]="/tmp/agentclash-scorecard-${run_agent_id}.json"
      replays["${run_agent_id}"]="/tmp/agentclash-replay-${run_agent_id}.json"
      break
    fi

    sleep "${POLL_INTERVAL_SECONDS}"
  done
done

echo "==> Scorecard summary"
for run_agent_id in "${run_agent_ids[@]}"; do
  scorecard_path="${scorecards[${run_agent_id}]}"
  replay_path="${replays[${run_agent_id}]}"
  label="$(jq -r --arg run_agent_id "${run_agent_id}" '.items[] | select(.id == $run_agent_id) | .label' <<<"${agents_response}")"
  status="$(jq -r --arg run_agent_id "${run_agent_id}" '.items[] | select(.id == $run_agent_id) | .status' <<<"${agents_response}")"
  score_state="$(jq -r '.state' "${scorecard_path}")"
  correctness="$(jq -r '.correctness_score // "null"' "${scorecard_path}")"
  reliability="$(jq -r '.reliability_score // "null"' "${scorecard_path}")"
  latency="$(jq -r '.latency_score // "null"' "${scorecard_path}")"
  cost="$(jq -r '.cost_score // "null"' "${scorecard_path}")"
  replay_state="$(jq -r '.state' "${replay_path}")"
  final_output="$(jq -r '.replay.summary.final_output // .replay.summary.output // .message // ""' "${replay_path}" | tr '\n' ' ')"
  echo "    ${label}"
  echo "      status=${status} scorecard_state=${score_state} replay_state=${replay_state}"
  echo "      correctness=${correctness} reliability=${reliability} latency=${latency} cost=${cost}"
  if [[ -n "${final_output}" ]]; then
    echo "      final_output=${final_output}"
  fi
done

echo "==> Fetching ranking"
ranking_response="$(curl -fsS "${headers[@]}" "${API_BASE_URL}/v1/runs/${run_id}/ranking?sort_by=composite")"
echo "==> Composite ranking"
jq -r '.ranking.items[] | "\(.rank // "null")\t\(.label)\t\(.sort_state)\t\(.composite_score // "null")\t\(.delta_from_top // "null")"' <<<"${ranking_response}"

echo "==> Ranking payload"
echo "${ranking_response}" | jq .
