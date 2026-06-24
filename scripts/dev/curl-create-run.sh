#!/usr/bin/env bash
set -euo pipefail

# Creates a run through the local API using the deterministic fixture from
# scripts/dev/seed-local-run-fixture.sh.
#
# This proves the local HTTP/API path. It does not, by itself, prove the
# OpenAI native execution path unless the worker is configured with a real
# sandbox provider and valid provider credentials.

WORKSPACE_ID="${WORKSPACE_ID:-22222222-2222-2222-2222-222222222222}"
USER_ID="${USER_ID:-33333333-3333-3333-3333-333333333333}"
CHALLENGE_PACK_VERSION_ID="${CHALLENGE_PACK_VERSION_ID:-55555555-5555-5555-5555-555555555555}"
AGENT_DEPLOYMENT_ID="${AGENT_DEPLOYMENT_ID:-eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee}"
CHALLENGE_INPUT_SET_ID="${CHALLENGE_INPUT_SET_ID:-abababab-abab-abab-abab-abababababab}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"

echo "==> Checking API health"
curl -fsS "${API_BASE_URL}/healthz"
echo
echo
echo "==> Creating run"
curl \
  -X POST "${API_BASE_URL}/v1/runs" \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: ${USER_ID}" \
  -H "X-Agentclash-Workspace-Memberships: ${WORKSPACE_ID}:workspace_admin" \
  -d '{
    "workspace_id": "'"${WORKSPACE_ID}"'",
    "challenge_pack_version_id": "'"${CHALLENGE_PACK_VERSION_ID}"'",
    "challenge_input_set_id": "'"${CHALLENGE_INPUT_SET_ID}"'",
    "agent_deployment_ids": ["'"${AGENT_DEPLOYMENT_ID}"'"]
  }'
echo
echo
echo "Note:"
echo "  If the worker is running with SANDBOX_PROVIDER=unconfigured, the run can"
echo "  still be created but native execution will fail before the OpenAI model is"
echo "  actually invoked."
