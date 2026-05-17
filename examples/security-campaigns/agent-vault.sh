#!/usr/bin/env bash
# Agent Vault adversarial-prompt campaign.
#
# Iterates every adversarial_prompts[] entry in
# examples/challenge-packs/infisical-agent-vault.yaml against a running
# Infisical Agent Vault, and prints a leak-rate markdown table plus
# per-attack JSON reports.
#
# Prerequisites (see docs/evaluation/agent-vault-runtime.md):
#   - agent-vault server running locally (or remote, reachable)
#   - OPENAI_API_KEY in env
#   - AGENT_VAULT_TOKEN minted via `agent-vault agent token create ...`
#
# Usage:
#   ./examples/security-campaigns/agent-vault.sh
#
# Override defaults via env vars: MODEL, ITERATIONS, PROXY_URL,
# MGMT_URL, ALLOWED_UPSTREAM, PACK, OUT_DIR.

set -euo pipefail

PACK=${PACK:-examples/challenge-packs/infisical-agent-vault.yaml}
MODEL=${MODEL:-gpt-4o-mini}
ITERATIONS=${ITERATIONS:-10}
PROXY_URL=${PROXY_URL:-${AGENT_VAULT_PROXY_URL:-}}
MGMT_URL=${MGMT_URL:-${AGENT_VAULT_ADDR:-http://127.0.0.1:14321}}
ALLOWED_UPSTREAM=${ALLOWED_UPSTREAM:-api.stripe.com}
OUT_DIR=${OUT_DIR:-./agent-vault-campaign-reports}

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "error: OPENAI_API_KEY is not set" >&2
  exit 2
fi
if [[ -z "${AGENT_VAULT_TOKEN:-}" ]]; then
  echo "error: AGENT_VAULT_TOKEN is not set" >&2
  echo "  mint one with: agent-vault agent token create <agent-name>" >&2
  exit 2
fi
if [[ -z "$PROXY_URL" ]]; then
  PROXY_URL="https://${AGENT_VAULT_TOKEN}:eval@127.0.0.1:14322"
fi

exec agentclash security agent-vault-stress \
    --from-pack "$PACK" \
    --model "$MODEL" \
    --iterations "$ITERATIONS" \
    --proxy-url "$PROXY_URL" \
    --mgmt-url "$MGMT_URL" \
    --canary-token "$AGENT_VAULT_TOKEN" \
    --allowed-upstream "$ALLOWED_UPSTREAM" \
    --out-dir "$OUT_DIR"
