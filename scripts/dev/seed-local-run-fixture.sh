#!/usr/bin/env bash
set -euo pipefail

# Seeds a deterministic local development fixture so /v1/runs can be exercised
# with curl using real IDs instead of placeholders.
#
# This is intentionally destructive to the local dev database. It resets the
# benchmark/org/user fixture tables the same way repository integration tests do.
#
# Optional:
#   OPENAI_MODEL   defaults to gpt-4.1-mini
#   DATABASE_URL   loaded from backend/.env or backend/.env.example if present

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
export OPENAI_MODEL="${OPENAI_MODEL:-gpt-4.1-mini}"

ORG_ID="11111111-1111-1111-1111-111111111111"
WORKSPACE_ID="22222222-2222-2222-2222-222222222222"
USER_ID="33333333-3333-3333-3333-333333333333"
CHALLENGE_PACK_ID="44444444-4444-4444-4444-444444444444"
CHALLENGE_PACK_VERSION_ID="55555555-5555-5555-5555-555555555555"
CHALLENGE_IDENTITY_ID="66666666-6666-6666-6666-666666666666"
CHALLENGE_VERSION_ID="77777777-7777-7777-7777-777777777777"
CHALLENGE_INPUT_SET_ID="abababab-abab-abab-abab-abababababab"
CHALLENGE_INPUT_ITEM_ID="cdcdcdcd-cdcd-cdcd-cdcd-cdcdcdcdcdcd"
RUNTIME_PROFILE_ID="88888888-8888-8888-8888-888888888888"
PROVIDER_ACCOUNT_ID="99999999-9999-9999-9999-999999999999"
MODEL_CATALOG_ENTRY_ID="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
MODEL_ALIAS_ID="bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
AGENT_BUILD_ID="cccccccc-cccc-cccc-cccc-cccccccccccc"
AGENT_BUILD_VERSION_ID="dddddddd-dddd-dddd-dddd-dddddddddddd"
AGENT_DEPLOYMENT_ID="eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
AGENT_DEPLOYMENT_SNAPSHOT_ID="ffffffff-ffff-ffff-ffff-ffffffffffff"

echo "==> Seeding local curl fixture into ${DATABASE_URL}"

psql "${DATABASE_URL}" <<SQL
TRUNCATE TABLE challenge_packs, organizations, users, model_catalog_entries RESTART IDENTITY CASCADE;

INSERT INTO organizations (id, name, slug)
VALUES ('${ORG_ID}', 'Local Smoke Org', 'local-smoke-org');

INSERT INTO workspaces (id, organization_id, name, slug)
VALUES ('${WORKSPACE_ID}', '${ORG_ID}', 'Local Smoke Workspace', 'local-smoke-workspace');

INSERT INTO users (id, workos_user_id, email, display_name)
VALUES ('${USER_ID}', 'workos-local-smoke-user', 'local-smoke@example.com', 'Local Smoke User');

INSERT INTO challenge_packs (id, slug, name, family)
VALUES ('${CHALLENGE_PACK_ID}', 'local-smoke-pack', 'Local Smoke Pack', 'reasoning');

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
  'local-smoke-manifest',
  '{
    "tool_policy":{"allowed_tool_kinds":["file"]},
    "evaluation_spec":{
      "name":"local-phase1-scorecard",
      "version_number":1,
      "judge_mode":"deterministic",
      "validators":[
        {
          "key":"output-shape",
          "type":"json_schema",
          "target":"final_output",
          "expected_from":"literal:{\"type\":\"object\",\"required\":[\"code\",\"risk_level\",\"summary\"],\"properties\":{\"code\":{\"type\":\"string\"},\"risk_level\":{\"type\":\"string\"},\"summary\":{\"type\":\"string\"}}}"
        },
        {
          "key":"ticket-code",
          "type":"json_path_match",
          "target":"final_output",
          "expected_from":"literal:{\"path\":\"$.code\",\"comparator\":\"equals\",\"value\":\"KAPPA-4821\"}"
        },
        {
          "key":"risk-level",
          "type":"json_path_match",
          "target":"final_output",
          "expected_from":"literal:{\"path\":\"$.risk_level\",\"comparator\":\"equals\",\"value\":\"high\"}"
        },
        {
          "key":"summary-mentions-cache",
          "type":"json_path_match",
          "target":"final_output",
          "expected_from":"literal:{\"path\":\"$.summary\",\"comparator\":\"contains\",\"value\":\"cache invalidation\"}"
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
        "max_cost_usd":0.2,
        "max_duration_ms":90000
      },
      "pricing":{
        "models":[
          {
            "provider_key":"openai",
            "provider_model_id":"gpt-4.1-mini",
            "input_cost_per_million_tokens":0.4,
            "output_cost_per_million_tokens":1.6
          }
        ]
      },
      "scorecard":{
        "dimensions":["correctness","reliability","latency","cost"],
        "normalization":{
          "latency":{"target_ms":2500,"max_ms":20000},
          "cost":{"target_usd":0.002,"max_usd":0.05}
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
  'local-smoke-challenge',
  'Local Smoke Challenge',
  'support',
  'easy',
  'Local smoke test challenge'
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
  'Local Smoke Challenge',
  'support',
  'easy',
  '{"instructions":"Return strict JSON with keys code, risk_level, and summary. Use code KAPPA-4821, risk_level high, and mention cache invalidation in the summary."}'::jsonb
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
  'local-phase1-inputs',
  'Local Phase 1 Inputs',
  'local-phase1-input-checksum'
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
  'prompt.json',
  '{"content":"Investigate incident KAPPA-4821. The incident points to cache invalidation issues causing stale payload replay. Output strict JSON with code, risk_level, and summary."}'::jsonb
);

INSERT INTO runtime_profiles (
  id,
  organization_id,
  workspace_id,
  name,
  slug,
  execution_target
)
VALUES (
  '${RUNTIME_PROFILE_ID}',
  '${ORG_ID}',
  '${WORKSPACE_ID}',
  'Local Native Runtime',
  'local-native-runtime',
  'native'
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
  'Local OpenAI Account',
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
VALUES (
  '${MODEL_CATALOG_ENTRY_ID}',
  'openai',
  '${OPENAI_MODEL}',
  '${OPENAI_MODEL}',
  '${OPENAI_MODEL}',
  '{"tier":"smoke"}'::jsonb
);

INSERT INTO model_aliases (
  id,
  organization_id,
  workspace_id,
  provider_account_id,
  model_catalog_entry_id,
  alias_key,
  display_name
)
VALUES (
  '${MODEL_ALIAS_ID}',
  '${ORG_ID}',
  '${WORKSPACE_ID}',
  '${PROVIDER_ACCOUNT_ID}',
  '${MODEL_CATALOG_ENTRY_ID}',
  'local-primary-model',
  'Local Primary Model'
);

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
  'Local Smoke Agent',
  'local-smoke-agent',
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
  '{"strategy":"respond briefly"}'::jsonb,
  'You are a precise local smoke-test agent.',
  '{"type":"object","properties":{"answer":{"type":"string"}}}'::jsonb,
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
VALUES (
  '${AGENT_DEPLOYMENT_ID}',
  '${ORG_ID}',
  '${WORKSPACE_ID}',
  '${AGENT_BUILD_ID}',
  '${AGENT_BUILD_VERSION_ID}',
  '${RUNTIME_PROFILE_ID}',
  '${PROVIDER_ACCOUNT_ID}',
  '${MODEL_ALIAS_ID}',
  'Local Smoke Deployment',
  'local-smoke-deployment',
  'native',
  '{}'::jsonb
);

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
VALUES (
  '${AGENT_DEPLOYMENT_SNAPSHOT_ID}',
  '${ORG_ID}',
  '${WORKSPACE_ID}',
  '${AGENT_BUILD_ID}',
  '${AGENT_DEPLOYMENT_ID}',
  '${AGENT_BUILD_VERSION_ID}',
  '${RUNTIME_PROFILE_ID}',
  '${PROVIDER_ACCOUNT_ID}',
  '${MODEL_ALIAS_ID}',
  'native',
  'local-smoke-snapshot',
  '{"temperature":0.1}'::jsonb
);
SQL

cat <<EOF

Seed complete.

Use these values for curl:
  WORKSPACE_ID=${WORKSPACE_ID}
  USER_ID=${USER_ID}
  CHALLENGE_PACK_VERSION_ID=${CHALLENGE_PACK_VERSION_ID}
  CHALLENGE_INPUT_SET_ID=${CHALLENGE_INPUT_SET_ID}
  AGENT_DEPLOYMENT_ID=${AGENT_DEPLOYMENT_ID}

EOF
