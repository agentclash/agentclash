#!/usr/bin/env bash
set -Eeuo pipefail

# Smoke-test an installed AgentClash CLI after `agentclash auth login`.
#
# Optional environment:
#   AGENTCLASH_BIN=/path/to/agentclash
#   AGENTCLASH_API_URL=https://api.example.com
#   EXPECTED_AGENTCLASH_VERSION=0.1.2
#   RUN_NEGATIVE=0

BIN="${AGENTCLASH_BIN:-agentclash}"
EXPECTED_VERSION="${EXPECTED_AGENTCLASH_VERSION:-}"
RUN_NEGATIVE="${RUN_NEGATIVE:-1}"

BASE=("$BIN" --no-color)
JSON=("$BIN" --no-color --json)
if [[ -n "${AGENTCLASH_API_URL:-}" ]]; then
  BASE+=(--api-url "$AGENTCLASH_API_URL")
  JSON+=(--api-url "$AGENTCLASH_API_URL")
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

passed=0
failed=0

say() {
  printf '\n==> %s\n' "$*"
}

ok() {
  passed=$((passed + 1))
  printf 'ok - %s\n' "$*"
}

fail() {
  failed=$((failed + 1))
  printf 'not ok - %s\n' "$*" >&2
}

slug() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9_' '_'
}

capture() {
  local name="$1"
  shift
  local file="$tmpdir/$(slug "$name").out"

  set +e
  "$@" >"$file" 2>&1
  local code=$?
  set -e

  printf '%s:%s\n' "$code" "$file"
}

expect_success() {
  local name="$1"
  shift
  local result code file
  result="$(capture "$name" "$@")"
  code="${result%%:*}"
  file="${result#*:}"

  if [[ "$code" -eq 0 ]]; then
    ok "$name"
    return 0
  fi

  fail "$name exited $code"
  sed -n '1,80p' "$file" >&2
  return 1
}

expect_failure_with_output() {
  local name="$1"
  shift
  local result code file
  result="$(capture "$name" "$@")"
  code="${result%%:*}"
  file="${result#*:}"

  if [[ "$code" -eq 0 ]]; then
    fail "$name unexpectedly succeeded"
    sed -n '1,80p' "$file" >&2
    return 1
  fi
  if [[ ! -s "$file" ]]; then
    fail "$name failed silently"
    return 1
  fi

  ok "$name"
}

expect_output_contains() {
  local name="$1"
  local needle="$2"
  shift 2
  local result code file
  result="$(capture "$name" "$@")"
  code="${result%%:*}"
  file="${result#*:}"

  if [[ "$code" -ne 0 ]]; then
    fail "$name exited $code"
    sed -n '1,80p' "$file" >&2
    return 1
  fi
  if ! grep -Fq "$needle" "$file"; then
    fail "$name did not contain '$needle'"
    sed -n '1,80p' "$file" >&2
    return 1
  fi

  ok "$name"
}

validate_json_file() {
  local name="$1"
  local file="$2"

  if command -v jq >/dev/null 2>&1; then
    if jq -e . "$file" >/dev/null; then
      ok "$name is valid JSON"
    else
      fail "$name is not valid JSON"
      sed -n '1,80p' "$file" >&2
      return 1
    fi
  else
    ok "$name JSON parse skipped because jq is not installed"
  fi
}

config_value() {
  "${BASE[@]}" config get "$1" 2>/dev/null || true
}

say "Binary"
if command -v "$BIN" >/dev/null 2>&1; then
  ok "found $BIN at $(command -v "$BIN")"
else
  fail "could not find $BIN on PATH"
  exit 1
fi

version_result="$(capture version "${BASE[@]}" version)"
version_code="${version_result%%:*}"
version_file="${version_result#*:}"
if [[ "$version_code" -eq 0 ]]; then
  ok "agentclash version runs"
  sed -n '1,8p' "$version_file"
else
  fail "agentclash version exited $version_code"
  sed -n '1,80p' "$version_file" >&2
fi

if [[ -n "$EXPECTED_VERSION" ]]; then
  if grep -Fq "agentclash $EXPECTED_VERSION" "$version_file"; then
    ok "version is $EXPECTED_VERSION"
  else
    fail "version is not $EXPECTED_VERSION"
  fi
fi

say "Resolved Config"
expect_success "config list" "${BASE[@]}" config list || true
config_api_url="$(config_value api_url)"
config_org_id="$(config_value default_org)"
config_workspace_id="$(config_value default_workspace)"

if [[ -n "${AGENTCLASH_API_URL:-}" ]]; then
  ok "using AGENTCLASH_API_URL=$AGENTCLASH_API_URL"
elif [[ -n "$config_api_url" ]]; then
  ok "using configured api_url=$config_api_url"
else
  printf 'note - AGENTCLASH_API_URL is not set; CLI will use config api_url or localhost default\n'
fi
if [[ -n "${AGENTCLASH_TOKEN:-}" ]]; then
  printf 'note - AGENTCLASH_TOKEN is set and will take precedence over stored login credentials\n'
fi

say "Authenticated Read Checks"
expect_success "auth status" "${BASE[@]}" auth status || true

status_json_result="$(capture "auth status json" "${JSON[@]}" auth status)"
status_json_code="${status_json_result%%:*}"
status_json_file="${status_json_result#*:}"
if [[ "$status_json_code" -eq 0 ]]; then
  ok "auth status --json"
  validate_json_file "auth status --json" "$status_json_file" || true
  if grep -Eq '"(user_id|email)"' "$status_json_file"; then
    ok "auth status --json includes identity"
  else
    fail "auth status --json did not include identity"
    sed -n '1,80p' "$status_json_file" >&2
  fi
else
  fail "auth status --json exited $status_json_code"
  sed -n '1,80p' "$status_json_file" >&2
fi

expect_output_contains "auth login --device no-ops when already logged in" "Already logged in" "${BASE[@]}" auth login --device || true
expect_success "auth tokens list" "${BASE[@]}" auth tokens list || true

tokens_json_result="$(capture "auth tokens list json" "${JSON[@]}" auth tokens list)"
tokens_json_code="${tokens_json_result%%:*}"
tokens_json_file="${tokens_json_result#*:}"
if [[ "$tokens_json_code" -eq 0 ]]; then
  ok "auth tokens list --json"
  validate_json_file "auth tokens list --json" "$tokens_json_file" || true
else
  fail "auth tokens list --json exited $tokens_json_code"
  sed -n '1,80p' "$tokens_json_file" >&2
fi

say "Authenticated API Surface"
expect_success "org list" "${BASE[@]}" org list || true

if [[ -n "${AGENTCLASH_ORG:-$config_org_id}" ]]; then
  expect_success "workspace list" "${BASE[@]}" workspace list || true
else
  ok "workspace list skipped because no AGENTCLASH_ORG/default_org is set"
fi

if [[ -n "${AGENTCLASH_WORKSPACE:-$config_workspace_id}" ]]; then
  expect_success "challenge-pack list" "${BASE[@]}" challenge-pack list || true
else
  ok "challenge-pack list skipped because no AGENTCLASH_WORKSPACE/default_workspace is set"
fi

expect_success "infra model-catalog list" "${BASE[@]}" infra model-catalog list || true

if [[ "$RUN_NEGATIVE" != "0" ]]; then
  say "Negative Check"
  expect_failure_with_output \
    "invalid AGENTCLASH_TOKEN fails with visible error" \
    env AGENTCLASH_TOKEN="agentclash_invalid_smoke_token" "${BASE[@]}" auth status || true
fi

say "Summary"
printf 'passed: %d\nfailed: %d\n' "$passed" "$failed"

if [[ "$failed" -ne 0 ]]; then
  printf '\nFailure output is above. Re-run with AGENTCLASH_API_URL set if the CLI is pointing at localhost by accident.\n' >&2
  exit 1
fi

cat <<'EOF'

Manual checks worth doing once:
  agentclash auth login --force --device
    Expect: printed URL + code, browser approval completes login.

  agentclash auth login --force --device
    Then click Cancel/Deny in browser.
    Expect: terminal exits non-zero with "authorization denied by user".

EOF
