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
    elif isinstance(value, list):
        try:
            value = value[int(part)]
        except (ValueError, IndexError):
            value = None
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

agentclash_supports_ci_commands() {
  local should_run_help
  local run_help
  should_run_help="$("$@" ci should-run --help 2>&1)" &&
    run_help="$("$@" ci run --help 2>&1)" &&
    [[ "$should_run_help" == *"agentclash ci should-run"* ]] &&
    [[ "$should_run_help" == *"Flags:"* ]] &&
    [[ "$run_help" == *"agentclash ci run"* ]] &&
    [[ "$run_help" == *"Flags:"* ]]
}

agentclash_supports_required_commands() {
  case "$mode" in
    ci) agentclash_supports_ci_commands "$@" ;;
    *)
      echo "::error::Unsupported AgentClash CI action mode '${mode}'. Use 'ci'."
      return 1
      ;;
  esac
}

resolve_agentclash_cli() {
  agentclash_cmd=(agentclash)

  if command -v agentclash >/dev/null 2>&1; then
    if agentclash_supports_required_commands agentclash; then
      echo "Using installed agentclash CLI"
      return 0
    fi
    echo "::notice::Installed agentclash CLI does not expose required ${mode} commands; checking source fallback"
  fi

  if bool_true "${INPUT_SOURCE_FALLBACK:-true}"; then
    local source_dir
    local fallback_bin
    source_dir="$(cd "${ACTION_PATH}/../../.." && pwd)/cli"
    if [[ -d "$source_dir" ]] && command -v go >/dev/null 2>&1; then
      fallback_bin="${RUNNER_TEMP:-/tmp}/agentclash-source-cli"
      if go -C "$source_dir" build -o "$fallback_bin" .; then
        if agentclash_supports_required_commands "$fallback_bin"; then
          agentclash_cmd=("$fallback_bin")
          echo "Using AgentClash CLI source fallback from ${source_dir}"
          return 0
        fi
        echo "::notice::AgentClash CLI source fallback exists but does not expose required ${mode} commands"
      else
        echo "::notice::AgentClash CLI source fallback failed to build"
      fi
    elif [[ ! -d "$source_dir" ]]; then
      echo "::notice::AgentClash CLI source fallback is unavailable at ${source_dir}"
    else
      echo "::notice::Go is unavailable, so AgentClash CLI source fallback cannot run"
    fi
  fi

  echo "::error::AgentClash CI requires an agentclash CLI with the required ${mode} commands. Publish a newer npm CLI, set cli-version to a compatible version, or keep source-fallback enabled with Go available."
  return 1
}

post_pr_comment() {
  local status="$1"
  local comment_manifest="$manifest"
  if ! bool_true "${INPUT_PR_COMMENT:-true}"; then
    comment_posted=true
    return 0
  fi

  set +e
  python3 "${ACTION_PATH}/comment.py" \
    --manifest "$comment_manifest" \
    --mode "$mode" \
    --result-file "$result_file" \
    --should-run-file "$should_run_file" \
    --exit-code "$status" \
    --enabled "${INPUT_PR_COMMENT:-true}" \
    --repo "${GITHUB_REPOSITORY:-}" \
    --event-path "${GITHUB_EVENT_PATH:-}" \
    --api-url "${GITHUB_API_URL:-https://api.github.com}" \
    --app-url "${INPUT_APP_URL:-https://agentclash.dev}"
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
  printf '{"gate_verdict":"error","failure_reason":"action_failed_before_%s_run","errors":["AgentClash action failed before %s run completed. Inspect the GitHub Actions log for the failing command."]}\n' "$mode" "$mode" >"$result_file"
}

on_error() {
  local status="$?"
  trap - ERR
  write_output "exit-code" "$status"
  if [[ "${comment_posted:-false}" != "true" ]]; then
    write_early_error_result
    post_pr_comment "$status" || true
  fi
  exit "$status"
}

mode="${INPUT_MODE:-ci}"
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

resolve_agentclash_cli

if bool_true "${INPUT_REMOTE_VALIDATE:-true}"; then
  "${agentclash_cmd[@]}" ci validate "$manifest" --remote
else
  "${agentclash_cmd[@]}" ci validate "$manifest"
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

"${agentclash_cmd[@]}" "${should_args[@]}" >"$should_run_file"
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
  append_summary "## AgentClash ${mode}"
  append_summary ""
  append_summary "Skipped: ${skip_reason:-trigger did not match this change set}"
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
"${agentclash_cmd[@]}" "${run_args[@]}" >"$result_file"
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
