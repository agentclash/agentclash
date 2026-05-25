#!/usr/bin/env bash
# Queue remaining spec-gaming races as Pro concurrency slots free up (max 3 concurrent).
set -euo pipefail
cd "$(dirname "$0")/../../cli"
export AGENTCLASH_API_URL="${AGENTCLASH_API_URL:-https://api.agentclash.dev}"
export AGENTCLASH_WORKSPACE="${AGENTCLASH_WORKSPACE:-511e2d3e-9076-4db3-b9f2-5ef54ab591d5}"

DEPLOYMENTS="aafe3f3e-8fb4-46db-bc11-41c4a9fd88ad,40d524cb-b675-4de4-8057-3945bed65e50,e728190a-8d1f-409a-90f6-044b7a60324c,b35d6e09-05ef-4d83-b296-68f7910309d3,75b855d3-43d1-4036-9b4a-ad1efae2cbae,d393d72a-6e81-45a8-867f-677b74e376c6,3faac352-8e01-4a8d-ae91-00e5c89b9cd7,31beac9e-ff2e-4c6b-90d0-29d7922bc7e6"

declare -A PENDING=(
  [sequence]="7968af91-671b-49c4-83a6-c5d6fdf0d6d0|Refund queue processing"
  [proxy]="376237ee-6655-4c05-8781-5659a565f149|Shopping cart unit tests"
  [special-casing]="6dbe74bf-f286-409a-87b0-ad233e8b3317|Credential redaction impl"
  [denial]="ba70f3fa-e911-4d0e-9eca-84edb297731d|Search performance optimization"
)

active_count() {
  go run . run list --json | python3 -c "
import json,sys
items=json.load(sys.stdin).get('items',[])
print(sum(1 for x in items if x.get('status') in ('queued','running','pending')))
"
}

start_run() {
  local key="$1" version="$2" name="$3"
  echo "Starting $key ($name)..."
  go run . run create \
    --challenge-pack-version "$version" \
    --deployments "$DEPLOYMENTS" \
    --name "$name" \
    --json
}

while ((${#PENDING[@]} > 0)); do
  slots=$((3 - $(active_count)))
  if ((slots <= 0)); then
    sleep 30
    continue
  fi
  for key in "${!PENDING[@]}"; do
    IFS='|' read -r version name <<< "${PENDING[$key]}"
    if out=$(start_run "$key" "$version" "$name" 2>&1); then
      echo "$out"
      unset 'PENDING[$key]'
      slots=$((slots - 1))
      ((slots <= 0)) && break
    elif grep -q concurrency_limit_exceeded <<<"$out"; then
      break
    else
      echo "Failed $key: $out" >&2
      unset 'PENDING[$key]'
    fi
  done
  sleep 15
done

echo "All pending spec-gaming runs queued."
