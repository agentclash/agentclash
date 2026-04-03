#!/usr/bin/env bash
set -euo pipefail

# Seeds a SWE-bench-inspired local coding pack fixture for real multi-agent runs.
# This is intentionally destructive to the local dev database.
#
# The seeded pack is a narrow deterministic patch-correctness benchmark:
# - one coding challenge
# - one challenge input item
# - staged workspace files inside the sandbox
# - structured JSON final answer
# - five OpenAI-backed native deployments

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"

if [[ -f "${BACKEND_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${BACKEND_DIR}/.env"
  set +a
elif [[ -f "${BACKEND_DIR}/.env.example" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${BACKEND_DIR}/.env.example"
  set +a
fi

export DATABASE_URL="${DATABASE_URL:-postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable}"

ORG_ID="11111111-1111-1111-1111-111111111111"
WORKSPACE_ID="22222222-2222-2222-2222-222222222222"
USER_ID="33333333-3333-3333-3333-333333333333"
CHALLENGE_PACK_ID="53535353-4444-4444-4444-444444444444"
CHALLENGE_PACK_VERSION_ID="53535353-5555-5555-5555-555555555555"
CHALLENGE_IDENTITY_ID="53535353-6666-6666-6666-666666666666"
CHALLENGE_VERSION_ID="53535353-7777-7777-7777-777777777777"
CHALLENGE_INPUT_SET_ID="53535353-abab-abab-abab-abababababab"
CHALLENGE_INPUT_ITEM_ID="53535353-cdcd-cdcd-cdcd-cdcdcdcdcdcd"
RUNTIME_PROFILE_ID="53535353-8888-8888-8888-888888888888"
PROVIDER_ACCOUNT_ID="53535353-9999-9999-9999-999999999999"
AGENT_BUILD_ID="53535353-cccc-cccc-cccc-cccccccccccc"
AGENT_BUILD_VERSION_ID="53535353-dddd-dddd-dddd-dddddddddddd"

MODEL_CATALOG_ENTRY_GPT54_ID="53535353-a001-a001-a001-a001a001a001"
MODEL_CATALOG_ENTRY_GPT54N_ID="53535353-a002-a002-a002-a002a002a002"
MODEL_CATALOG_ENTRY_GPT41_ID="53535353-a003-a003-a003-a003a003a003"
MODEL_CATALOG_ENTRY_GPT41M_ID="53535353-a004-a004-a004-a004a004a004"
MODEL_CATALOG_ENTRY_GPT4OM_ID="53535353-a005-a005-a005-a005a005a005"

MODEL_ALIAS_GPT54_ID="53535353-b001-b001-b001-b001b001b001"
MODEL_ALIAS_GPT54N_ID="53535353-b002-b002-b002-b002b002b002"
MODEL_ALIAS_GPT41_ID="53535353-b003-b003-b003-b003b003b003"
MODEL_ALIAS_GPT41M_ID="53535353-b004-b004-b004-b004b004b004"
MODEL_ALIAS_GPT4OM_ID="53535353-b005-b005-b005-b005b005b005"

AGENT_DEPLOYMENT_GPT54_ID="53535353-e001-e001-e001-e001e001e001"
AGENT_DEPLOYMENT_GPT54N_ID="53535353-e002-e002-e002-e002e002e002"
AGENT_DEPLOYMENT_GPT41_ID="53535353-e003-e003-e003-e003e003e003"
AGENT_DEPLOYMENT_GPT41M_ID="53535353-e004-e004-e004-e004e004e004"
AGENT_DEPLOYMENT_GPT4OM_ID="53535353-e005-e005-e005-e005e005e005"

AGENT_DEPLOYMENT_SNAPSHOT_GPT54_ID="53535353-f001-f001-f001-f001f001f001"
AGENT_DEPLOYMENT_SNAPSHOT_GPT54N_ID="53535353-f002-f002-f002-f002f002f002"
AGENT_DEPLOYMENT_SNAPSHOT_GPT41_ID="53535353-f003-f003-f003-f003f003f003"
AGENT_DEPLOYMENT_SNAPSHOT_GPT41M_ID="53535353-f004-f004-f004-f004f004f004"
AGENT_DEPLOYMENT_SNAPSHOT_GPT4OM_ID="53535353-f005-f005-f005-f005f005f005"

APP_PY_CONTENT=$'def add(a, b):\n    """Return the sum of two integers."""\n    return a - b\n'
TEST_PY_CONTENT=$'from app import add\n\n\ndef test_add_two_positive_numbers():\n    assert add(2, 3) == 5\n\n\ndef test_add_negative_and_positive_number():\n    assert add(-4, 10) == 6\n'
README_CONTENT=$'# Patch Correctness Fixture\n\nFix the arithmetic regression in app.py so the staged tests pass.\n'

APP_PY_JSON="$(jq -Rn --arg value "${APP_PY_CONTENT}" '$value')"
TEST_PY_JSON="$(jq -Rn --arg value "${TEST_PY_CONTENT}" '$value')"
README_JSON="$(jq -Rn --arg value "${README_CONTENT}" '$value')"

INPUT_PAYLOAD="$(jq -nc \
  --arg issue_title "Fix add() arithmetic regression in the staged workspace" \
  --arg issue_summary "A recent refactor inverted the operator in project/app.py. The staged pytest file shows the intended behavior." \
  --arg failing_test "pytest /workspace/project/test_app.py" \
  --arg expected_path "/workspace/project/app.py" \
  --arg app_py "${APP_PY_CONTENT}" \
  --arg test_py "${TEST_PY_CONTENT}" \
  --arg readme "${README_CONTENT}" \
  '{
    issue_title: $issue_title,
    issue_summary: $issue_summary,
    failing_test_command: $failing_test,
    expected_patch_path: $expected_path,
    workspace_files: [
      {path: "/workspace/project/app.py", content: $app_py},
      {path: "/workspace/project/test_app.py", content: $test_py},
      {path: "/workspace/project/README.md", content: $readme}
    ]
  }')"

echo "==> Seeding SWE-bench-inspired coding fixture into ${DATABASE_URL}"

psql "${DATABASE_URL}" <<SQL
TRUNCATE TABLE challenge_packs, organizations, users, model_catalog_entries RESTART IDENTITY CASCADE;

INSERT INTO organizations (id, name, slug)
VALUES ('${ORG_ID}', 'Local Coding Benchmark Org', 'local-coding-benchmark-org');

INSERT INTO workspaces (id, organization_id, name, slug)
VALUES ('${WORKSPACE_ID}', '${ORG_ID}', 'Local Coding Benchmark Workspace', 'local-coding-benchmark-workspace');

INSERT INTO users (id, workos_user_id, email, display_name)
VALUES ('${USER_ID}', 'workos-local-coding-user', 'local-coding@example.com', 'Local Coding User');

INSERT INTO challenge_packs (id, slug, name, family)
VALUES ('${CHALLENGE_PACK_ID}', 'swebench-mini-pack', 'SWE-bench Mini Pack', 'coding');

INSERT INTO challenge_pack_versions (
  id,
  challenge_pack_id,
  version_number,
  lifecycle_status,
  manifest_checksum,
  manifest
)
VALUES (
  '${CHALLENGE_PACK_VERSION_ID}',
  '${CHALLENGE_PACK_ID}',
  1,
  'runnable',
  'swebench-mini-manifest-v1',
  '{
    "tool_policy":{"allowed_tool_kinds":["file","shell"]},
    "pack_notes":{
      "intended_use":"Measure deterministic patch correctness for a single-file coding task.",
      "limitations":[
        "This pack validates the submitted final output, not an arbitrary workspace diff.",
        "It contains one narrow bugfix task for fast iteration."
      ]
    },
    "evaluation_spec":{
      "name":"swebench-mini-patch-correctness",
      "version_number":1,
      "judge_mode":"deterministic",
      "validators":[
        {
          "key":"output-shape",
          "type":"json_schema",
          "target":"final_output",
          "expected_from":"literal:{\"type\":\"object\",\"required\":[\"file_path\",\"fixed_content\",\"summary\",\"tests_ran\"],\"properties\":{\"file_path\":{\"type\":\"string\"},\"fixed_content\":{\"type\":\"string\"},\"summary\":{\"type\":\"string\"},\"tests_ran\":{\"type\":\"string\"}},\"additionalProperties\":false}"
        },
        {
          "key":"patch-target-path",
          "type":"json_path_match",
          "target":"final_output",
          "expected_from":"literal:{\"path\":\"$.file_path\",\"comparator\":\"equals\",\"value\":\"/workspace/project/app.py\"}"
        },
        {
          "key":"fixed-content-signature",
          "type":"json_path_match",
          "target":"final_output",
          "expected_from":"literal:{\"path\":\"$.fixed_content\",\"comparator\":\"contains\",\"value\":\"return a + b\"}"
        },
        {
          "key":"summary-mentions-regression",
          "type":"json_path_match",
          "target":"final_output",
          "expected_from":"literal:{\"path\":\"$.summary\",\"comparator\":\"contains\",\"value\":\"regression\"}"
        },
        {
          "key":"tests-command-mentioned",
          "type":"json_path_match",
          "target":"final_output",
          "expected_from":"literal:{\"path\":\"$.tests_ran\",\"comparator\":\"contains\",\"value\":\"pytest /workspace/project/test_app.py\"}"
        }
      ],
      "metrics":[
        {
          "key":"completion",
          "type":"boolean",
          "collector":"run_completed_successfully"
        },
        {
          "key":"failures",
          "type":"numeric",
          "collector":"run_failure_count"
        },
        {
          "key":"total-latency-ms",
          "type":"numeric",
          "collector":"run_total_latency_ms",
          "unit":"ms"
        },
        {
          "key":"ttft-ms",
          "type":"numeric",
          "collector":"run_ttft_ms",
          "unit":"ms"
        },
        {
          "key":"model-cost-usd",
          "type":"numeric",
          "collector":"run_model_cost_usd",
          "unit":"usd"
        },
        {
          "key":"validator-pass-rate",
          "type":"numeric",
          "collector":"validator_pass_rate"
        }
      ],
      "runtime_limits":{
        "max_cost_usd":0.25,
        "max_duration_ms":180000
      },
      "pricing":{
        "models":[
          {
            "provider_key":"openai",
            "provider_model_id":"gpt-5.4",
            "input_cost_per_million_tokens":5,
            "output_cost_per_million_tokens":15
          },
          {
            "provider_key":"openai",
            "provider_model_id":"gpt-5.4-nano",
            "input_cost_per_million_tokens":0.1,
            "output_cost_per_million_tokens":0.4
          },
          {
            "provider_key":"openai",
            "provider_model_id":"gpt-4.1",
            "input_cost_per_million_tokens":2,
            "output_cost_per_million_tokens":8
          },
          {
            "provider_key":"openai",
            "provider_model_id":"gpt-4.1-mini",
            "input_cost_per_million_tokens":0.4,
            "output_cost_per_million_tokens":1.6
          },
          {
            "provider_key":"openai",
            "provider_model_id":"gpt-4o-mini",
            "input_cost_per_million_tokens":0.15,
            "output_cost_per_million_tokens":0.6
          }
        ]
      },
      "scorecard":{
        "dimensions":["correctness","reliability","latency","cost"],
        "normalization":{
          "latency":{"target_ms":8000,"max_ms":120000},
          "cost":{"target_usd":0.01,"max_usd":0.25}
        }
      }
    }
  }'::jsonb
);

INSERT INTO challenge_identities (
  id,
  challenge_pack_id,
  challenge_key,
  name,
  category,
  difficulty,
  description
)
VALUES (
  '${CHALLENGE_IDENTITY_ID}',
  '${CHALLENGE_PACK_ID}',
  'patch-correctness-single-file',
  'Fix add() arithmetic regression',
  'coding',
  'medium',
  'Inspect a staged workspace fixture, identify the one-line arithmetic regression, and submit the corrected file content as strict JSON.'
);

INSERT INTO challenge_pack_version_challenges (
  id,
  challenge_pack_version_id,
  challenge_pack_id,
  challenge_identity_id,
  execution_order,
  title_snapshot,
  category_snapshot,
  difficulty_snapshot,
  challenge_definition
)
VALUES (
  '${CHALLENGE_VERSION_ID}',
  '${CHALLENGE_PACK_VERSION_ID}',
  '${CHALLENGE_PACK_ID}',
  '${CHALLENGE_IDENTITY_ID}',
  0,
  'Fix add() arithmetic regression',
  'coding',
  'medium',
  '{
    "instructions":"Inspect the staged workspace under /workspace/project. The bug is intentionally narrow: a small arithmetic regression breaks the staged pytest file. Use the available tools to inspect files, optionally run pytest, update the workspace if helpful, and then submit your final answer as a strict JSON string with keys file_path, fixed_content, summary, and tests_ran. fixed_content must contain the full corrected contents of /workspace/project/app.py. tests_ran must mention the exact command you ran or the command you would run to verify the fix.",
    "intended_signal":"patch correctness under a deterministic single-file bugfix",
    "limitations":["single challenge item","final-output validation only"]
  }'::jsonb
);

INSERT INTO challenge_input_sets (
  id,
  challenge_pack_version_id,
  input_key,
  name,
  input_checksum
)
VALUES (
  '${CHALLENGE_INPUT_SET_ID}',
  '${CHALLENGE_PACK_VERSION_ID}',
  'swebench-mini-inputs',
  'SWE-bench Mini Inputs',
  'swebench-mini-input-checksum'
);

INSERT INTO challenge_input_items (
  id,
  challenge_input_set_id,
  challenge_pack_version_id,
  challenge_identity_id,
  item_key,
  payload
)
VALUES (
  '${CHALLENGE_INPUT_ITEM_ID}',
  '${CHALLENGE_INPUT_SET_ID}',
  '${CHALLENGE_PACK_VERSION_ID}',
  '${CHALLENGE_IDENTITY_ID}',
  'task.json',
  '${INPUT_PAYLOAD}'::jsonb
);

INSERT INTO runtime_profiles (
  id,
  organization_id,
  workspace_id,
  name,
  slug,
  execution_target,
  trace_mode,
  max_iterations,
  max_tool_calls,
  step_timeout_seconds,
  run_timeout_seconds,
  profile_config
)
VALUES (
  '${RUNTIME_PROFILE_ID}',
  '${ORG_ID}',
  '${WORKSPACE_ID}',
  'Local Coding Native Runtime',
  'local-coding-native-runtime',
  'native',
  'preferred',
  8,
  16,
  90,
  240,
  '{
    "sandbox":{
      "working_directory":"/workspace/project",
      "readable_roots":["/workspace"],
      "writable_roots":["/workspace"]
    }
  }'::jsonb
);

INSERT INTO provider_accounts (
  id,
  organization_id,
  workspace_id,
  provider_key,
  name,
  credential_reference,
  limits_config
)
VALUES (
  '${PROVIDER_ACCOUNT_ID}',
  '${ORG_ID}',
  '${WORKSPACE_ID}',
  'openai',
  'Local OpenAI Coding Account',
  'secret://openai',
  '{"rpm":60}'::jsonb
);

INSERT INTO model_catalog_entries (
  id,
  provider_key,
  provider_model_id,
  display_name,
  model_family,
  metadata
)
VALUES
  ('${MODEL_CATALOG_ENTRY_GPT54_ID}', 'openai', 'gpt-5.4', 'GPT-5.4', 'gpt-5.4', '{"tier":"frontier","pack":"swebench-mini"}'::jsonb),
  ('${MODEL_CATALOG_ENTRY_GPT54N_ID}', 'openai', 'gpt-5.4-nano', 'GPT-5.4 Nano', 'gpt-5.4-nano', '{"tier":"small","pack":"swebench-mini"}'::jsonb),
  ('${MODEL_CATALOG_ENTRY_GPT41_ID}', 'openai', 'gpt-4.1', 'GPT-4.1', 'gpt-4.1', '{"tier":"frontier","pack":"swebench-mini"}'::jsonb),
  ('${MODEL_CATALOG_ENTRY_GPT41M_ID}', 'openai', 'gpt-4.1-mini', 'GPT-4.1 Mini', 'gpt-4.1-mini', '{"tier":"small","pack":"swebench-mini"}'::jsonb),
  ('${MODEL_CATALOG_ENTRY_GPT4OM_ID}', 'openai', 'gpt-4o-mini', 'GPT-4o Mini', 'gpt-4o-mini', '{"tier":"small","pack":"swebench-mini"}'::jsonb);

INSERT INTO model_aliases (
  id,
  organization_id,
  workspace_id,
  provider_account_id,
  model_catalog_entry_id,
  alias_key,
  display_name
)
VALUES
  ('${MODEL_ALIAS_GPT54_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_CATALOG_ENTRY_GPT54_ID}', 'swebench-gpt-5-4', 'SWE-bench GPT-5.4'),
  ('${MODEL_ALIAS_GPT54N_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_CATALOG_ENTRY_GPT54N_ID}', 'swebench-gpt-5-4-nano', 'SWE-bench GPT-5.4 Nano'),
  ('${MODEL_ALIAS_GPT41_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_CATALOG_ENTRY_GPT41_ID}', 'swebench-gpt-4-1', 'SWE-bench GPT-4.1'),
  ('${MODEL_ALIAS_GPT41M_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_CATALOG_ENTRY_GPT41M_ID}', 'swebench-gpt-4-1-mini', 'SWE-bench GPT-4.1 Mini'),
  ('${MODEL_ALIAS_GPT4OM_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_CATALOG_ENTRY_GPT4OM_ID}', 'swebench-gpt-4o-mini', 'SWE-bench GPT-4o Mini');

INSERT INTO agent_builds (
  id,
  organization_id,
  workspace_id,
  name,
  slug,
  created_by_user_id
)
VALUES (
  '${AGENT_BUILD_ID}',
  '${ORG_ID}',
  '${WORKSPACE_ID}',
  'SWE-bench Mini Agent',
  'swebench-mini-agent',
  '${USER_ID}'
);

INSERT INTO agent_build_versions (
  id,
  agent_build_id,
  version_number,
  version_status,
  build_definition,
  prompt_spec,
  output_schema,
  trace_contract,
  created_by_user_id
)
VALUES (
  '${AGENT_BUILD_VERSION_ID}',
  '${AGENT_BUILD_ID}',
  1,
  'ready',
  '{"strategy":"inspect-fix-submit"}'::jsonb,
  'Inspect the staged workspace carefully. Minimize guesswork. Fix the regression, preserve the existing file structure, and submit strict JSON only.',
  '{"type":"object","required":["file_path","fixed_content","summary","tests_ran"],"properties":{"file_path":{"type":"string"},"fixed_content":{"type":"string"},"summary":{"type":"string"},"tests_ran":{"type":"string"}},"additionalProperties":false}'::jsonb,
  '{}'::jsonb,
  '${USER_ID}'
);

INSERT INTO agent_deployments (
  id,
  organization_id,
  workspace_id,
  agent_build_id,
  current_build_version_id,
  runtime_profile_id,
  provider_account_id,
  model_alias_id,
  name,
  slug,
  deployment_type,
  deployment_config
)
VALUES
  ('${AGENT_DEPLOYMENT_GPT54_ID}',  '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT54_ID}',  'SWE-bench GPT-5.4',      'swebench-gpt-5-4',      'native', '{}'::jsonb),
  ('${AGENT_DEPLOYMENT_GPT54N_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT54N_ID}', 'SWE-bench GPT-5.4 Nano', 'swebench-gpt-5-4-nano', 'native', '{}'::jsonb),
  ('${AGENT_DEPLOYMENT_GPT41_ID}',  '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT41_ID}',  'SWE-bench GPT-4.1',      'swebench-gpt-4-1',      'native', '{}'::jsonb),
  ('${AGENT_DEPLOYMENT_GPT41M_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT41M_ID}', 'SWE-bench GPT-4.1 Mini', 'swebench-gpt-4-1-mini', 'native', '{}'::jsonb),
  ('${AGENT_DEPLOYMENT_GPT4OM_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT4OM_ID}', 'SWE-bench GPT-4o Mini',  'swebench-gpt-4o-mini',  'native', '{}'::jsonb);

INSERT INTO agent_deployment_snapshots (
  id,
  organization_id,
  workspace_id,
  agent_build_id,
  agent_deployment_id,
  source_agent_build_version_id,
  source_runtime_profile_id,
  source_provider_account_id,
  source_model_alias_id,
  deployment_type,
  snapshot_hash,
  snapshot_config
)
VALUES
  ('${AGENT_DEPLOYMENT_SNAPSHOT_GPT54_ID}',  '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_DEPLOYMENT_GPT54_ID}',  '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT54_ID}',  'native', 'swebench-gpt-5-4-snapshot',      '{"temperature":0.1}'::jsonb),
  ('${AGENT_DEPLOYMENT_SNAPSHOT_GPT54N_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_DEPLOYMENT_GPT54N_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT54N_ID}', 'native', 'swebench-gpt-5-4-nano-snapshot', '{"temperature":0.1}'::jsonb),
  ('${AGENT_DEPLOYMENT_SNAPSHOT_GPT41_ID}',  '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_DEPLOYMENT_GPT41_ID}',  '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT41_ID}',  'native', 'swebench-gpt-4-1-snapshot',      '{"temperature":0.1}'::jsonb),
  ('${AGENT_DEPLOYMENT_SNAPSHOT_GPT41M_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_DEPLOYMENT_GPT41M_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT41M_ID}', 'native', 'swebench-gpt-4-1-mini-snapshot', '{"temperature":0.1}'::jsonb),
  ('${AGENT_DEPLOYMENT_SNAPSHOT_GPT4OM_ID}', '${ORG_ID}', '${WORKSPACE_ID}', '${AGENT_BUILD_ID}', '${AGENT_DEPLOYMENT_GPT4OM_ID}', '${AGENT_BUILD_VERSION_ID}', '${RUNTIME_PROFILE_ID}', '${PROVIDER_ACCOUNT_ID}', '${MODEL_ALIAS_GPT4OM_ID}', 'native', 'swebench-gpt-4o-mini-snapshot',  '{"temperature":0.1}'::jsonb);
SQL

cat <<EOF

Seed complete.

Use these values for curl:
  WORKSPACE_ID=${WORKSPACE_ID}
  USER_ID=${USER_ID}
  CHALLENGE_PACK_VERSION_ID=${CHALLENGE_PACK_VERSION_ID}
  CHALLENGE_INPUT_SET_ID=${CHALLENGE_INPUT_SET_ID}
  AGENT_DEPLOYMENT_IDS=${AGENT_DEPLOYMENT_GPT54_ID},${AGENT_DEPLOYMENT_GPT54N_ID},${AGENT_DEPLOYMENT_GPT41_ID},${AGENT_DEPLOYMENT_GPT41M_ID},${AGENT_DEPLOYMENT_GPT4OM_ID}

Model lineup:
  ${AGENT_DEPLOYMENT_GPT54_ID}  GPT-5.4
  ${AGENT_DEPLOYMENT_GPT54N_ID} GPT-5.4 Nano
  ${AGENT_DEPLOYMENT_GPT41_ID}  GPT-4.1
  ${AGENT_DEPLOYMENT_GPT41M_ID} GPT-4.1 Mini
  ${AGENT_DEPLOYMENT_GPT4OM_ID} GPT-4o Mini

EOF
