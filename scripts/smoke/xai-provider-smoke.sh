#!/usr/bin/env bash
set -euo pipefail

# This script runs a real xAI smoke test against the xAI adapter path.
#
# Required:
#   XAI_API_KEY
#
# Optional:
#   XAI_MODEL             defaults to grok-4-1-fast-reasoning
#   GOCACHE               defaults to /tmp/go-build
#
# Example:
#   XAI_API_KEY=... scripts/smoke/xai-provider-smoke.sh

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"

if [[ -f "${BACKEND_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${BACKEND_DIR}/.env"
  set +a
fi

if [[ -z "${XAI_API_KEY:-}" ]]; then
  echo "XAI_API_KEY is required" >&2
  exit 1
fi

export XAI_MODEL="${XAI_MODEL:-grok-4-1-fast-reasoning}"
export GOCACHE="${GOCACHE:-/tmp/go-build}"

echo "==> Running real xAI adapter smoke test"
echo "    model: ${XAI_MODEL}"
echo "    backend dir: ${BACKEND_DIR}"

cd "${BACKEND_DIR}"
go test -tags xaismoke ./internal/provider -run TestXAIProviderSmoke -v
