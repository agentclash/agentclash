#!/usr/bin/env bash
set -euo pipefail

# This script runs a real OpenAI smoke test against the merged #41 adapter path.
# It intentionally uses the production HTTP client and the real OpenAI endpoint.
#
# Required:
#   OPENAI_API_KEY
#
# Optional:
#   OPENAI_MODEL           defaults to gpt-4.1-mini
#   GOCACHE                defaults to /tmp/go-build
#
# Example:
#   OPENAI_API_KEY=... scripts/smoke/openai-provider-smoke.sh

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"

if [[ -f "${BACKEND_DIR}/.env" ]]; then
  # Load existing local defaults without overriding explicitly exported vars.
  set -a
  # shellcheck disable=SC1091
  source "${BACKEND_DIR}/.env"
  set +a
fi

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "OPENAI_API_KEY is required" >&2
  exit 1
fi

export OPENAI_MODEL="${OPENAI_MODEL:-gpt-4.1-mini}"
export GOCACHE="${GOCACHE:-/tmp/go-build}"

echo "==> Running real OpenAI adapter smoke test"
echo "    model: ${OPENAI_MODEL}"
echo "    backend dir: ${BACKEND_DIR}"

cd "${BACKEND_DIR}"
go test -tags openaismoke ./internal/provider -run TestOpenAICompatibleClientSmoke -v
