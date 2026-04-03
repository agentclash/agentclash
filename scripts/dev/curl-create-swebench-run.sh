#!/usr/bin/env bash
set -euo pipefail

WORKSPACE_ID="${WORKSPACE_ID:-22222222-2222-2222-2222-222222222222}"
USER_ID="${USER_ID:-33333333-3333-3333-3333-333333333333}"
CHALLENGE_PACK_VERSION_ID="${CHALLENGE_PACK_VERSION_ID:-53535353-5555-5555-5555-555555555555}"
CHALLENGE_INPUT_SET_ID="${CHALLENGE_INPUT_SET_ID:-53535353-abab-abab-abab-abababababab}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"

AGENT_DEPLOYMENT_IDS="${AGENT_DEPLOYMENT_IDS:-53535353-e001-e001-e001-e001e001e001,53535353-e002-e002-e002-e002e002e002,53535353-e003-e003-e003-e003e003e003,53535353-e004-e004-e004-e004e004e004,53535353-e005-e005-e005-e005e005e005}"

json_array_from_csv() {
  local csv="$1"
  jq -Rn --arg csv "${csv}" '$csv | split(",") | map(select(length > 0))'
}

deployment_ids_json="$(json_array_from_csv "${AGENT_DEPLOYMENT_IDS}")"

echo "==> Checking API health"
curl -fsS "${API_BASE_URL}/healthz"
echo
echo
echo "==> Creating SWE-bench mini run"
jq -nc \
  --arg workspace_id "${WORKSPACE_ID}" \
  --arg challenge_pack_version_id "${CHALLENGE_PACK_VERSION_ID}" \
  --arg challenge_input_set_id "${CHALLENGE_INPUT_SET_ID}" \
  --argjson agent_deployment_ids "${deployment_ids_json}" \
  '{
    workspace_id: $workspace_id,
    challenge_pack_version_id: $challenge_pack_version_id,
    challenge_input_set_id: $challenge_input_set_id,
    agent_deployment_ids: $agent_deployment_ids
  }' | curl \
  -X POST "${API_BASE_URL}/v1/runs" \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  --data-binary @-
echo
