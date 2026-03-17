-- Dev seed data for AgentClash
-- Matches the frontend dev auth preset:
--   User:      11111111-1111-1111-1111-111111111111
--   Workspace: 22222222-2222-2222-2222-222222222222
--
-- Run: psql "$DATABASE_URL" -f scripts/db/seed-dev.sql
-- Or:  make db-seed

BEGIN;

-- ─── Organization ───
INSERT INTO organizations (id, name, slug)
VALUES ('33333333-3333-3333-3333-333333333333', 'Dev Org', 'dev-org')
ON CONFLICT (slug) DO NOTHING;

-- ─── Workspace ───
INSERT INTO workspaces (id, organization_id, name, slug)
VALUES ('22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333', 'Dev Workspace', 'dev-workspace')
ON CONFLICT (id) DO NOTHING;

-- ─── User ───
INSERT INTO users (id, workos_user_id, email, display_name)
VALUES ('11111111-1111-1111-1111-111111111111', 'dev-workos-user-001', 'dev@agentclash.dev', 'Dev User')
ON CONFLICT (id) DO NOTHING;

-- ─── Organization Membership ───
INSERT INTO organization_memberships (organization_id, user_id, role)
VALUES ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111', 'org_admin')
ON CONFLICT (organization_id, user_id) DO NOTHING;

-- ─── Workspace Membership ───
INSERT INTO workspace_memberships (organization_id, workspace_id, user_id, role)
VALUES ('33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', 'workspace_admin')
ON CONFLICT (workspace_id, user_id) DO NOTHING;

-- ─── Challenge Pack + Version ───
INSERT INTO challenge_packs (id, slug, name, family, description, lifecycle_status)
VALUES ('44444444-4444-4444-4444-444444444444', 'fix-auth-server', 'Fix Auth Server', 'code-repair', 'A broken Go auth server with timestamp parsing and Bearer prefix bugs', 'active')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO challenge_pack_versions (id, challenge_pack_id, version_number, lifecycle_status, manifest_checksum, manifest, published_at)
VALUES ('55555555-5555-5555-5555-555555555555', '44444444-4444-4444-4444-444444444444', 1, 'runnable', 'dev-seed-checksum', '{"challenges": 2, "description": "v1 — two auth bugs"}'::jsonb, now())
ON CONFLICT (challenge_pack_id, version_number) DO NOTHING;

-- ─── Provider Infrastructure (needed by agent_deployments FK chain) ───
INSERT INTO provider_accounts (id, organization_id, provider_key, name, credential_reference)
VALUES ('66666666-6666-6666-6666-666666666661', '33333333-3333-3333-3333-333333333333', 'openai', 'OpenAI Dev', 'env:OPENAI_API_KEY')
ON CONFLICT (id) DO NOTHING;

INSERT INTO model_catalog_entries (id, provider_key, provider_model_id, display_name, model_family)
VALUES
  ('77777777-7777-7777-7777-777777777771', 'openai', 'gpt-4.1', 'GPT-4.1', 'gpt-4'),
  ('77777777-7777-7777-7777-777777777772', 'anthropic', 'claude-sonnet-4-6', 'Claude Sonnet 4.6', 'claude-4')
ON CONFLICT (provider_key, provider_model_id) DO NOTHING;

INSERT INTO model_aliases (id, organization_id, provider_account_id, model_catalog_entry_id, alias_key, display_name)
VALUES
  ('88888888-8888-8888-8888-888888888881', '33333333-3333-3333-3333-333333333333', '66666666-6666-6666-6666-666666666661', '77777777-7777-7777-7777-777777777771', 'gpt-4.1', 'GPT-4.1')
ON CONFLICT (id) DO NOTHING;

INSERT INTO routing_policies (id, organization_id, name, policy_kind, config)
VALUES ('99999999-9999-9999-9999-999999999991', '33333333-3333-3333-3333-333333333333', 'single-model-dev', 'single_model', '{}'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO spend_policies (id, organization_id, name, window_kind, hard_limit)
VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', 'dev-budget', 'run', 10.00)
ON CONFLICT (id) DO NOTHING;

INSERT INTO runtime_profiles (id, organization_id, name, slug, execution_target, max_iterations, run_timeout_seconds)
VALUES ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1', '33333333-3333-3333-3333-333333333333', 'Default Dev', 'default-dev', 'hosted_external', 10, 600)
ON CONFLICT (id) DO NOTHING;

-- ─── Agent Builds ───
INSERT INTO agent_builds (id, organization_id, workspace_id, name, slug, created_by_user_id)
VALUES
  ('cccccccc-cccc-cccc-cccc-cccccccccc01', '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'GPT-4.1 Agent', 'gpt-4-1-agent', '11111111-1111-1111-1111-111111111111'),
  ('cccccccc-cccc-cccc-cccc-cccccccccc02', '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'Claude Sonnet Agent', 'claude-sonnet-agent', '11111111-1111-1111-1111-111111111111')
ON CONFLICT (id) DO NOTHING;

INSERT INTO agent_build_versions (id, agent_build_id, version_number, version_status)
VALUES
  ('dddddddd-dddd-dddd-dddd-dddddddddd01', 'cccccccc-cccc-cccc-cccc-cccccccccc01', 1, 'ready'),
  ('dddddddd-dddd-dddd-dddd-dddddddddd02', 'cccccccc-cccc-cccc-cccc-cccccccccc02', 1, 'ready')
ON CONFLICT (agent_build_id, version_number) DO NOTHING;

-- ─── Agent Deployments ───
INSERT INTO agent_deployments (id, organization_id, workspace_id, agent_build_id, current_build_version_id, runtime_profile_id, provider_account_id, model_alias_id, routing_policy_id, spend_policy_id, name, slug, deployment_type, endpoint_url, status)
VALUES
  ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee01', '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'cccccccc-cccc-cccc-cccc-cccccccccc01', 'dddddddd-dddd-dddd-dddd-dddddddddd01', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1', '66666666-6666-6666-6666-666666666661', '88888888-8888-8888-8888-888888888881', '99999999-9999-9999-9999-999999999991', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'GPT-4.1', 'gpt-4-1', 'hosted_external', 'https://api.openai.com/v1', 'active'),
  ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee02', '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'cccccccc-cccc-cccc-cccc-cccccccccc02', 'dddddddd-dddd-dddd-dddd-dddddddddd02', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1', '66666666-6666-6666-6666-666666666661', '88888888-8888-8888-8888-888888888881', '99999999-9999-9999-9999-999999999991', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Claude Sonnet 4.6', 'claude-sonnet', 'hosted_external', 'https://api.anthropic.com/v1', 'active')
ON CONFLICT (id) DO NOTHING;

-- ─── Agent Deployment Snapshots (required by run creation) ───
INSERT INTO agent_deployment_snapshots (id, organization_id, workspace_id, agent_build_id, agent_deployment_id, source_agent_build_version_id, source_runtime_profile_id, source_provider_account_id, source_model_alias_id, source_routing_policy_id, source_spend_policy_id, deployment_type, endpoint_url, snapshot_hash, snapshot_config)
VALUES
  ('ffffffff-ffff-ffff-ffff-ffffffffffff', '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'cccccccc-cccc-cccc-cccc-cccccccccc01', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeee01', 'dddddddd-dddd-dddd-dddd-dddddddddd01', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1', '66666666-6666-6666-6666-666666666661', '88888888-8888-8888-8888-888888888881', '99999999-9999-9999-9999-999999999991', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'hosted_external', 'https://api.openai.com/v1', 'snap-gpt41-v1', '{}'),
  ('ffffffff-ffff-ffff-ffff-fffffffffff2', '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'cccccccc-cccc-cccc-cccc-cccccccccc02', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeee02', 'dddddddd-dddd-dddd-dddd-dddddddddd02', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1', '66666666-6666-6666-6666-666666666661', '88888888-8888-8888-8888-888888888881', '99999999-9999-9999-9999-999999999991', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'hosted_external', 'https://api.anthropic.com/v1', 'snap-claude-v1', '{}')
ON CONFLICT (agent_deployment_id, snapshot_hash) DO NOTHING;

COMMIT;
