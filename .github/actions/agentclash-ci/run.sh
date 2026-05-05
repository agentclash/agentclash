#!/usr/bin/env bash
set -Eeuo pipefail

bool_true() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    1 | true | yes | y | on) return 0 ;;
    *) return 1 ;;
  esac
}

write_output() {
  local key="$1"
  local value="$2"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    printf '%s=%s\n' "$key" "$value" >>"$GITHUB_OUTPUT"
  fi
}

append_summary() {
  if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    printf '%s\n' "$1" >>"$GITHUB_STEP_SUMMARY"
  fi
}

json_get() {
  local file="$1"
  local path="$2"
  python3 - "$file" "$path" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    value = json.load(handle)

for part in sys.argv[2].split("."):
    if not part:
        continue
    if isinstance(value, dict):
        value = value.get(part)
    else:
        value = None
    if value is None:
        break

if isinstance(value, bool):
    print("true" if value else "false")
elif value is None:
    print("")
else:
    print(value)
PY
}

post_pr_comment() {
  local status="$1"
  if ! bool_true "${INPUT_PR_COMMENT:-true}"; then
    comment_posted=true
    return 0
  fi

  set +e
  python3 "${ACTION_PATH}/comment.py" \
    --manifest "$manifest" \
    --result-file "$result_file" \
    --should-run-file "$should_run_file" \
    --exit-code "$status" \
    --enabled "${INPUT_PR_COMMENT:-true}" \
    --repo "${GITHUB_REPOSITORY:-}" \
    --event-path "${GITHUB_EVENT_PATH:-}" \
    --api-url "${GITHUB_API_URL:-https://api.github.com}"
  local comment_status=$?
  set -e

  if [[ "$comment_status" -ne 0 ]]; then
    echo "::notice::AgentClash CI PR comment helper exited with status ${comment_status}"
  fi
  comment_posted=true
}

write_early_error_result() {
  if [[ -s "$result_file" ]]; then
    return 0
  fi
  mkdir -p "$(dirname "$result_file")"
  printf '%s\n' '{"gate_verdict":"error","failure_reason":"action_failed_before_ci_run","errors":["AgentClash action failed before ci run completed. Inspect the GitHub Actions log for the failing command."]}' >"$result_file"
}

on_error() {
  local status="$?"
  trap - ERR
  if [[ "${comment_posted:-false}" != "true" ]]; then
    write_early_error_result
    post_pr_comment "$status" || true
  fi
  exit "$status"
}

manifest="${INPUT_MANIFEST:-.agentclash/ci.yaml}"
artifact_dir="${INPUT_ARTIFACT_DIR:-agentclash-artifacts}"
result_file="${INPUT_RESULT_FILE:-agentclash-ci-result.json}"
should_run_file="${RUNNER_TEMP:-/tmp}/agentclash-should-run.json"
comment_posted=false
trap on_error ERR

write_output "result-file" "$result_file"
write_output "artifact-dir" "$artifact_dir"

if [[ -n "${INPUT_TOKEN:-}" ]]; then
  export AGENTCLASH_TOKEN="$INPUT_TOKEN"
fi
if [[ -n "${INPUT_WORKSPACE:-}" ]]; then
  export AGENTCLASH_WORKSPACE="$INPUT_WORKSPACE"
fi
export AGENTCLASH_API_URL="${INPUT_API_URL:-${AGENTCLASH_API_URL:-https://api.agentclash.dev}}"

if bool_true "${INPUT_INSTALL_CLI:-true}"; then
  npm install --global "agentclash@${INPUT_CLI_VERSION:-latest}"
fi

if bool_true "${INPUT_REMOTE_VALIDATE:-true}"; then
  agentclash ci validate "$manifest" --remote
else
  agentclash ci validate "$manifest"
fi

should_args=(ci should-run --manifest "$manifest" --json)
if [[ -n "${INPUT_CHANGED_FILES:-}" ]]; then
  while IFS= read -r changed_file; do
    [[ -z "$changed_file" ]] && continue
    should_args+=(--changed-file "$changed_file")
  done <<<"${INPUT_CHANGED_FILES}"
else
  base="${INPUT_BASE:-}"
  head="${INPUT_HEAD:-}"
  if [[ -z "$base" && -n "${GITHUB_BASE_REF:-}" ]]; then
    base="origin/${GITHUB_BASE_REF}"
  fi
  if [[ -z "$head" ]]; then
    head="HEAD"
  fi
  if [[ -n "$base" ]]; then
    should_args+=(--base "$base" --head "$head")
  fi
fi
if [[ -n "${INPUT_LABELS:-}" ]]; then
  should_args+=(--labels "${INPUT_LABELS}")
fi

agentclash "${should_args[@]}" >"$should_run_file"
should_run="$(json_get "$should_run_file" should_run)"
skip_reason="$(json_get "$should_run_file" reason)"
write_output "should-run" "$should_run"
write_output "skip-reason" "$skip_reason"

skip_if_unmatched=false
if bool_true "${INPUT_SKIP_IF_UNMATCHED:-true}"; then
  skip_if_unmatched=true
fi

if [[ "$should_run" != "true" && "$skip_if_unmatched" == "true" ]]; then
  write_output "exit-code" "0"
  append_summary "## AgentClash CI"
  append_summary ""
  append_summary "Skipped: ${skip_reason:-manifest trigger did not match this change set}"
  post_pr_comment "0"
  exit 0
fi

run_args=(ci run --manifest "$manifest" --json --artifact-dir "$artifact_dir")
if [[ -n "${INPUT_TIMEOUT:-}" ]]; then
  run_args+=(--timeout "${INPUT_TIMEOUT}")
fi
if [[ -n "${INPUT_POLL_INTERVAL:-}" ]]; then
  run_args+=(--poll-interval "${INPUT_POLL_INTERVAL}")
fi
if bool_true "${INPUT_FOLLOW:-false}"; then
  run_args+=(--follow)
fi
if [[ -n "${INPUT_DEFAULT_BRANCH:-}" ]]; then
  run_args+=(--ci-default-branch "${INPUT_DEFAULT_BRANCH}")
fi

set +e
agentclash "${run_args[@]}" >"$result_file"
status=$?
set -e

write_output "exit-code" "$status"
if [[ -s "$result_file" ]]; then
  cat "$result_file"
  write_output "run-id" "$(json_get "$result_file" candidate.run_id)"
  write_output "gate-verdict" "$(json_get "$result_file" gate_verdict)"
else
  write_output "run-id" ""
  write_output "gate-verdict" ""
fi
post_pr_comment "$status"
exit "$status"
