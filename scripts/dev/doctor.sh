#!/usr/bin/env bash
#
# AgentClash local-stack health check.
#
# Confirms the *running* dev stack is reachable and prints next steps. This is a
# RUNTIME check — for build/vet/lint/test gating use `make check` instead.
set -uo pipefail
cd "$(dirname "$0")/../.."

green(){ printf '\033[1;32m✓\033[0m %s\n' "$*"; }
red(){   printf '\033[1;31m✗\033[0m %s\n' "$*"; }
note(){  printf '\033[1;34m==>\033[0m %s\n' "$*"; }

fail=0

port_open(){ # host port
  timeout 1 bash -c ">/dev/tcp/$1/$2" >/dev/null 2>&1
}

check_port(){ # label host port
  if port_open "$2" "$3"; then
    green "$1 reachable ($2:$3)"
  else
    red "$1 not reachable ($2:$3)"; fail=1
  fi
}

note "Checking AgentClash local stack"
check_port "Postgres" 127.0.0.1 5432
check_port "Redis"    127.0.0.1 6379
check_port "Temporal" 127.0.0.1 7233

# Only /healthz is a registered route on the API server (there is no /healthz/ready).
if curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; then
  green "API server healthy (http://localhost:8080/healthz)"
else
  red "API server not responding on http://localhost:8080/healthz"; fail=1
fi

echo
if [ "$fail" -eq 0 ]; then
  green "Stack looks healthy."
  echo "   → open http://localhost:3000   (web)"
  echo "   → Temporal UI: http://localhost:8233"
else
  red "Some checks failed. Bring the stack up with 'make start' (run 'make setup' first if you haven't)."
  exit 1
fi
