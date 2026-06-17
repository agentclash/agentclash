package repository_test

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestListRunnableDeployments_SnapshotBillingFields proves the 4d-1 query extension: BYOK signal,
// model identity, and the FROZEN output rate all come from the snapshot → model_alias → catalog path
// (the frozen alias rate, not a live catalog read).
func TestListRunnableDeployments_SnapshotBillingFields(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	org := uuid.New()
	ws := uuid.New()
	user := uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\nsql: %s", err, sql)
		}
	}
	exec(`INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`, org, "o", uniqueSlug("o"))
	exec(`INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1,$2,$3,$4)`, ws, org, "w", uniqueSlug("w"))
	exec(`INSERT INTO users (id, workos_user_id, email, display_name) VALUES ($1,$2,$3,$4)`, user, "wk-"+user.String()[:8], user.String()[:8]+"@e.com", "U")

	runtimeProfile := uuid.New()
	exec(`INSERT INTO runtime_profiles (id, organization_id, workspace_id, name, slug, execution_target) VALUES ($1,$2,$3,$4,$5,'native')`, runtimeProfile, org, ws, "rp", uniqueSlug("rp"))
	providerAccount := uuid.New()
	exec(`INSERT INTO provider_accounts (id, organization_id, workspace_id, provider_key, name, credential_reference, limits_config) VALUES ($1,$2,$3,'openai','acct','secret://x','{}'::jsonb)`, providerAccount, org, ws)
	catalog := uuid.New()
	modelID := "gpt-4.1-" + catalog.String()[:8] // unique per run (model_catalog UNIQUE on provider_key+model_id)
	exec(`INSERT INTO model_catalog_entries (id, provider_key, provider_model_id, display_name, model_family, metadata) VALUES ($1,'openai',$2,'GPT-4.1','gpt-4.1','{}'::jsonb)`, catalog, modelID)
	// FROZEN alias output rate = 5.0 (distinctive) — what the estimate must use.
	alias := uuid.New()
	exec(`INSERT INTO model_aliases (id, organization_id, workspace_id, provider_account_id, model_catalog_entry_id, alias_key, display_name, output_cost_per_million_tokens) VALUES ($1,$2,$3,$4,$5,'a','A',5.0)`, alias, org, ws, providerAccount, catalog)
	build := uuid.New()
	exec(`INSERT INTO agent_builds (id, organization_id, workspace_id, name, slug, created_by_user_id) VALUES ($1,$2,$3,'b',$4,$5)`, build, org, ws, uniqueSlug("b"), user)
	buildVersion := uuid.New()
	exec(`INSERT INTO agent_build_versions (id, agent_build_id, version_number, version_status, build_definition, prompt_spec, output_schema, trace_contract, created_by_user_id) VALUES ($1,$2,1,'ready','{}'::jsonb,'p','{}'::jsonb,'{}'::jsonb,$3)`, buildVersion, build, user)

	managed := seedDeploymentWithSnapshot(t, ctx, db, org, ws, build, buildVersion, runtimeProfile, alias, nil, alias) // managed: no source provider account
	byok := seedDeploymentWithSnapshot(t, ctx, db, org, ws, build, buildVersion, runtimeProfile, alias, &providerAccount, alias)

	deployments, err := repo.ListRunnableDeploymentsWithLatestSnapshot(ctx, ws, []uuid.UUID{managed, byok})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[uuid.UUID]repository.RunnableDeployment{}
	for _, d := range deployments {
		byID[d.ID] = d
	}
	m, b := byID[managed], byID[byok]

	// Managed lane: no source provider account; identity + FROZEN rate from the snapshot path.
	if m.SourceProviderAccountID != nil {
		t.Fatalf("managed lane SourceProviderAccountID = %v, want nil", m.SourceProviderAccountID)
	}
	if m.ProviderKey != "openai" || m.ProviderModelID != modelID {
		t.Fatalf("managed lane model identity = %s/%s, want openai/%s", m.ProviderKey, m.ProviderModelID, modelID)
	}
	if m.OutputCostPerMillionTokens != 5.0 {
		t.Fatalf("managed lane frozen rate = %v, want 5.0 (from the model alias)", m.OutputCostPerMillionTokens)
	}
	// BYOK lane: source provider account present.
	if b.SourceProviderAccountID == nil || *b.SourceProviderAccountID != providerAccount {
		t.Fatalf("byok lane SourceProviderAccountID = %v, want %s", b.SourceProviderAccountID, providerAccount)
	}
}

func seedDeploymentWithSnapshot(t *testing.T, ctx context.Context, db *pgxpool.Pool, org, ws, build, buildVersion, runtimeProfile, alias uuid.UUID, sourceProviderAccount *uuid.UUID, sourceAlias uuid.UUID) uuid.UUID {
	t.Helper()
	deployment := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO agent_deployments (id, organization_id, workspace_id, agent_build_id, current_build_version_id, runtime_profile_id, model_alias_id, name, slug, deployment_type, deployment_config) VALUES ($1,$2,$3,$4,$5,$6,$7,'d',$8,'native','{}'::jsonb)`,
		deployment, org, ws, build, buildVersion, runtimeProfile, alias, uniqueSlug("d")); err != nil {
		t.Fatalf("seed deployment: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO agent_deployment_snapshots (id, organization_id, workspace_id, agent_build_id, agent_deployment_id, source_agent_build_version_id, source_runtime_profile_id, source_provider_account_id, source_model_alias_id, deployment_type, snapshot_hash, snapshot_config) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'native',$10,'{}'::jsonb)`,
		uuid.New(), org, ws, build, deployment, buildVersion, runtimeProfile, sourceProviderAccount, sourceAlias, "hash-"+deployment.String()[:8]); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	return deployment
}
