#!/usr/bin/env bash
set -Eeuo pipefail

# Large live smoke/E2E suite for an installed AgentClash CLI.
#
# Default mode is read-only. Pass --create-resources to create an isolated
# codex-e2e org/workspace and exercise write paths.
#
# Examples:
#   AGENTCLASH_API_URL=https://api.agentclash.dev ./testing/cli-e2e-suite.sh
#   AGENTCLASH_API_URL=https://api.agentclash.dev ./testing/cli-e2e-suite.sh --create-resources
#
# Optional:
#   AGENTCLASH_BIN=/path/to/agentclash
#   EXPECTED_AGENTCLASH_VERSION=0.1.2

BIN="${AGENTCLASH_BIN:-agentclash}"
API_URL="${AGENTCLASH_API_URL:-https://api.agentclash.dev}"
EXPECTED_VERSION="${EXPECTED_AGENTCLASH_VERSION:-}"
CREATE_RESOURCES=0
CLEANUP=1
KEEP_TEMP=0
RUN_EVALS=0
RUN_EXPERIMENTS=0

usage() {
  cat <<'EOF'
Usage: cli-e2e-suite.sh [options]

Options:
  --api-url URL          API URL to test. Default: AGENTCLASH_API_URL or https://api.agentclash.dev
  --create-resources    Create isolated codex-e2e resources and test write paths
  --full                Alias for --create-resources
  --run-evals           Also create an evaluation run. May enqueue worker/provider work
  --run-experiments     Also create playground experiments. May enqueue provider work
  --no-cleanup          Leave created resources in place for inspection
  --keep-temp           Keep the temporary config/work directory
  --help                Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --api-url)
      API_URL="${2:?--api-url requires a value}"
      shift 2
      ;;
    --create-resources|--full)
      CREATE_RESOURCES=1
      shift
      ;;
    --run-evals)
      RUN_EVALS=1
      CREATE_RESOURCES=1
      shift
      ;;
    --run-experiments)
      RUN_EXPERIMENTS=1
      CREATE_RESOURCES=1
      shift
      ;;
    --no-cleanup)
      CLEANUP=0
      shift
      ;;
    --keep-temp)
      KEEP_TEMP=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      printf 'unknown option: %s\n\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

tmpdir="$(mktemp -d)"
RUN_XDG="$tmpdir/xdg"
RUN_HOME="$tmpdir/home"
WORKDIR="$tmpdir/work"
mkdir -p "$RUN_XDG/agentclash" "$RUN_HOME" "$WORKDIR"

passed=0
failed=0
skipped=0
case_no=0
LAST_OUT=""
LAST_ERR=""
LAST_CODE=0

ORG_ID=""
WORKSPACE_ID=""
CREATED_ORG=0
CREATED_WORKSPACE=0
RUNTIME_PROFILE_ID=""
PROVIDER_ACCOUNT_ID=""
MODEL_ALIAS_ID=""
TOOL_ID=""
KNOWLEDGE_SOURCE_ID=""
ROUTING_POLICY_ID=""
SPEND_POLICY_ID=""
PLAYGROUND_ID=""
PLAYGROUND_TEST_CASE_ID=""
BUILD_ID=""
BUILD_VERSION_ID=""
DEPLOYMENT_ID=""
CHALLENGE_PACK_VERSION_ID=""
ARTIFACT_ID=""
RUN_ID=""
PLAYGROUND_EXPERIMENT_A_ID=""
PLAYGROUND_EXPERIMENT_B_ID=""
SECRET_KEY=""

slug() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9_' '_'
}

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

skip() {
  skipped=$((skipped + 1))
  printf 'skip - %s\n' "$*"
}

show_failure() {
  if [[ -s "$LAST_ERR" ]]; then
    printf '%s\n' '--- stderr ---' >&2
    sed -n '1,120p' "$LAST_ERR" >&2
  fi
  if [[ -s "$LAST_OUT" ]]; then
    printf '%s\n' '--- stdout ---' >&2
    sed -n '1,120p' "$LAST_OUT" >&2
  fi
}

capture() {
  local name="$1"
  shift
  case_no=$((case_no + 1))
  local base
  base="$(printf '%03d_%s' "$case_no" "$(slug "$name")")"
  LAST_OUT="$tmpdir/$base.out"
  LAST_ERR="$tmpdir/$base.err"

  set +e
  "$@" >"$LAST_OUT" 2>"$LAST_ERR"
  LAST_CODE=$?
  set -e
}

expect_success() {
  local name="$1"
  shift
  capture "$name" "$@"
  if [[ "$LAST_CODE" -eq 0 ]]; then
    ok "$name"
    return 0
  fi
  fail "$name exited $LAST_CODE"
  show_failure
  return 1
}

expect_failure_with_output() {
  local name="$1"
  shift
  capture "$name" "$@"
  if [[ "$LAST_CODE" -eq 0 ]]; then
    fail "$name unexpectedly succeeded"
    show_failure
    return 1
  fi
  if [[ -s "$LAST_ERR" || -s "$LAST_OUT" ]]; then
    ok "$name"
    return 0
  fi
  fail "$name failed silently"
  return 1
}

expect_contains() {
  local name="$1"
  local needle="$2"
  shift 2
  capture "$name" "$@"
  if [[ "$LAST_CODE" -ne 0 ]]; then
    fail "$name exited $LAST_CODE"
    show_failure
    return 1
  fi
  if grep -Fq "$needle" "$LAST_OUT" || grep -Fq "$needle" "$LAST_ERR"; then
    ok "$name"
    return 0
  fi
  fail "$name did not contain '$needle'"
  show_failure
  return 1
}

need_jq() {
  if command -v jq >/dev/null 2>&1; then
    return 0
  fi
  fail "jq is required for resource mode JSON extraction"
  return 1
}

expect_json() {
  local name="$1"
  local filter="$2"
  shift 2
  if ! expect_success "$name" "$@"; then
    return 1
  fi
  if command -v jq >/dev/null 2>&1; then
    if jq -e "$filter" "$LAST_OUT" >/dev/null; then
      ok "$name JSON assertion"
      return 0
    fi
    fail "$name JSON assertion failed: $filter"
    show_failure
    return 1
  fi
  skip "$name JSON assertion skipped because jq is not installed"
  return 0
}

json_get() {
  local filter="$1"
  jq -r "$filter" "$LAST_OUT"
}

copy_current_config() {
  local current_xdg
  current_xdg="${XDG_CONFIG_HOME:-$HOME/.config}"
  local current_config="$current_xdg/agentclash"

  if [[ -f "$current_config/credentials.json" ]]; then
    cp "$current_config/credentials.json" "$RUN_XDG/agentclash/credentials.json"
    chmod 0600 "$RUN_XDG/agentclash/credentials.json" || true
  fi
  if [[ -f "$current_config/config.yaml" ]]; then
    cp "$current_config/config.yaml" "$RUN_XDG/agentclash/config.yaml"
    chmod 0600 "$RUN_XDG/agentclash/config.yaml" || true
  fi
}

CLI_ENV=(env XDG_CONFIG_HOME="$RUN_XDG" HOME="$RUN_HOME")
BASE=("${CLI_ENV[@]}" "$BIN" --no-color --api-url "$API_URL")
JSON=("${CLI_ENV[@]}" "$BIN" --no-color --json --api-url "$API_URL")

cleanup() {
  local cleanup_failed=0
  if [[ "$CREATE_RESOURCES" -ne 1 || "$CLEANUP" -ne 1 ]]; then
    if [[ "$KEEP_TEMP" -eq 1 ]]; then
      printf '\ntemp kept at %s\n' "$tmpdir"
    else
      rm -rf "$tmpdir"
    fi
    return
  fi

  printf '\n==> Cleanup\n'
  set +e
  if [[ -n "$PLAYGROUND_TEST_CASE_ID" ]]; then
    "${BASE[@]}" playground test-case delete "$PLAYGROUND_TEST_CASE_ID" >/dev/null 2>&1 || cleanup_failed=1
  fi
  if [[ -n "$PLAYGROUND_ID" ]]; then
    "${BASE[@]}" playground delete "$PLAYGROUND_ID" >/dev/null 2>&1 || cleanup_failed=1
  fi
  if [[ -n "$SECRET_KEY" && -n "$WORKSPACE_ID" ]]; then
    "${BASE[@]}" --workspace "$WORKSPACE_ID" secret delete "$SECRET_KEY" >/dev/null 2>&1 || true
  fi
  if [[ -n "$MODEL_ALIAS_ID" && -z "$DEPLOYMENT_ID" ]]; then
    "${BASE[@]}" infra model-alias delete "$MODEL_ALIAS_ID" >/dev/null 2>&1 || cleanup_failed=1
  elif [[ -n "$MODEL_ALIAS_ID" ]]; then
    printf 'note - model alias retained because deployment %s references it\n' "$DEPLOYMENT_ID"
  fi
  if [[ -n "$PROVIDER_ACCOUNT_ID" && -z "$DEPLOYMENT_ID" ]]; then
    "${BASE[@]}" infra provider-account delete "$PROVIDER_ACCOUNT_ID" >/dev/null 2>&1 || cleanup_failed=1
  elif [[ -n "$PROVIDER_ACCOUNT_ID" ]]; then
    printf 'note - provider account retained because deployment %s references it\n' "$DEPLOYMENT_ID"
  fi
  if [[ -n "$RUNTIME_PROFILE_ID" ]]; then
    "${BASE[@]}" infra runtime-profile archive "$RUNTIME_PROFILE_ID" >/dev/null 2>&1 || cleanup_failed=1
  fi
  if [[ -n "$WORKSPACE_ID" ]]; then
    "${JSON[@]}" workspace update "$WORKSPACE_ID" --status archived >/dev/null 2>&1 || cleanup_failed=1
  fi
  if [[ -n "$ORG_ID" && "$CREATED_ORG" -eq 1 ]]; then
    "${JSON[@]}" org update "$ORG_ID" --status archived >/dev/null 2>&1 || cleanup_failed=1
  elif [[ -n "$ORG_ID" ]]; then
    printf 'note - org %s retained because it existed before this suite\n' "$ORG_ID"
  fi
  set -e

  if [[ "$cleanup_failed" -eq 0 ]]; then
    printf 'ok - cleanup completed\n'
  else
    printf 'not ok - cleanup had one or more failures; inspect resources with prefix codex-e2e\n' >&2
  fi

  if [[ "$KEEP_TEMP" -eq 1 ]]; then
    printf 'temp kept at %s\n' "$tmpdir"
  else
    rm -rf "$tmpdir"
  fi
}
trap cleanup EXIT

write_challenge_pack() {
  local path="$1"
  local pack_slug="$2"
  local spec_name="$3"
  cat >"$path" <<EOF
pack:
  slug: $pack_slug
  name: Codex E2E Pack
  family: codex-e2e
version:
  number: 1
  execution_mode: prompt_eval
  evaluation_spec:
    name: $spec_name
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: echo
    title: Echo
    category: smoke
    difficulty: easy
    instructions: "Return exactly {{text}}"
input_sets:
  - key: default
    name: Default
    cases:
      - challenge_key: echo
        case_key: echo-hello
        inputs:
          - key: text
            kind: text
            value: hello
        expectations:
          - key: answer
            kind: text
            source: input:text
EOF
}

resource_suite() {
  need_jq || return 1

  local suffix prefix org_slug workspace_slug alias_key model_id provider_key provider_model_id
  suffix="$(date -u +%Y%m%d%H%M%S)-$RANDOM"
  prefix="codex-e2e-$suffix"
  org_slug="$prefix-org"
  workspace_slug="$prefix-ws"
  alias_key="codex_e2e_${suffix//[^A-Za-z0-9]/_}"
  SECRET_KEY="CODEX_E2E_${suffix//[^A-Za-z0-9]/_}"

  say "Resource Mode"
  printf 'prefix: %s\n' "$prefix"

  capture "create org" "${JSON[@]}" org create --name "Codex E2E Org $suffix" --slug "$org_slug"
  if [[ "$LAST_CODE" -eq 0 ]] && jq -e '.id and .name and .slug' "$LAST_OUT" >/dev/null; then
    ok "create org"
    ok "create org JSON assertion"
    ORG_ID="$(json_get '.id')"
    CREATED_ORG=1
  elif grep -Fq "organization_limit_reached" "$LAST_ERR" "$LAST_OUT"; then
    skip "org create hit organization_limit_reached; falling back to an existing org"
    expect_json "org list for fallback" '.items[0].id' "${JSON[@]}" org list || return 1
    ORG_ID="$(json_get '.items[0].id')"
  else
    fail "create org exited $LAST_CODE"
    show_failure
    return 1
  fi

  expect_json "get selected org" '.id != null' \
    "${JSON[@]}" org get "$ORG_ID" || return 1
  if [[ "$CREATED_ORG" -eq 1 ]]; then
    expect_json "update created org" '.name | contains("Updated")' \
      "${JSON[@]}" org update "$ORG_ID" --name "Codex E2E Org Updated $suffix" || return 1
  else
    skip "org update skipped because fallback org was not created by this suite"
  fi
  expect_success "list org members" "${BASE[@]}" org members list "$ORG_ID" || true

  expect_json "create workspace" '.id and .organization_id' \
    "${JSON[@]}" workspace create --org "$ORG_ID" --name "Codex E2E Workspace $suffix" --slug "$workspace_slug" || return 1
  WORKSPACE_ID="$(json_get '.id')"
  CREATED_WORKSPACE=1

  expect_json "get created workspace" '.id and .organization_id' \
    "${JSON[@]}" workspace get "$WORKSPACE_ID" || return 1
  expect_json "update created workspace" '.name | contains("Updated")' \
    "${JSON[@]}" workspace update "$WORKSPACE_ID" --name "Codex E2E Workspace Updated $suffix" || return 1
  expect_json "workspace use writes isolated config" '.workspace_id and .organization_id' \
    "${JSON[@]}" workspace use "$WORKSPACE_ID" || return 1
  expect_success "list workspace members" "${BASE[@]}" --workspace "$WORKSPACE_ID" workspace members list || true

  local project_dir="$WORKDIR/project"
  mkdir -p "$project_dir"
  pushd "$project_dir" >/dev/null
  expect_json "init project config in temp dir" '(.workspace_id or .WorkspaceID) and (.org_id or .OrgID)' \
    "${JSON[@]}" init --workspace-id "$WORKSPACE_ID" --org-id "$ORG_ID" || true
  popd >/dev/null

  WS_BASE=("${BASE[@]}" --workspace "$WORKSPACE_ID")
  WS_JSON=("${JSON[@]}" --workspace "$WORKSPACE_ID")

  say "Model Catalog"
  if expect_json "model-catalog list" '.items | type == "array"' "${JSON[@]}" infra model-catalog list; then
    model_id="$(jq -r '.items[0].id // empty' "$LAST_OUT")"
    provider_key="$(jq -r '.items[0].provider_key // "openai"' "$LAST_OUT")"
    provider_model_id="$(jq -r '.items[0].provider_model_id // "gpt-4.1-mini"' "$LAST_OUT")"
    if [[ -n "$model_id" ]]; then
      expect_json "model-catalog get" '.id' "${JSON[@]}" infra model-catalog get "$model_id" || true
    else
      skip "model-catalog get skipped because catalog is empty"
    fi
  else
    model_id=""
    provider_key="openai"
    provider_model_id="gpt-4.1-mini"
  fi

  say "Infrastructure Creates"
  jq -n --arg name "Codex E2E Runtime $suffix" '{
    name: $name,
    execution_target: "native",
    max_iterations: 1,
    max_tool_calls: 0,
    step_timeout_seconds: 30,
    run_timeout_seconds: 120,
    profile_config: {suite: "codex-e2e"}
  }' >"$WORKDIR/runtime-profile.json"
  expect_json "create runtime profile" '.id' \
    "${WS_JSON[@]}" infra runtime-profile create --from-file "$WORKDIR/runtime-profile.json" || return 1
  RUNTIME_PROFILE_ID="$(json_get '.id')"
  expect_success "get runtime profile" "${WS_BASE[@]}" infra runtime-profile get "$RUNTIME_PROFILE_ID" || true

  jq -n --arg provider "$provider_key" --arg name "Codex E2E Provider $suffix" '{
    provider_key: $provider,
    name: $name,
    api_key: "codex-e2e-not-a-real-provider-key",
    limits_config: {suite: "codex-e2e"}
  }' >"$WORKDIR/provider-account.json"
  expect_json "create provider account" '.id' \
    "${WS_JSON[@]}" infra provider-account create --from-file "$WORKDIR/provider-account.json" || return 1
  PROVIDER_ACCOUNT_ID="$(json_get '.id')"
  expect_success "get provider account" "${WS_BASE[@]}" infra provider-account get "$PROVIDER_ACCOUNT_ID" || true

  if [[ -n "$model_id" ]]; then
    jq -n --arg alias "$alias_key" --arg display "Codex E2E Alias $suffix" --arg model "$model_id" --arg provider_account "$PROVIDER_ACCOUNT_ID" '{
      alias_key: $alias,
      display_name: $display,
      model_catalog_entry_id: $model,
      provider_account_id: $provider_account
    }' >"$WORKDIR/model-alias.json"
    expect_json "create model alias" '.id' \
      "${WS_JSON[@]}" infra model-alias create --from-file "$WORKDIR/model-alias.json" || return 1
    MODEL_ALIAS_ID="$(json_get '.id')"
    expect_success "get model alias" "${WS_BASE[@]}" infra model-alias get "$MODEL_ALIAS_ID" || true
  else
    skip "model alias create skipped because model catalog is empty"
  fi

  jq -n --arg name "Codex E2E Tool $suffix" '{
    name: $name,
    tool_kind: "http",
    capability_key: "codex.e2e.echo",
    definition: {suite: "codex-e2e", method: "GET"}
  }' >"$WORKDIR/tool.json"
  expect_json "create tool" '.id' "${WS_JSON[@]}" infra tool create --from-file "$WORKDIR/tool.json" || true
  TOOL_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
  if [[ -n "$TOOL_ID" ]]; then
    expect_success "get tool" "${WS_BASE[@]}" infra tool get "$TOOL_ID" || true
  fi

  jq -n --arg name "Codex E2E Knowledge $suffix" '{
    name: $name,
    source_kind: "static",
    connection_config: {suite: "codex-e2e"}
  }' >"$WORKDIR/knowledge-source.json"
  expect_json "create knowledge source" '.id' "${WS_JSON[@]}" infra knowledge-source create --from-file "$WORKDIR/knowledge-source.json" || true
  KNOWLEDGE_SOURCE_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
  if [[ -n "$KNOWLEDGE_SOURCE_ID" ]]; then
    expect_success "get knowledge source" "${WS_BASE[@]}" infra knowledge-source get "$KNOWLEDGE_SOURCE_ID" || true
  fi

  jq -n --arg name "Codex E2E Routing $suffix" '{
    name: $name,
    policy_kind: "single_model",
    config: {suite: "codex-e2e", strategy: "single_model"}
  }' >"$WORKDIR/routing-policy.json"
  expect_json "create routing policy" '.id' "${WS_JSON[@]}" infra routing-policy create --from-file "$WORKDIR/routing-policy.json" || true
  ROUTING_POLICY_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"

  jq -n --arg name "Codex E2E Spend $suffix" '{
    name: $name,
    currency_code: "USD",
    window_kind: "month",
    soft_limit: 1,
    hard_limit: 2,
    config: {suite: "codex-e2e"}
  }' >"$WORKDIR/spend-policy.json"
  expect_json "create spend policy" '.id' "${WS_JSON[@]}" infra spend-policy create --from-file "$WORKDIR/spend-policy.json" || true
  SPEND_POLICY_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"

  say "Workspace Lists"
  for resource in runtime-profile provider-account model-alias tool knowledge-source routing-policy spend-policy; do
    expect_success "infra $resource list" "${WS_BASE[@]}" infra "$resource" list || true
    expect_json "infra $resource list json" '.items | type == "array"' "${WS_JSON[@]}" infra "$resource" list || true
  done
  expect_success "build list" "${WS_BASE[@]}" build list || true
  expect_success "deployment list" "${WS_BASE[@]}" deployment list || true
  expect_success "run list" "${WS_BASE[@]}" run list || true
  expect_success "challenge-pack list" "${WS_BASE[@]}" challenge-pack list || true
  expect_success "playground list" "${WS_BASE[@]}" playground list || true
  expect_success "secret list" "${WS_BASE[@]}" secret list || true

  say "Secrets"
  expect_success "set workspace secret" "${WS_BASE[@]}" secret set "$SECRET_KEY" --value "codex-e2e-secret-$suffix" || true
  expect_contains "secret list includes key" "$SECRET_KEY" "${WS_BASE[@]}" secret list || true
  expect_success "delete workspace secret" "${WS_BASE[@]}" secret delete "$SECRET_KEY" || true
  SECRET_KEY=""

  say "Artifacts"
  printf 'codex e2e artifact %s\n' "$suffix" >"$WORKDIR/artifact.txt"
  expect_json "upload artifact" '.id' \
    "${WS_JSON[@]}" artifact upload "$WORKDIR/artifact.txt" --type codex-e2e --metadata "{\"suite\":\"$prefix\"}" || true
  ARTIFACT_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
  if [[ -n "$ARTIFACT_ID" ]]; then
    expect_success "download artifact" "${WS_BASE[@]}" artifact download "$ARTIFACT_ID" -O "$WORKDIR/artifact.downloaded.txt" || true
    if cmp -s "$WORKDIR/artifact.txt" "$WORKDIR/artifact.downloaded.txt"; then
      ok "downloaded artifact content matches"
    else
      fail "downloaded artifact content mismatch"
    fi
  else
    skip "artifact download skipped because upload did not return an id"
  fi

  say "Challenge Packs"
  write_challenge_pack "$WORKDIR/challenge-pack.yaml" "$prefix-pack" "codex-e2e-$suffix"
  expect_success "validate challenge pack" "${WS_BASE[@]}" challenge-pack validate "$WORKDIR/challenge-pack.yaml" || true
  expect_json "publish challenge pack" '.challenge_pack_version_id' \
    "${WS_JSON[@]}" challenge-pack publish "$WORKDIR/challenge-pack.yaml" || true
  CHALLENGE_PACK_VERSION_ID="$(jq -r '.challenge_pack_version_id // empty' "$LAST_OUT" 2>/dev/null || true)"
  expect_success "challenge-pack list after publish" "${WS_BASE[@]}" challenge-pack list || true

  say "Builds and Deployments"
  expect_json "create build" '.id' \
    "${WS_JSON[@]}" build create --name "Codex E2E Build $suffix" --description "Created by CLI E2E suite" || true
  BUILD_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
  if [[ -n "$BUILD_ID" ]]; then
    jq -n '{
      agent_kind: "llm_agent",
      policy_spec: {instructions: "Return exactly what the prompt asks for."},
      interface_spec: {input: "text", output: "text"},
      model_spec: {suite: "codex-e2e"},
      publication_spec: {visibility: "private"}
    }' >"$WORKDIR/build-version.json"
    expect_json "create build version" '.id' \
      "${WS_JSON[@]}" build version create "$BUILD_ID" --spec-file "$WORKDIR/build-version.json" || true
    BUILD_VERSION_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
    if [[ -n "$BUILD_VERSION_ID" ]]; then
      expect_json "get build version" '.id' "${WS_JSON[@]}" build version get "$BUILD_VERSION_ID" || true
      expect_json "validate build version" '.valid == true' "${WS_JSON[@]}" build version validate "$BUILD_VERSION_ID" || true
      expect_json "ready build version" '.version_status == "ready"' "${WS_JSON[@]}" build version ready "$BUILD_VERSION_ID" || true
      expect_success "get build detail" "${WS_BASE[@]}" build get "$BUILD_ID" || true
    fi
  fi

  if [[ -n "$BUILD_ID" && -n "$BUILD_VERSION_ID" && -n "$RUNTIME_PROFILE_ID" && -n "$PROVIDER_ACCOUNT_ID" && -n "$MODEL_ALIAS_ID" ]]; then
    jq -n \
      --arg name "Codex E2E Deployment $suffix" \
      --arg build "$BUILD_ID" \
      --arg version "$BUILD_VERSION_ID" \
      --arg runtime "$RUNTIME_PROFILE_ID" \
      --arg provider "$PROVIDER_ACCOUNT_ID" \
      --arg alias "$MODEL_ALIAS_ID" \
      '{
        name: $name,
        agent_build_id: $build,
        build_version_id: $version,
        runtime_profile_id: $runtime,
        provider_account_id: $provider,
        model_alias_id: $alias,
        deployment_config: {suite: "codex-e2e"}
      }' >"$WORKDIR/deployment.json"
    expect_json "create deployment" '.id' "${WS_JSON[@]}" deployment create --from-file "$WORKDIR/deployment.json" || true
    DEPLOYMENT_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
  else
    skip "deployment create skipped because build/runtime/provider/model prerequisites are missing"
  fi
  expect_success "deployment list after create" "${WS_BASE[@]}" deployment list || true

  say "Playgrounds"
  jq -n --arg name "Codex E2E Playground $suffix" '{
    name: $name,
    prompt_template: "Return exactly: {{text}}",
    system_prompt: "You are a deterministic echo test.",
    evaluation_spec: {
      name: "codex-e2e-playground",
      version_number: 1,
      judge_mode: "deterministic",
      validators: [
        {key: "exact", type: "exact_match", target: "final_output", expected_from: "challenge_input"}
      ],
      scorecard: {dimensions: ["correctness"]}
    }
  }' >"$WORKDIR/playground.json"
  expect_json "create playground" '.id' "${WS_JSON[@]}" playground create --from-file "$WORKDIR/playground.json" || true
  PLAYGROUND_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
  if [[ -n "$PLAYGROUND_ID" ]]; then
    expect_success "get playground" "${WS_BASE[@]}" playground get "$PLAYGROUND_ID" || true
    jq -n --arg name "Codex E2E Playground Updated $suffix" '{
      name: $name,
      prompt_template: "Return exactly: {{text}}",
      system_prompt: "You are a deterministic echo test.",
      evaluation_spec: {
        name: "codex-e2e-playground",
        version_number: 1,
        judge_mode: "deterministic",
        validators: [
          {key: "exact", type: "exact_match", target: "final_output", expected_from: "challenge_input"}
        ],
        scorecard: {dimensions: ["correctness"]}
      }
    }' >"$WORKDIR/playground-update.json"
    expect_json "update playground" '.id' "${WS_JSON[@]}" playground update "$PLAYGROUND_ID" --from-file "$WORKDIR/playground-update.json" || true

    jq -n '{
      case_key: "echo-hello",
      variables: {text: "hello"},
      expectations: {answer: "hello"}
    }' >"$WORKDIR/playground-test-case.json"
    expect_json "create playground test case" '.id' \
      "${WS_JSON[@]}" playground test-case create "$PLAYGROUND_ID" --from-file "$WORKDIR/playground-test-case.json" || true
    PLAYGROUND_TEST_CASE_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
    expect_success "list playground test cases" "${WS_BASE[@]}" playground test-case list "$PLAYGROUND_ID" || true

    if [[ "$RUN_EXPERIMENTS" -eq 1 && -n "$PROVIDER_ACCOUNT_ID" && -n "$MODEL_ALIAS_ID" ]]; then
      jq -n --arg provider "$PROVIDER_ACCOUNT_ID" --arg alias "$MODEL_ALIAS_ID" '{
        name: "codex-e2e-experiment-a",
        provider_account_id: $provider,
        model_alias_id: $alias,
        request_config: {temperature: 0}
      }' >"$WORKDIR/playground-experiment-a.json"
      expect_json "create playground experiment A" '.id' \
        "${WS_JSON[@]}" playground experiment create "$PLAYGROUND_ID" --from-file "$WORKDIR/playground-experiment-a.json" || true
      PLAYGROUND_EXPERIMENT_A_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
      if [[ -n "$PLAYGROUND_EXPERIMENT_A_ID" ]]; then
        expect_success "get playground experiment A" "${WS_BASE[@]}" playground experiment get "$PLAYGROUND_EXPERIMENT_A_ID" || true
        expect_success "playground experiment A results" "${WS_BASE[@]}" playground experiment results "$PLAYGROUND_EXPERIMENT_A_ID" || true
      fi
    else
      skip "playground experiment create skipped; pass --run-experiments to enqueue provider work"
    fi

    if [[ -n "$PLAYGROUND_TEST_CASE_ID" && "$RUN_EXPERIMENTS" -eq 0 ]]; then
      expect_success "delete playground test case" "${WS_BASE[@]}" playground test-case delete "$PLAYGROUND_TEST_CASE_ID" || true
      PLAYGROUND_TEST_CASE_ID=""
    fi
    if [[ "$RUN_EXPERIMENTS" -eq 0 ]]; then
      expect_success "delete playground" "${WS_BASE[@]}" playground delete "$PLAYGROUND_ID" || true
      PLAYGROUND_ID=""
    fi
  fi

  say "Runs"
  if [[ "$RUN_EVALS" -eq 1 && -n "$CHALLENGE_PACK_VERSION_ID" && -n "$DEPLOYMENT_ID" ]]; then
    expect_json "create evaluation run" '.id' \
      "${WS_JSON[@]}" run create --challenge-pack-version "$CHALLENGE_PACK_VERSION_ID" --deployments "$DEPLOYMENT_ID" --name "Codex E2E Run $suffix" || true
    RUN_ID="$(jq -r '.id // empty' "$LAST_OUT" 2>/dev/null || true)"
    if [[ -n "$RUN_ID" ]]; then
      expect_success "get run" "${WS_BASE[@]}" run get "$RUN_ID" || true
      expect_success "run ranking" "${WS_BASE[@]}" run ranking "$RUN_ID" || true
      expect_success "run agents" "${WS_BASE[@]}" run agents "$RUN_ID" || true
    fi
  else
    skip "run create skipped; pass --run-evals to enqueue evaluation work"
  fi
}

copy_current_config

say "Binary"
if command -v "$BIN" >/dev/null 2>&1; then
  ok "found $BIN at $(command -v "$BIN")"
else
  fail "could not find $BIN on PATH"
  exit 1
fi

if expect_success "agentclash version" "${BASE[@]}" version; then
  sed -n '1,8p' "$LAST_OUT"
fi
if [[ -n "$EXPECTED_VERSION" ]]; then
  if grep -Fq "agentclash $EXPECTED_VERSION" "$LAST_OUT"; then
    ok "version is $EXPECTED_VERSION"
  else
    fail "version is not $EXPECTED_VERSION"
  fi
fi

say "Workflow Discovery"
expect_contains "root help mentions link" "agentclash link" "${BASE[@]}" --help || true
expect_contains "eval help lists start" "start" "${BASE[@]}" eval --help || true
expect_contains "baseline help lists set" "set" "${BASE[@]}" baseline --help || true
expect_contains "doctor help describes readiness checks" "Check auth, workspace, and eval readiness" "${BASE[@]}" doctor --help || true
expect_contains "challenge-pack init help works" "Scaffold a minimal challenge pack YAML bundle" "${BASE[@]}" challenge-pack init --help || true

say "Config and Auth"
printf 'api: %s\n' "$API_URL"
printf 'temp config: %s/agentclash\n' "$RUN_XDG"
expect_success "config list" "${BASE[@]}" config list || true
expect_json "auth status json" '.user_id or .email' "${JSON[@]}" auth status || true
expect_success "auth status table" "${BASE[@]}" auth status || true
expect_contains "auth login --device no-op" "Already logged in" "${BASE[@]}" auth login --device || true
expect_json "auth tokens list json" '.items | type == "array"' "${JSON[@]}" auth tokens list || true
expect_success "auth tokens list table" "${BASE[@]}" auth tokens list || true

say "Read-Only API Surface"
expect_json "org list json" '.items | type == "array"' "${JSON[@]}" org list || true
expect_success "org list table" "${BASE[@]}" org list || true
expect_json "model catalog list json" '.items | type == "array"' "${JSON[@]}" infra model-catalog list || true
if command -v jq >/dev/null 2>&1 && [[ -s "$LAST_OUT" ]]; then
  first_model="$(jq -r '.items[0].id // empty' "$LAST_OUT" 2>/dev/null || true)"
  if [[ -n "$first_model" ]]; then
    expect_success "model catalog get first item" "${BASE[@]}" infra model-catalog get "$first_model" || true
  else
    skip "model catalog get skipped because catalog is empty"
  fi
fi

configured_org="$("${BASE[@]}" config get default_org 2>/dev/null || true)"
configured_workspace="$("${BASE[@]}" config get default_workspace 2>/dev/null || true)"

if [[ -n "${AGENTCLASH_ORG:-$configured_org}" ]]; then
  expect_success "workspace list table" "${BASE[@]}" workspace list || true
  expect_json "workspace list json" '.items | type == "array"' "${JSON[@]}" workspace list || true
else
  skip "workspace list needs AGENTCLASH_ORG or default_org"
fi

if [[ -n "${AGENTCLASH_WORKSPACE:-$configured_workspace}" ]]; then
  for cmd in \
    "challenge-pack list" \
    "build list" \
    "deployment list" \
    "run list" \
    "playground list" \
    "secret list" \
    "infra runtime-profile list" \
    "infra provider-account list" \
    "infra model-alias list" \
    "infra tool list" \
    "infra knowledge-source list" \
    "infra routing-policy list" \
    "infra spend-policy list"; do
    # shellcheck disable=SC2206
    parts=($cmd)
    expect_success "$cmd" "${BASE[@]}" "${parts[@]}" || true
  done
else
  skip "workspace-scoped read-only lists need AGENTCLASH_WORKSPACE or default_workspace"
fi

say "Negative Checks"
expect_failure_with_output \
  "invalid AGENTCLASH_TOKEN fails with visible error" \
  env XDG_CONFIG_HOME="$RUN_XDG" HOME="$RUN_HOME" AGENTCLASH_TOKEN="agentclash_invalid_smoke_token" \
  "$BIN" --no-color --api-url "$API_URL" auth status || true
expect_failure_with_output \
  "missing workspace fails visibly" \
  env XDG_CONFIG_HOME="$tmpdir/empty-xdg" HOME="$RUN_HOME" \
  "$BIN" --no-color --api-url "$API_URL" challenge-pack list || true

if [[ "$CREATE_RESOURCES" -eq 1 ]]; then
  resource_suite || true
else
  skip "resource creation suite skipped; pass --create-resources"
fi

say "Summary"
printf 'passed: %d\nfailed: %d\nskipped: %d\n' "$passed" "$failed" "$skipped"

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi
