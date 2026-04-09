#!/usr/bin/env bash
# Verify that all components of the reasoning lane stack are running and healthy.
# Usage: ./scripts/reasoning/verify-stack.sh

set -euo pipefail

API_URL="${E2E_API_URL:-http://localhost:8080}"
REASONING_URL="${E2E_REASONING_URL:-http://localhost:8000}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass=0
fail=0

check() {
  local name="$1"
  local url="$2"
  if curl -sf --max-time 5 "$url" > /dev/null 2>&1; then
    echo -e "  ${GREEN}PASS${NC} $name ($url)"
    ((pass++))
  else
    echo -e "  ${RED}FAIL${NC} $name ($url)"
    ((fail++))
  fi
}

echo "=== Reasoning Lane Stack Verification ==="
echo ""
echo "Checking service health..."
check "PostgreSQL" "http://localhost:5432" || true  # pg doesn't serve HTTP
check "Go API server" "$API_URL/healthz"
check "Python reasoning service" "$REASONING_URL/healthz"

# PostgreSQL uses TCP not HTTP, check differently
echo ""
echo "Checking PostgreSQL..."
if pg_isready -h localhost -p 5432 -U agentclash > /dev/null 2>&1; then
  echo -e "  ${GREEN}PASS${NC} PostgreSQL (localhost:5432)"
  ((pass++))
else
  echo -e "  ${YELLOW}SKIP${NC} PostgreSQL (pg_isready not available or pg not running)"
fi

echo ""
echo "Checking Temporal..."
if curl -sf --max-time 5 "http://localhost:7233" > /dev/null 2>&1 || \
   curl -sf --max-time 5 "http://localhost:8233/api/v1/namespaces" > /dev/null 2>&1; then
  echo -e "  ${GREEN}PASS${NC} Temporal"
  ((pass++))
else
  echo -e "  ${YELLOW}WARN${NC} Temporal not detected on :7233 or :8233 (needed for full E2E)"
fi

echo ""
echo "=== Results: $pass passed, $fail failed ==="

if [[ $fail -gt 0 ]]; then
  echo ""
  echo "To start the stack:"
  echo "  make db-up db-migrate         # PostgreSQL"
  echo "  temporal server start-dev &   # Temporal (needs separate install)"
  echo "  make api-server &             # Go API server"
  echo "  REASONING_SERVICE_ENABLED=true make worker &  # Go worker"
  echo "  make reasoning-service &      # Python reasoning service"
  echo ""
  echo "Then re-run: $0"
  exit 1
fi

echo ""
echo "Stack is ready. Run E2E tests with:"
echo "  make test-e2e"
