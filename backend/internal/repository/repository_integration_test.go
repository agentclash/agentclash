package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryGetRunByID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	run, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID returned error: %v", err)
	}

	if run.ID != fixture.runID {
		t.Fatalf("run id = %s, want %s", run.ID, fixture.runID)
	}
	if run.Status != domain.RunStatusDraft {
		t.Fatalf("run status = %s, want %s", run.Status, domain.RunStatusDraft)
	}
	if run.Name != fixture.runName {
		t.Fatalf("run name = %q, want %q", run.Name, fixture.runName)
	}
	if run.TemporalWorkflowID != nil || run.TemporalRunID != nil {
		t.Fatalf("expected temporal ids to be unset")
	}
}

func TestRepositoryPublishChallengePackBundle(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	published, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		Bundle: challengepack.Bundle{
			Pack: challengepack.PackMetadata{
				Slug:        "customer-support-pack",
				Name:        "Customer Support Pack",
				Family:      "support",
				Description: stringPtr("Workspace-scoped support benchmark"),
			},
			Version: challengepack.VersionMetadata{
				Number: 1,
				ToolPolicy: map[string]any{
					"allowed_tool_kinds": []string{"file"},
				},
				EvaluationSpec: scoring.EvaluationSpec{
					Name:          "support-pack-v1",
					VersionNumber: 1,
					JudgeMode:     scoring.JudgeModeDeterministic,
					Validators: []scoring.ValidatorDeclaration{
						{Key: "exact", Type: scoring.ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
					},
					Scorecard: scoring.ScorecardDeclaration{
						Dimensions: []scoring.DimensionDeclaration{{Key: "correctness"}},
					},
				},
				Assets: []challengepack.AssetReference{
					{Key: "workspace-archive", Path: "assets/workspace.zip"},
				},
			},
			Challenges: []challengepack.ChallengeDefinition{
				{
					Key:          "ticket-1",
					Title:        "Ticket One",
					Category:     "support",
					Difficulty:   "easy",
					Instructions: "Handle the customer issue",
				},
			},
			InputSets: []challengepack.InputSetDefinition{
				{
					Key:  "default",
					Name: "Default",
					Cases: []challengepack.CaseDefinition{
						{
							ChallengeKey: "ticket-1",
							CaseKey:      "prompt.txt",
							Inputs: []challengepack.CaseInput{
								{Key: "prompt", Kind: "text", Value: "Customer needs help"},
							},
							Expectations: []challengepack.CaseExpectation{
								{Key: "answer", Kind: "text", Source: "input:prompt"},
							},
						},
					},
				},
			},
		},
		BundleArtifact: &repository.CreateArtifactParams{
			ArtifactType:    "challenge_pack_bundle",
			StorageBucket:   "bundle-bucket",
			StorageKey:      "challenge-pack-bundles/customer-support-pack/v1.yaml",
			ContentType:     stringPtr("application/yaml"),
			SizeBytes:       int64Ptr(128),
			ChecksumSHA256:  stringPtr("bundle-checksum"),
			Visibility:      "private",
			RetentionStatus: "active",
			Metadata:        []byte(`{"filename":"customer-support-pack-v1.yaml","artifact_role":"challenge_pack_bundle"}`),
		},
	})
	if err != nil {
		t.Fatalf("PublishChallengePackBundle returned error: %v", err)
	}
	if published.BundleArtifactID == nil {
		t.Fatal("bundle artifact id is nil")
	}

	var workspaceID *uuid.UUID
	var manifest json.RawMessage
	if err := db.QueryRow(ctx, `
		SELECT cp.workspace_id, cpv.manifest
		FROM challenge_packs cp
		JOIN challenge_pack_versions cpv ON cpv.challenge_pack_id = cp.id
		WHERE cp.id = $1 AND cpv.id = $2
	`, published.ChallengePackID, published.ChallengePackVersionID).Scan(&workspaceID, &manifest); err != nil {
		t.Fatalf("load published challenge pack returned error: %v", err)
	}
	if workspaceID == nil || *workspaceID != fixture.workspaceID {
		t.Fatalf("workspace_id = %v, want %s", workspaceID, fixture.workspaceID)
	}

	var manifestDoc map[string]any
	if err := json.Unmarshal(manifest, &manifestDoc); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifestDoc["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %#v, want 1", manifestDoc["schema_version"])
	}
	inputSets, ok := manifestDoc["input_sets"].([]any)
	if !ok || len(inputSets) != 1 {
		t.Fatalf("input_sets = %#v, want one input set", manifestDoc["input_sets"])
	}
	firstInputSet, ok := inputSets[0].(map[string]any)
	if !ok {
		t.Fatalf("first input set type = %T, want object", inputSets[0])
	}
	cases, ok := firstInputSet["cases"].([]any)
	if !ok || len(cases) != 1 {
		t.Fatalf("cases = %#v, want one case", firstInputSet["cases"])
	}

	runnableVersion, err := repo.GetRunnableChallengePackVersionByID(ctx, published.ChallengePackVersionID)
	if err != nil {
		t.Fatalf("GetRunnableChallengePackVersionByID returned error: %v", err)
	}
	if runnableVersion.WorkspaceID == nil || *runnableVersion.WorkspaceID != fixture.workspaceID {
		t.Fatalf("runnable workspace_id = %v, want %s", runnableVersion.WorkspaceID, fixture.workspaceID)
	}

	bundleArtifact, err := repo.GetArtifactByID(ctx, *published.BundleArtifactID)
	if err != nil {
		t.Fatalf("GetArtifactByID returned error: %v", err)
	}
	if bundleArtifact.ArtifactType != "challenge_pack_bundle" {
		t.Fatalf("artifact_type = %q, want challenge_pack_bundle", bundleArtifact.ArtifactType)
	}
	var metadata map[string]any
	if err := json.Unmarshal(bundleArtifact.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal bundle artifact metadata: %v", err)
	}
	if metadata["challenge_pack_version_id"] != published.ChallengePackVersionID.String() {
		t.Fatalf("metadata challenge_pack_version_id = %#v, want %s", metadata["challenge_pack_version_id"], published.ChallengePackVersionID)
	}
	if metadata["challenge_pack_slug"] != "customer-support-pack" {
		t.Fatalf("metadata challenge_pack_slug = %#v, want customer-support-pack", metadata["challenge_pack_slug"])
	}
}

func TestRepositoryPublishChallengePackBundleRejectsDuplicateVersion(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	params := repository.PublishChallengePackBundleParams{
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		Bundle: challengepack.Bundle{
			Pack: challengepack.PackMetadata{
				Slug:   "customer-support-pack",
				Name:   "Customer Support Pack",
				Family: "support",
			},
			Version: challengepack.VersionMetadata{
				Number: 1,
				EvaluationSpec: scoring.EvaluationSpec{
					Name:          "support-pack-v1",
					VersionNumber: 1,
					JudgeMode:     scoring.JudgeModeDeterministic,
					Validators: []scoring.ValidatorDeclaration{
						{Key: "exact", Type: scoring.ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
					},
					Scorecard: scoring.ScorecardDeclaration{
						Dimensions: []scoring.DimensionDeclaration{{Key: "correctness"}},
					},
				},
			},
			Challenges: []challengepack.ChallengeDefinition{
				{Key: "ticket-1", Title: "Ticket One", Category: "support", Difficulty: "easy"},
			},
		},
	}

	if _, err := repo.PublishChallengePackBundle(ctx, params); err != nil {
		t.Fatalf("first PublishChallengePackBundle returned error: %v", err)
	}
	if _, err := repo.PublishChallengePackBundle(ctx, params); !errors.Is(err, repository.ErrChallengePackVersionExists) {
		t.Fatalf("second PublishChallengePackBundle error = %v, want ErrChallengePackVersionExists", err)
	}
}

func TestRepositoryListRunAgentsByRunID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	runAgents, err := repo.ListRunAgentsByRunID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("ListRunAgentsByRunID returned error: %v", err)
	}

	if len(runAgents) != 2 {
		t.Fatalf("run agent count = %d, want 2", len(runAgents))
	}
	if runAgents[0].ID != fixture.primaryRunAgentID {
		t.Fatalf("first run agent id = %s, want %s", runAgents[0].ID, fixture.primaryRunAgentID)
	}
	if runAgents[0].LaneIndex != 0 {
		t.Fatalf("first lane index = %d, want 0", runAgents[0].LaneIndex)
	}
	if runAgents[1].ID != fixture.secondaryRunAgentID {
		t.Fatalf("second run agent id = %s, want %s", runAgents[1].ID, fixture.secondaryRunAgentID)
	}
	if runAgents[1].LaneIndex != 1 {
		t.Fatalf("second lane index = %d, want 1", runAgents[1].LaneIndex)
	}
}

func stringPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func TestRepositoryGetRunAgentExecutionContextByIDNative(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE runtime_profiles
		SET trace_mode = 'preferred',
		    max_iterations = 3,
		    max_tool_calls = 5,
		    step_timeout_seconds = 90,
		    run_timeout_seconds = 600,
		    profile_config = $2
		WHERE id = $1
	`, fixture.runtimeProfileID, []byte(`{"sandbox":"fake"}`)); err != nil {
		t.Fatalf("update runtime profile returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		UPDATE agent_deployment_snapshots
		SET snapshot_config = $2
		WHERE id = $1
	`, fixture.agentDeploymentSnapshotID, []byte(`{"entrypoint":"runner"}`)); err != nil {
		t.Fatalf("update snapshot config returned error: %v", err)
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned error: %v", err)
	}

	if executionContext.RunAgent.ID != fixture.primaryRunAgentID {
		t.Fatalf("run agent id = %s, want %s", executionContext.RunAgent.ID, fixture.primaryRunAgentID)
	}
	if executionContext.Run.ID != fixture.runID {
		t.Fatalf("run id = %s, want %s", executionContext.Run.ID, fixture.runID)
	}
	if executionContext.ChallengePackVersion.ID != fixture.challengePackVersionID {
		t.Fatalf("challenge pack version id = %s, want %s", executionContext.ChallengePackVersion.ID, fixture.challengePackVersionID)
	}
	if len(executionContext.ChallengePackVersion.Challenges) != 2 {
		t.Fatalf("challenge count = %d, want 2", len(executionContext.ChallengePackVersion.Challenges))
	}
	if executionContext.ChallengePackVersion.Challenges[0].ChallengeKey != "first-ticket" {
		t.Fatalf("first challenge key = %q, want first-ticket", executionContext.ChallengePackVersion.Challenges[0].ChallengeKey)
	}
	if executionContext.ChallengePackVersion.Challenges[1].Title != "Ticket Two" {
		t.Fatalf("second challenge title = %q, want Ticket Two", executionContext.ChallengePackVersion.Challenges[1].Title)
	}
	if !jsonEqual(executionContext.ChallengePackVersion.Challenges[0].Definition, []byte(`{"instructions":"Solve the first ticket"}`)) {
		t.Fatalf("first challenge definition = %s, want first ticket definition", executionContext.ChallengePackVersion.Challenges[0].Definition)
	}
	if executionContext.ChallengeInputSet == nil || executionContext.ChallengeInputSet.ID != fixture.challengeInputSetID {
		t.Fatalf("challenge input set = %#v, want id %s", executionContext.ChallengeInputSet, fixture.challengeInputSetID)
	}
	if len(executionContext.ChallengeInputSet.Items) != 2 {
		t.Fatalf("challenge input item count = %d, want 2", len(executionContext.ChallengeInputSet.Items))
	}
	if len(executionContext.ChallengeInputSet.Cases) != 2 {
		t.Fatalf("challenge case count = %d, want 2", len(executionContext.ChallengeInputSet.Cases))
	}
	if executionContext.ChallengeInputSet.Items[0].ChallengeKey != "first-ticket" {
		t.Fatalf("first challenge input item key = %q, want first-ticket", executionContext.ChallengeInputSet.Items[0].ChallengeKey)
	}
	if executionContext.ChallengeInputSet.Cases[0].CaseKey != executionContext.ChallengeInputSet.Items[0].ItemKey {
		t.Fatalf("first challenge case key = %q, want %q", executionContext.ChallengeInputSet.Cases[0].CaseKey, executionContext.ChallengeInputSet.Items[0].ItemKey)
	}
	if !jsonEqual(executionContext.ChallengeInputSet.Items[1].Payload, []byte(`{"content":"Customer two needs follow-up"}`)) {
		t.Fatalf("second challenge input payload = %s, want second item payload", executionContext.ChallengeInputSet.Items[1].Payload)
	}
	if !jsonEqual(executionContext.ChallengeInputSet.Cases[1].Payload, []byte(`{"content":"Customer two needs follow-up"}`)) {
		t.Fatalf("second challenge case payload = %s, want second case payload", executionContext.ChallengeInputSet.Cases[1].Payload)
	}
	if executionContext.Deployment.DeploymentType != "native" {
		t.Fatalf("deployment type = %q, want native", executionContext.Deployment.DeploymentType)
	}
	if executionContext.Deployment.EndpointURL != nil {
		t.Fatalf("native deployment endpoint_url = %v, want nil", executionContext.Deployment.EndpointURL)
	}
	if executionContext.Deployment.RuntimeProfile.ExecutionTarget != "native" {
		t.Fatalf("runtime execution target = %q, want native", executionContext.Deployment.RuntimeProfile.ExecutionTarget)
	}
	if executionContext.Deployment.RuntimeProfile.TraceMode != "preferred" {
		t.Fatalf("trace mode = %q, want preferred", executionContext.Deployment.RuntimeProfile.TraceMode)
	}
	if !jsonEqual(executionContext.Deployment.SnapshotConfig, []byte(`{"entrypoint":"runner"}`)) {
		t.Fatalf("snapshot config = %s, want entrypoint runner", executionContext.Deployment.SnapshotConfig)
	}
	if executionContext.Deployment.AgentBuildVersion.AgentKind != "llm_agent" {
		t.Fatalf("agent kind = %q, want llm_agent", executionContext.Deployment.AgentBuildVersion.AgentKind)
	}
	if !jsonEqual(executionContext.Deployment.AgentBuildVersion.PolicySpec, []byte(`{"instructions":"You are a precise support benchmark agent."}`)) {
		t.Fatalf("policy spec = %s, want benchmark instructions", executionContext.Deployment.AgentBuildVersion.PolicySpec)
	}
	if !jsonEqual(executionContext.Deployment.AgentBuildVersion.OutputSchema, []byte(`{"type":"object","properties":{"answer":{"type":"string"}}}`)) {
		t.Fatalf("output schema = %s, want answer schema", executionContext.Deployment.AgentBuildVersion.OutputSchema)
	}
	if !jsonEqual(executionContext.Deployment.AgentBuildVersion.AgentSpec, []byte(`{"agent_kind":"llm_agent","guardrail_spec":{},"interface_spec":{},"knowledge_sources":[],"memory_spec":{},"model_spec":{},"output_schema":{"type":"object","properties":{"answer":{"type":"string"}}},"policy_spec":{"instructions":"You are a precise support benchmark agent."},"publication_spec":{},"reasoning_spec":{},"tools":[],"trace_contract":{},"workflow_spec":{}}`)) {
		t.Fatalf("agent spec = %s, want frozen canonical spec", executionContext.Deployment.AgentBuildVersion.AgentSpec)
	}
	if executionContext.Deployment.ProviderAccount == nil || executionContext.Deployment.ProviderAccount.ID != fixture.providerAccountID {
		t.Fatalf("provider account = %#v, want %s", executionContext.Deployment.ProviderAccount, fixture.providerAccountID)
	}
	if executionContext.Deployment.ModelAlias == nil || executionContext.Deployment.ModelAlias.ID != fixture.modelAliasID {
		t.Fatalf("model alias = %#v, want %s", executionContext.Deployment.ModelAlias, fixture.modelAliasID)
	}
}

func TestRepositoryGetRunAgentExecutionContextByIDWithoutChallengeInputSet(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE runs
		SET challenge_input_set_id = NULL
		WHERE id = $1
	`, fixture.runID); err != nil {
		t.Fatalf("clear run challenge_input_set_id returned error: %v", err)
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned error: %v", err)
	}

	if executionContext.Run.ChallengeInputSetID != nil {
		t.Fatalf("run challenge_input_set_id = %v, want nil", executionContext.Run.ChallengeInputSetID)
	}
	if executionContext.ChallengeInputSet != nil {
		t.Fatalf("challenge input set = %#v, want nil", executionContext.ChallengeInputSet)
	}
}

func TestRepositoryGetRunAgentExecutionContextByIDUsesFrozenSourceAgentSpec(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE agent_build_versions
		SET policy_spec = $2,
		    output_schema = $3
		WHERE id = $1
	`, fixture.agentBuildVersionID, []byte(`{"instructions":"mutated after snapshot"}`), []byte(`{"type":"object","properties":{"changed":{"type":"boolean"}}}`)); err != nil {
		t.Fatalf("update agent build version returned error: %v", err)
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned error: %v", err)
	}

	if !jsonEqual(executionContext.Deployment.AgentBuildVersion.PolicySpec, []byte(`{"instructions":"You are a precise support benchmark agent."}`)) {
		t.Fatalf("policy spec = %s, want frozen snapshot instructions", executionContext.Deployment.AgentBuildVersion.PolicySpec)
	}
	if !jsonEqual(executionContext.Deployment.AgentBuildVersion.OutputSchema, []byte(`{"type":"object","properties":{"answer":{"type":"string"}}}`)) {
		t.Fatalf("output schema = %s, want frozen snapshot schema", executionContext.Deployment.AgentBuildVersion.OutputSchema)
	}
}

func TestRepositoryAgentSpecSchemaRejectsNonObjectJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)

	if _, err := db.Exec(ctx, `
		UPDATE agent_build_versions
		SET policy_spec = '[]'::jsonb
		WHERE id = $1
	`, fixture.agentBuildVersionID); err == nil {
		t.Fatalf("expected policy_spec object constraint violation")
	}

	if _, err := db.Exec(ctx, `
		UPDATE agent_deployment_snapshots
		SET source_agent_spec = '42'::jsonb
		WHERE id = $1
	`, fixture.agentDeploymentSnapshotID); err == nil {
		t.Fatalf("expected source_agent_spec object constraint violation")
	}
}

func TestRepositoryGetRunAgentExecutionContextByIDHosted(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	hostedRuntimeProfileID := uuid.New()
	hostedDeploymentID := uuid.New()
	hostedSnapshotID := uuid.New()

	if _, err := db.Exec(ctx, `
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, hostedRuntimeProfileID, fixture.organizationID, fixture.workspaceID, "Hosted Runtime", "hosted-runtime", "hosted_external", "disabled", 1, 0, 30, 120, []byte(`{"transport":"http"}`)); err != nil {
		t.Fatalf("insert hosted runtime profile returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
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
			endpoint_url,
			deployment_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, hostedDeploymentID, fixture.organizationID, fixture.workspaceID, fixture.agentBuildID, fixture.agentBuildVersionID, hostedRuntimeProfileID, fixture.providerAccountID, fixture.modelAliasID, "Hosted Support Agent", "hosted-support-agent", "hosted_external", "https://example.com/agent", []byte(`{"mode":"black_box"}`)); err != nil {
		t.Fatalf("insert hosted deployment returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
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
			endpoint_url,
			snapshot_hash,
			snapshot_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, hostedSnapshotID, fixture.organizationID, fixture.workspaceID, fixture.agentBuildID, hostedDeploymentID, fixture.agentBuildVersionID, hostedRuntimeProfileID, fixture.providerAccountID, fixture.modelAliasID, "hosted_external", "https://example.com/agent", "hosted-snapshot-hash", []byte(`{"mode":"black_box"}`)); err != nil {
		t.Fatalf("insert hosted deployment snapshot returned error: %v", err)
	}

	hostedRunAgent, err := queries.CreateRunAgent(ctx, repositorysqlc.CreateRunAgentParams{
		OrganizationID:            fixture.organizationID,
		WorkspaceID:               fixture.workspaceID,
		RunID:                     fixture.runID,
		AgentDeploymentID:         hostedDeploymentID,
		AgentDeploymentSnapshotID: hostedSnapshotID,
		LaneIndex:                 2,
		Label:                     "hosted-lane",
		Status:                    string(domain.RunAgentStatusQueued),
		QueuedAt:                  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateRunAgent for hosted lane returned error: %v", err)
	}

	executionContext, err := repo.GetRunAgentExecutionContextByID(ctx, hostedRunAgent.ID)
	if err != nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned error: %v", err)
	}

	if executionContext.Deployment.DeploymentType != "hosted_external" {
		t.Fatalf("deployment type = %q, want hosted_external", executionContext.Deployment.DeploymentType)
	}
	if executionContext.Deployment.EndpointURL == nil || *executionContext.Deployment.EndpointURL != "https://example.com/agent" {
		t.Fatalf("endpoint url = %v, want hosted endpoint", executionContext.Deployment.EndpointURL)
	}
	if executionContext.Deployment.RuntimeProfile.ExecutionTarget != "hosted_external" {
		t.Fatalf("runtime execution target = %q, want hosted_external", executionContext.Deployment.RuntimeProfile.ExecutionTarget)
	}
	if executionContext.Deployment.ProviderAccount == nil || executionContext.Deployment.ProviderAccount.ID != fixture.providerAccountID {
		t.Fatalf("provider account = %#v, want %s", executionContext.Deployment.ProviderAccount, fixture.providerAccountID)
	}
	if executionContext.Deployment.ProviderAccount.ProviderKey != "openai" {
		t.Fatalf("provider key = %q, want openai", executionContext.Deployment.ProviderAccount.ProviderKey)
	}
	if executionContext.Deployment.ModelAlias == nil || executionContext.Deployment.ModelAlias.ID != fixture.modelAliasID {
		t.Fatalf("model alias = %#v, want %s", executionContext.Deployment.ModelAlias, fixture.modelAliasID)
	}
	if executionContext.Deployment.ModelAlias.ModelCatalogEntry.ProviderModelID != "gpt-4.1" {
		t.Fatalf("provider model id = %q, want gpt-4.1", executionContext.Deployment.ModelAlias.ModelCatalogEntry.ProviderModelID)
	}
}

func TestRepositoryGetRunAgentExecutionContextByIDRejectsInconsistentFrozenRefs(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		UPDATE runtime_profiles
		SET execution_target = 'hosted_external'
		WHERE id = $1
	`, fixture.runtimeProfileID); err != nil {
		t.Fatalf("update runtime execution_target returned error: %v", err)
	}

	_, err := repo.GetRunAgentExecutionContextByID(ctx, fixture.primaryRunAgentID)
	if err == nil {
		t.Fatalf("GetRunAgentExecutionContextByID returned nil error for inconsistent frozen refs")
	}
	if !errors.Is(err, repository.ErrFrozenExecutionContext) {
		t.Fatalf("GetRunAgentExecutionContextByID error = %v, want ErrFrozenExecutionContext", err)
	}
}

func TestRepositorySetRunTemporalIDs(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	run, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("SetRunTemporalIDs returned error: %v", err)
	}

	if run.TemporalWorkflowID == nil || *run.TemporalWorkflowID != "run-workflow-123" {
		t.Fatalf("temporal workflow id = %v, want %q", run.TemporalWorkflowID, "run-workflow-123")
	}
	if run.TemporalRunID == nil || *run.TemporalRunID != "temporal-run-456" {
		t.Fatalf("temporal run id = %v, want %q", run.TemporalRunID, "temporal-run-456")
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after SetRunTemporalIDs returned error: %v", err)
	}
	if persisted.TemporalWorkflowID == nil || *persisted.TemporalWorkflowID != "run-workflow-123" {
		t.Fatalf("persisted temporal workflow id = %v, want %q", persisted.TemporalWorkflowID, "run-workflow-123")
	}
	if persisted.TemporalRunID == nil || *persisted.TemporalRunID != "temporal-run-456" {
		t.Fatalf("persisted temporal run id = %v, want %q", persisted.TemporalRunID, "temporal-run-456")
	}
}

func TestRepositorySetRunTemporalIDsIsIdempotentForSameValues(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	firstRun, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("initial SetRunTemporalIDs returned error: %v", err)
	}

	secondRun, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("idempotent SetRunTemporalIDs returned error: %v", err)
	}

	if firstRun.UpdatedAt != secondRun.UpdatedAt {
		t.Fatalf("updated_at changed on idempotent temporal id set: first=%s second=%s", firstRun.UpdatedAt, secondRun.UpdatedAt)
	}
}

func TestRepositorySetRunTemporalIDsRejectsReassignment(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	_, err := repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-123",
		TemporalRunID:      "temporal-run-456",
	})
	if err != nil {
		t.Fatalf("initial SetRunTemporalIDs returned error: %v", err)
	}

	_, err = repo.SetRunTemporalIDs(ctx, repository.SetRunTemporalIDsParams{
		RunID:              fixture.runID,
		TemporalWorkflowID: "run-workflow-999",
		TemporalRunID:      "temporal-run-999",
	})
	if err == nil {
		t.Fatalf("SetRunTemporalIDs returned nil error for temporal id reassignment")
	}
	if !errors.Is(err, repository.ErrTemporalIDConflict) {
		t.Fatalf("SetRunTemporalIDs error = %v, want ErrTemporalIDConflict", err)
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after temporal id conflict returned error: %v", err)
	}
	if persisted.TemporalWorkflowID == nil || *persisted.TemporalWorkflowID != "run-workflow-123" {
		t.Fatalf("persisted temporal workflow id = %v, want %q", persisted.TemporalWorkflowID, "run-workflow-123")
	}
	if persisted.TemporalRunID == nil || *persisted.TemporalRunID != "temporal-run-456" {
		t.Fatalf("persisted temporal run id = %v, want %q", persisted.TemporalRunID, "temporal-run-456")
	}
}

func TestRepositoryTransitionRunStatusWritesCurrentStateAndHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	queuedReason := "queued by workflow"
	queuedRun, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:           fixture.runID,
		ToStatus:        domain.RunStatusQueued,
		Reason:          &queuedReason,
		ChangedByUserID: &fixture.userID,
	})
	if err != nil {
		t.Fatalf("TransitionRunStatus to queued returned error: %v", err)
	}
	if queuedRun.Status != domain.RunStatusQueued {
		t.Fatalf("queued run status = %s, want %s", queuedRun.Status, domain.RunStatusQueued)
	}
	if queuedRun.QueuedAt == nil {
		t.Fatalf("queued run queued_at was not set")
	}

	provisioningRun, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:    fixture.runID,
		ToStatus: domain.RunStatusProvisioning,
	})
	if err != nil {
		t.Fatalf("TransitionRunStatus to provisioning returned error: %v", err)
	}
	if provisioningRun.Status != domain.RunStatusProvisioning {
		t.Fatalf("provisioning run status = %s, want %s", provisioningRun.Status, domain.RunStatusProvisioning)
	}
	if provisioningRun.StartedAt == nil {
		t.Fatalf("provisioning run started_at was not set")
	}

	historyRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{RunID: fixture.runID})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(historyRows) != 2 {
		t.Fatalf("run status history count = %d, want 2", len(historyRows))
	}
	if historyRows[0].FromStatus == nil || *historyRows[0].FromStatus != string(domain.RunStatusDraft) {
		t.Fatalf("first history from_status = %v, want %q", historyRows[0].FromStatus, domain.RunStatusDraft)
	}
	if historyRows[0].ToStatus != string(domain.RunStatusQueued) {
		t.Fatalf("first history to_status = %q, want %q", historyRows[0].ToStatus, domain.RunStatusQueued)
	}
	if historyRows[1].FromStatus == nil || *historyRows[1].FromStatus != string(domain.RunStatusQueued) {
		t.Fatalf("second history from_status = %v, want %q", historyRows[1].FromStatus, domain.RunStatusQueued)
	}
	if historyRows[1].ToStatus != string(domain.RunStatusProvisioning) {
		t.Fatalf("second history to_status = %q, want %q", historyRows[1].ToStatus, domain.RunStatusProvisioning)
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after transitions returned error: %v", err)
	}
	if persisted.Status != domain.RunStatusProvisioning {
		t.Fatalf("persisted run status = %s, want %s", persisted.Status, domain.RunStatusProvisioning)
	}
}

func TestRepositoryCreateQueuedRunWritesRunRunAgentsAndInitialHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	result, err := repo.CreateQueuedRun(ctx, repository.CreateQueuedRunParams{
		OrganizationID:         fixture.organizationID,
		WorkspaceID:            fixture.workspaceID,
		ChallengePackVersionID: fixture.challengePackVersionID,
		CreatedByUserID:        &fixture.userID,
		Name:                   "Created From API",
		ExecutionMode:          "single_agent",
		ExecutionPlan:          []byte(`{"participants":[{"lane_index":0}]}`),
		RunAgents: []repository.CreateQueuedRunAgentParams{
			{
				AgentDeploymentID:         fixture.agentDeploymentID,
				AgentDeploymentSnapshotID: fixture.agentDeploymentSnapshotID,
				LaneIndex:                 0,
				Label:                     "Support Agent Deployment",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateQueuedRun returned error: %v", err)
	}

	if result.Run.Status != domain.RunStatusQueued {
		t.Fatalf("run status = %s, want %s", result.Run.Status, domain.RunStatusQueued)
	}
	if result.Run.QueuedAt == nil {
		t.Fatalf("queued_at was not set")
	}
	if len(result.RunAgents) != 1 {
		t.Fatalf("run agent count = %d, want 1", len(result.RunAgents))
	}
	if result.RunAgents[0].Status != domain.RunAgentStatusQueued {
		t.Fatalf("run agent status = %s, want %s", result.RunAgents[0].Status, domain.RunAgentStatusQueued)
	}

	runHistoryRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{
		RunID: result.Run.ID,
	})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(runHistoryRows) != 1 {
		t.Fatalf("run history count = %d, want 1", len(runHistoryRows))
	}
	if runHistoryRows[0].FromStatus != nil {
		t.Fatalf("run history from_status = %v, want nil", runHistoryRows[0].FromStatus)
	}
	if runHistoryRows[0].ToStatus != string(domain.RunStatusQueued) {
		t.Fatalf("run history to_status = %q, want %q", runHistoryRows[0].ToStatus, domain.RunStatusQueued)
	}

	runAgentHistoryRows, err := queries.ListRunAgentStatusHistoryByRunAgentID(ctx, repositorysqlc.ListRunAgentStatusHistoryByRunAgentIDParams{
		RunAgentID: result.RunAgents[0].ID,
	})
	if err != nil {
		t.Fatalf("ListRunAgentStatusHistoryByRunAgentID returned error: %v", err)
	}
	if len(runAgentHistoryRows) != 1 {
		t.Fatalf("run-agent history count = %d, want 1", len(runAgentHistoryRows))
	}
	if runAgentHistoryRows[0].FromStatus != nil {
		t.Fatalf("run-agent history from_status = %v, want nil", runAgentHistoryRows[0].FromStatus)
	}
	if runAgentHistoryRows[0].ToStatus != string(domain.RunAgentStatusQueued) {
		t.Fatalf("run-agent history to_status = %q, want %q", runAgentHistoryRows[0].ToStatus, domain.RunAgentStatusQueued)
	}
}

func TestRepositoryGetRunAgentReplayByRunAgentID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	replayID := uuid.New()
	artifactID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO artifacts (
			id, organization_id, workspace_id, run_id, run_agent_id, artifact_type, storage_bucket, storage_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, artifactID, fixture.organizationID, fixture.workspaceID, fixture.runID, fixture.primaryRunAgentID, "replay", "bucket", "replays/one.json"); err != nil {
		t.Fatalf("insert replay artifact returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_replays (
			id, run_agent_id, artifact_id, summary, latest_sequence_number, event_count
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, replayID, fixture.primaryRunAgentID, artifactID, []byte(`{"headline":"ready"}`), int64(7), int64(7)); err != nil {
		t.Fatalf("insert run-agent replay returned error: %v", err)
	}

	replay, err := repo.GetRunAgentReplayByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentReplayByRunAgentID returned error: %v", err)
	}
	if replay.ID != replayID {
		t.Fatalf("replay id = %s, want %s", replay.ID, replayID)
	}
	if replay.ArtifactID == nil || *replay.ArtifactID != artifactID {
		t.Fatalf("artifact_id = %v, want %s", replay.ArtifactID, artifactID)
	}
	if replay.LatestSequenceNumber == nil || *replay.LatestSequenceNumber != 7 {
		t.Fatalf("latest_sequence_number = %v, want 7", replay.LatestSequenceNumber)
	}
}

func TestRepositoryRecordRunEventAssignsSequenceAndSupportsReadAfterWrite(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	occurredAt := time.Date(2026, 3, 16, 8, 30, 0, 0, time.UTC)

	event, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:step-start:1",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    occurredAt,
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent returned error: %v", err)
	}

	if event.SequenceNumber != 1 {
		t.Fatalf("sequence number = %d, want 1", event.SequenceNumber)
	}
	if event.EventType != runevents.EventTypeSystemStepStarted {
		t.Fatalf("event type = %q, want %q", event.EventType, runevents.EventTypeSystemStepStarted)
	}
	if event.Source != runevents.SourceNativeEngine {
		t.Fatalf("source = %q, want %q", event.Source, runevents.SourceNativeEngine)
	}
	if !jsonEqual(event.Payload, []byte(`{"step_index":1}`)) {
		t.Fatalf("payload = %s, want step payload", event.Payload)
	}

	events, err := repo.ListRunEventsByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunEventsByRunAgentID returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].SequenceNumber != 1 {
		t.Fatalf("listed sequence number = %d, want 1", events[0].SequenceNumber)
	}
	if !events[0].OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", events[0].OccurredAt, occurredAt)
	}
}

func TestRepositoryRecordRunEventMaintainsSequencePerRunAgent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	firstEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:first",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 8, 30, 0, 0, time.UTC),
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent first returned error: %v", err)
	}

	secondEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:second",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeModelCallStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 8, 30, 1, 0, time.UTC),
			Payload:       []byte(`{"provider_key":"openai"}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent second returned error: %v", err)
	}

	otherRunAgentEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:other-agent",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.secondaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 8, 31, 0, 0, time.UTC),
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent other-agent returned error: %v", err)
	}

	if firstEvent.SequenceNumber != 1 {
		t.Fatalf("first sequence number = %d, want 1", firstEvent.SequenceNumber)
	}
	if secondEvent.SequenceNumber != 2 {
		t.Fatalf("second sequence number = %d, want 2", secondEvent.SequenceNumber)
	}
	if otherRunAgentEvent.SequenceNumber != 1 {
		t.Fatalf("other run-agent sequence number = %d, want 1", otherRunAgentEvent.SequenceNumber)
	}

	events, err := repo.ListRunEventsByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunEventsByRunAgentID returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("primary run-agent event count = %d, want 2", len(events))
	}
	if events[0].SequenceNumber != 1 || events[1].SequenceNumber != 2 {
		t.Fatalf("primary run-agent sequence numbers = [%d %d], want [1 2]", events[0].SequenceNumber, events[1].SequenceNumber)
	}
}

func TestRepositoryRecordRunEventDoesNotDeduplicateSameEventID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	event := runevents.Envelope{
		EventID:       "native:duplicate",
		SchemaVersion: runevents.SchemaVersionV1,
		RunID:         fixture.runID,
		RunAgentID:    fixture.primaryRunAgentID,
		EventType:     runevents.EventTypeToolCallCompleted,
		Source:        runevents.SourceNativeEngine,
		OccurredAt:    time.Date(2026, 3, 16, 8, 32, 0, 0, time.UTC),
		Payload:       []byte(`{"tool_name":"submit"}`),
	}

	first, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{Event: event})
	if err != nil {
		t.Fatalf("RecordRunEvent first returned error: %v", err)
	}
	second, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{Event: event})
	if err != nil {
		t.Fatalf("RecordRunEvent second returned error: %v", err)
	}

	if first.SequenceNumber != 1 {
		t.Fatalf("first sequence number = %d, want 1", first.SequenceNumber)
	}
	if second.SequenceNumber != 2 {
		t.Fatalf("second sequence number = %d, want 2", second.SequenceNumber)
	}
}

func TestRepositoryBuildRunAgentReplayMaterializesCompletedNativeRun(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:run-start", runevents.EventTypeSystemRunStarted, time.Date(2026, 3, 16, 9, 10, 0, 0, time.UTC), `{"deployment_type":"native","execution_target":"native"}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:step-start", runevents.EventTypeSystemStepStarted, time.Date(2026, 3, 16, 9, 10, 1, 0, time.UTC), `{"step_index":1}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:model-start", runevents.EventTypeModelCallStarted, time.Date(2026, 3, 16, 9, 10, 2, 0, time.UTC), `{"provider_key":"openai","model":"gpt-4.1"}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:model-complete", runevents.EventTypeModelCallCompleted, time.Date(2026, 3, 16, 9, 10, 3, 0, time.UTC), `{"provider_key":"openai","provider_model_id":"gpt-4.1","output_text":"hello"}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:tool-complete", runevents.EventTypeToolCallCompleted, time.Date(2026, 3, 16, 9, 10, 4, 0, time.UTC), `{"tool_name":"submit","result":{"content":"done","is_error":false}}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:step-complete", runevents.EventTypeSystemStepCompleted, time.Date(2026, 3, 16, 9, 10, 5, 0, time.UTC), `{"step_index":1}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:run-complete", runevents.EventTypeSystemRunCompleted, time.Date(2026, 3, 16, 9, 10, 6, 0, time.UTC), `{"final_output":"done","step_count":1,"tool_call_count":1,"total_tokens":12}`)

	replay, err := repo.BuildRunAgentReplay(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("BuildRunAgentReplay returned error: %v", err)
	}
	if replay.LatestSequenceNumber == nil || *replay.LatestSequenceNumber != 7 {
		t.Fatalf("latest_sequence_number = %v, want 7", replay.LatestSequenceNumber)
	}
	if replay.EventCount != 7 {
		t.Fatalf("event_count = %d, want 7", replay.EventCount)
	}

	summary := decodeReplaySummary(t, replay.Summary)
	if summary["status"] != "completed" {
		t.Fatalf("summary status = %#v, want completed", summary["status"])
	}

	counts := summary["counts"].(map[string]any)
	if counts["events"] != float64(7) {
		t.Fatalf("counts.events = %#v, want 7", counts["events"])
	}
	if counts["model_calls"] != float64(1) {
		t.Fatalf("counts.model_calls = %#v, want 1", counts["model_calls"])
	}
	if counts["tool_calls"] != float64(1) {
		t.Fatalf("counts.tool_calls = %#v, want 1", counts["tool_calls"])
	}

	terminalState := summary["terminal_state"].(map[string]any)
	if terminalState["event_type"] != string(runevents.EventTypeSystemRunCompleted) {
		t.Fatalf("terminal_state.event_type = %#v, want %q", terminalState["event_type"], runevents.EventTypeSystemRunCompleted)
	}

	steps := summary["steps"].([]any)
	if len(steps) != 4 {
		t.Fatalf("step count = %d, want 4", len(steps))
	}
	if steps[0].(map[string]any)["type"] != "run" {
		t.Fatalf("first step type = %#v, want run", steps[0].(map[string]any)["type"])
	}
	if steps[1].(map[string]any)["type"] != "agent_step" {
		t.Fatalf("second step type = %#v, want agent_step", steps[1].(map[string]any)["type"])
	}
	if steps[1].(map[string]any)["status"] != "completed" {
		t.Fatalf("second step status = %#v, want completed", steps[1].(map[string]any)["status"])
	}
	if steps[2].(map[string]any)["type"] != "model_call" {
		t.Fatalf("third step type = %#v, want model_call", steps[2].(map[string]any)["type"])
	}
	if steps[2].(map[string]any)["status"] != "completed" {
		t.Fatalf("third step status = %#v, want completed", steps[2].(map[string]any)["status"])
	}
	if steps[3].(map[string]any)["type"] != "tool_call" {
		t.Fatalf("fourth step type = %#v, want tool_call", steps[3].(map[string]any)["type"])
	}
	if steps[3].(map[string]any)["status"] != "completed" {
		t.Fatalf("fourth step status = %#v, want completed", steps[3].(map[string]any)["status"])
	}
}

func TestRepositoryBuildRunAgentReplayIsInspectableForFailureAndRerunnable(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:run-start", runevents.EventTypeSystemRunStarted, time.Date(2026, 3, 16, 9, 20, 0, 0, time.UTC), `{"deployment_type":"native"}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:step-start", runevents.EventTypeSystemStepStarted, time.Date(2026, 3, 16, 9, 20, 1, 0, time.UTC), `{"step_index":2}`)

	firstReplay, err := repo.BuildRunAgentReplay(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("first BuildRunAgentReplay returned error: %v", err)
	}
	if firstReplay.EventCount != 2 {
		t.Fatalf("first event_count = %d, want 2", firstReplay.EventCount)
	}

	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:run-failed", runevents.EventTypeSystemRunFailed, time.Date(2026, 3, 16, 9, 20, 2, 0, time.UTC), `{"error":"provider timeout","step_index":2,"stop_reason":"provider_error"}`)

	secondReplay, err := repo.BuildRunAgentReplay(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("second BuildRunAgentReplay returned error: %v", err)
	}
	if secondReplay.ID != firstReplay.ID {
		t.Fatalf("replay id = %s, want %s", secondReplay.ID, firstReplay.ID)
	}
	if secondReplay.LatestSequenceNumber == nil || *secondReplay.LatestSequenceNumber != 3 {
		t.Fatalf("latest_sequence_number = %v, want 3", secondReplay.LatestSequenceNumber)
	}
	if secondReplay.EventCount != 3 {
		t.Fatalf("event_count = %d, want 3", secondReplay.EventCount)
	}

	summary := decodeReplaySummary(t, secondReplay.Summary)
	if summary["status"] != "failed" {
		t.Fatalf("summary status = %#v, want failed", summary["status"])
	}

	terminalState := summary["terminal_state"].(map[string]any)
	if terminalState["event_type"] != string(runevents.EventTypeSystemRunFailed) {
		t.Fatalf("terminal_state.event_type = %#v, want %q", terminalState["event_type"], runevents.EventTypeSystemRunFailed)
	}
	if terminalState["error_message"] != "provider timeout" {
		t.Fatalf("terminal_state.error_message = %#v, want provider timeout", terminalState["error_message"])
	}

	steps := summary["steps"].([]any)
	if len(steps) != 2 {
		t.Fatalf("step count = %d, want 2", len(steps))
	}
	agentStep := steps[1].(map[string]any)
	if agentStep["status"] != "running" {
		t.Fatalf("agent step status = %#v, want running", agentStep["status"])
	}
}

func TestRepositoryBuildRunAgentReplayKeepsIncompleteRunStatusRunning(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:run-start", runevents.EventTypeSystemRunStarted, time.Date(2026, 3, 16, 9, 30, 0, 0, time.UTC), `{"deployment_type":"native"}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:step-start", runevents.EventTypeSystemStepStarted, time.Date(2026, 3, 16, 9, 30, 1, 0, time.UTC), `{"step_index":1}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:model-start", runevents.EventTypeModelCallStarted, time.Date(2026, 3, 16, 9, 30, 2, 0, time.UTC), `{"provider_key":"openai","model":"gpt-4.1"}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:model-complete", runevents.EventTypeModelCallCompleted, time.Date(2026, 3, 16, 9, 30, 3, 0, time.UTC), `{"provider_key":"openai","provider_model_id":"gpt-4.1"}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "native:tool-complete", runevents.EventTypeToolCallCompleted, time.Date(2026, 3, 16, 9, 30, 4, 0, time.UTC), `{"tool_name":"submit"}`)

	replay, err := repo.BuildRunAgentReplay(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("BuildRunAgentReplay returned error: %v", err)
	}

	summary := decodeReplaySummary(t, replay.Summary)
	if summary["status"] != "running" {
		t.Fatalf("summary status = %#v, want running", summary["status"])
	}
	if summary["headline"] != "Tool call: submit" {
		t.Fatalf("summary headline = %#v, want tool headline", summary["headline"])
	}
}

func TestRepositoryHostedAndNativeEventsCoexistInCanonicalReadModel(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	hostedSummary, err := json.Marshal(map[string]any{
		"status": "completed",
	})
	if err != nil {
		t.Fatalf("marshal hosted summary: %v", err)
	}

	hostedReplay, err := repo.RecordHostedRunEvent(ctx, repository.RecordHostedRunEventParams{
		Event: runevents.Envelope{
			EventID:       "hosted:1",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.secondaryRunAgentID,
			EventType:     runevents.EventTypeSystemRunCompleted,
			Source:        runevents.SourceHostedExternal,
			OccurredAt:    time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC),
			Payload:       []byte(`{"raw_event_type":"run_finished","external_run_id":"ext-123","output":{"answer":"done"}}`),
			Summary: runevents.SummaryMetadata{
				Status:         "completed",
				ExternalRunID:  "ext-123",
				EvidenceLevel:  runevents.EvidenceLevelHostedBlackBox,
				IdempotencyKey: "hosted:1",
			},
		},
		Summary: hostedSummary,
	})
	if err != nil {
		t.Fatalf("RecordHostedRunEvent returned error: %v", err)
	}
	if hostedReplay.LatestSequenceNumber == nil || *hostedReplay.LatestSequenceNumber != 1 {
		t.Fatalf("hosted replay latest_sequence_number = %v, want 1", hostedReplay.LatestSequenceNumber)
	}

	nativeEvent, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       "native:1",
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         fixture.runID,
			RunAgentID:    fixture.primaryRunAgentID,
			EventType:     runevents.EventTypeSystemStepStarted,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC),
			Payload:       []byte(`{"step_index":1}`),
		},
	})
	if err != nil {
		t.Fatalf("RecordRunEvent returned error: %v", err)
	}
	if nativeEvent.SequenceNumber != 1 {
		t.Fatalf("native event sequence number = %d, want 1", nativeEvent.SequenceNumber)
	}

	hostedEvents, err := repo.ListRunEventsByRunAgentID(ctx, fixture.secondaryRunAgentID)
	if err != nil {
		t.Fatalf("ListRunEventsByRunAgentID hosted returned error: %v", err)
	}
	if len(hostedEvents) != 1 {
		t.Fatalf("hosted event count = %d, want 1", len(hostedEvents))
	}
	if hostedEvents[0].EventType != runevents.EventTypeSystemRunCompleted {
		t.Fatalf("hosted event type = %q, want %q", hostedEvents[0].EventType, runevents.EventTypeSystemRunCompleted)
	}
	if hostedEvents[0].Source != runevents.SourceHostedExternal {
		t.Fatalf("hosted event source = %q, want %q", hostedEvents[0].Source, runevents.SourceHostedExternal)
	}

	hostedReplayRead, err := repo.GetRunAgentReplayByRunAgentID(ctx, fixture.secondaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentReplayByRunAgentID hosted returned error: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(hostedReplayRead.Summary, &summary); err != nil {
		t.Fatalf("unmarshal hosted replay summary: %v", err)
	}
	if summary["status"] != "completed" {
		t.Fatalf("summary status = %#v, want completed", summary["status"])
	}
	if summary["headline"] != "Run completed" {
		t.Fatalf("summary headline = %#v, want Run completed", summary["headline"])
	}
	steps := summary["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("hosted replay step count = %d, want 1", len(steps))
	}
	if steps[0].(map[string]any)["type"] != "run" {
		t.Fatalf("hosted replay step type = %#v, want run", steps[0].(map[string]any)["type"])
	}
}

func TestRepositoryGetRunAgentScorecardByRunAgentID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := uuid.New()
	scorecardID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO evaluation_specs (
			id, challenge_pack_version_id, name, version_number, judge_mode, definition
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, evaluationSpecID, fixture.challengePackVersionID, "Core Eval", 1, "deterministic", []byte(`{}`)); err != nil {
		t.Fatalf("insert evaluation spec returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_scorecards (
			id, run_agent_id, evaluation_spec_id, overall_score, correctness_score, reliability_score, latency_score, cost_score, behavioral_score, scorecard
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, scorecardID, fixture.primaryRunAgentID, evaluationSpecID, 0.91, 0.88, 0.93, 0.74, 0.65, 0.82, []byte(`{"winner":true}`)); err != nil {
		t.Fatalf("insert run-agent scorecard returned error: %v", err)
	}

	scorecard, err := repo.GetRunAgentScorecardByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentScorecardByRunAgentID returned error: %v", err)
	}
	if scorecard.ID != scorecardID {
		t.Fatalf("scorecard id = %s, want %s", scorecard.ID, scorecardID)
	}
	if scorecard.OverallScore == nil || *scorecard.OverallScore != 0.91 {
		t.Fatalf("overall_score = %v, want 0.91", scorecard.OverallScore)
	}
	if scorecard.BehavioralScore == nil || *scorecard.BehavioralScore != 0.82 {
		t.Fatalf("behavioral_score = %v, want 0.82", scorecard.BehavioralScore)
	}
}

func TestRepositoryGetRunAgentScorecardByRunAgentIDPreservesNullScores(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := uuid.New()
	scorecardID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO evaluation_specs (
			id, challenge_pack_version_id, name, version_number, judge_mode, definition
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, evaluationSpecID, fixture.challengePackVersionID, "Partial Eval", 1, "deterministic", []byte(`{}`)); err != nil {
		t.Fatalf("insert evaluation spec returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_scorecards (
			id, run_agent_id, evaluation_spec_id, overall_score, correctness_score, reliability_score, latency_score, cost_score, behavioral_score, scorecard
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, scorecardID, fixture.primaryRunAgentID, evaluationSpecID, nil, 0.88, nil, nil, 0.65, nil, []byte(`{"partial":true}`)); err != nil {
		t.Fatalf("insert run-agent scorecard returned error: %v", err)
	}

	scorecard, err := repo.GetRunAgentScorecardByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentScorecardByRunAgentID returned error: %v", err)
	}
	if scorecard.ID != scorecardID {
		t.Fatalf("scorecard id = %s, want %s", scorecard.ID, scorecardID)
	}
	if scorecard.OverallScore != nil {
		t.Fatalf("overall_score = %v, want nil", scorecard.OverallScore)
	}
	if scorecard.CorrectnessScore == nil || *scorecard.CorrectnessScore != 0.88 {
		t.Fatalf("correctness_score = %v, want 0.88", scorecard.CorrectnessScore)
	}
	if scorecard.ReliabilityScore != nil {
		t.Fatalf("reliability_score = %v, want nil", scorecard.ReliabilityScore)
	}
	if scorecard.LatencyScore != nil {
		t.Fatalf("latency_score = %v, want nil", scorecard.LatencyScore)
	}
	if scorecard.CostScore == nil || *scorecard.CostScore != 0.65 {
		t.Fatalf("cost_score = %v, want 0.65", scorecard.CostScore)
	}
	if scorecard.BehavioralScore != nil {
		t.Fatalf("behavioral_score = %v, want nil", scorecard.BehavioralScore)
	}
}

func TestRepositoryBuildRunScorecardPersistsWinnerAndSummary(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "run-scorecard", 1)
	insertRunAgentScorecardRecord(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.84),
		Reliability: float64Ptr(0.72),
	})
	insertRunAgentScorecardRecord(t, ctx, db, fixture.secondaryRunAgentID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.84),
		Reliability: float64Ptr(0.91),
	})

	scorecard, err := repo.BuildRunScorecard(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("BuildRunScorecard returned error: %v", err)
	}
	if scorecard.RunID != fixture.runID {
		t.Fatalf("run id = %s, want %s", scorecard.RunID, fixture.runID)
	}
	if scorecard.WinningRunAgentID == nil || *scorecard.WinningRunAgentID != fixture.secondaryRunAgentID {
		t.Fatalf("winning run agent id = %v, want %s", scorecard.WinningRunAgentID, fixture.secondaryRunAgentID)
	}

	stored, err := repo.GetRunScorecardByRunID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunScorecardByRunID returned error: %v", err)
	}
	if stored.ID != scorecard.ID {
		t.Fatalf("stored id = %s, want %s", stored.ID, scorecard.ID)
	}

	document := decodeReplaySummary(t, stored.Scorecard)
	if document["winning_run_agent_id"] != fixture.secondaryRunAgentID.String() {
		t.Fatalf("winning_run_agent_id = %v, want %s", document["winning_run_agent_id"], fixture.secondaryRunAgentID)
	}
	winnerDetermination, ok := document["winner_determination"].(map[string]any)
	if !ok {
		t.Fatalf("winner_determination = %T, want map", document["winner_determination"])
	}
	if winnerDetermination["reason_code"] != "reliability_tiebreaker" {
		t.Fatalf("winner reason_code = %v, want reliability_tiebreaker", winnerDetermination["reason_code"])
	}
	dimensionDeltas, ok := document["dimension_deltas"].(map[string]any)
	if !ok {
		t.Fatalf("dimension_deltas = %T, want map", document["dimension_deltas"])
	}
	reliabilityDelta, ok := dimensionDeltas["reliability"].(map[string]any)
	if !ok {
		t.Fatalf("reliability delta = %T, want map", dimensionDeltas["reliability"])
	}
	if delta, ok := reliabilityDelta["delta"].(float64); !ok || math.Abs(delta-0.19) > 1e-9 {
		t.Fatalf("reliability delta = %v, want 0.19", reliabilityDelta["delta"])
	}
}

func TestRepositoryCreateEvaluationSpecAndReadItBack(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	created, err := repo.CreateEvaluationSpec(ctx, repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: fixture.challengePackVersionID,
		Name:                   "coding-fix-v0",
		VersionNumber:          1,
		JudgeMode:              "deterministic",
		Definition:             []byte(`{"name":"coding-fix-v0","version_number":1,"judge_mode":"deterministic","validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}`),
	})
	if err != nil {
		t.Fatalf("CreateEvaluationSpec returned error: %v", err)
	}
	if created.Name != "coding-fix-v0" {
		t.Fatalf("created.Name = %q, want coding-fix-v0", created.Name)
	}
	if created.ChallengePackVersionID != fixture.challengePackVersionID {
		t.Fatalf("created.ChallengePackVersionID = %s, want %s", created.ChallengePackVersionID, fixture.challengePackVersionID)
	}

	byID, err := repo.GetEvaluationSpecByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetEvaluationSpecByID returned error: %v", err)
	}
	if byID.ID != created.ID {
		t.Fatalf("byID.ID = %s, want %s", byID.ID, created.ID)
	}

	byVersion, err := repo.GetEvaluationSpecByChallengePackVersionAndVersion(ctx, fixture.challengePackVersionID, "coding-fix-v0", 1)
	if err != nil {
		t.Fatalf("GetEvaluationSpecByChallengePackVersionAndVersion returned error: %v", err)
	}
	if byVersion.ID != created.ID {
		t.Fatalf("byVersion.ID = %s, want %s", byVersion.ID, created.ID)
	}
}

func TestRepositoryCreateEvaluationSpecRejectsDuplicateNameAndVersion(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	params := repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: fixture.challengePackVersionID,
		Name:                   "duplicate-spec",
		VersionNumber:          1,
		JudgeMode:              "deterministic",
		Definition:             []byte(`{"name":"duplicate-spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}`),
	}

	if _, err := repo.CreateEvaluationSpec(ctx, params); err != nil {
		t.Fatalf("first CreateEvaluationSpec returned error: %v", err)
	}
	if _, err := repo.CreateEvaluationSpec(ctx, params); err == nil {
		t.Fatal("second CreateEvaluationSpec returned nil error")
	}
}

func TestRepositoryCreateEvaluationSpecRejectsInvalidDefinition(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	_, err := repo.CreateEvaluationSpec(ctx, repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: fixture.challengePackVersionID,
		Name:                   "invalid-spec",
		VersionNumber:          1,
		JudgeMode:              "deterministic",
		Definition:             []byte(`{"name":"","version_number":1,"judge_mode":"deterministic","validators":[{"key":"v1","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"scorecard":{"dimensions":["correctness"]}}`),
	})
	if err == nil {
		t.Fatal("CreateEvaluationSpec returned nil error")
	}
}

func TestRepositoryStoreRunAgentEvaluationResultsUpsertsJudgeAndMetricRows(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	specRecord, err := repo.CreateEvaluationSpec(ctx, repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: fixture.challengePackVersionID,
		Name:                   "store-results-spec",
		VersionNumber:          1,
		JudgeMode:              "deterministic",
		Definition: []byte(`{
			"name":"store-results-spec",
			"version_number":1,
			"judge_mode":"deterministic",
			"validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],
			"metrics":[{"key":"completion","type":"boolean","collector":"run_completed_successfully"}],
			"scorecard":{"dimensions":["correctness","reliability"]}
		}`),
	})
	if err != nil {
		t.Fatalf("CreateEvaluationSpec returned error: %v", err)
	}

	initialEvaluation := scoring.RunAgentEvaluation{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: specRecord.ID,
		Status:           scoring.EvaluationStatusPartial,
		ValidatorResults: []scoring.ValidatorResult{
			{
				Key:                 "exact",
				Type:                scoring.ValidatorTypeExactMatch,
				State:               scoring.OutputStateUnavailable,
				Reason:              "final output evidence is unavailable",
				RawOutput:           []byte(`{"state":"unavailable"}`),
				ChallengeIdentityID: &fixture.firstChallengeIdentityID,
			},
		},
		MetricResults: []scoring.MetricResult{
			{
				Key:                 "completion",
				Type:                scoring.MetricTypeBoolean,
				State:               scoring.OutputStateUnavailable,
				Collector:           "run_completed_successfully",
				Reason:              "terminal success evidence is unavailable",
				Metadata:            []byte(`{"state":"unavailable"}`),
				ChallengeIdentityID: &fixture.firstChallengeIdentityID,
			},
		},
	}
	if err := repo.StoreRunAgentEvaluationResults(ctx, initialEvaluation); err != nil {
		t.Fatalf("StoreRunAgentEvaluationResults returned error: %v", err)
	}

	updatedEvaluation := scoring.RunAgentEvaluation{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: specRecord.ID,
		Status:           scoring.EvaluationStatusComplete,
		DimensionScores: map[string]*float64{
			string(scoring.ScorecardDimensionBehavioral): float64Ptr(0.75),
		},
		ValidatorResults: []scoring.ValidatorResult{
			{
				Key:                 "exact",
				Type:                scoring.ValidatorTypeExactMatch,
				State:               scoring.OutputStateAvailable,
				Verdict:             "pass",
				NormalizedScore:     float64Ptr(1),
				RawOutput:           []byte(`{"state":"available","verdict":"pass"}`),
				ChallengeIdentityID: &fixture.firstChallengeIdentityID,
			},
		},
		MetricResults: []scoring.MetricResult{
			{
				Key:                 "completion",
				Type:                scoring.MetricTypeBoolean,
				State:               scoring.OutputStateAvailable,
				Collector:           "run_completed_successfully",
				BooleanValue:        boolPtr(true),
				Metadata:            []byte(`{"state":"available"}`),
				ChallengeIdentityID: &fixture.firstChallengeIdentityID,
			},
		},
		LLMJudgeResults: []scoring.LLMJudgeResult{
			{
				JudgeKey:        "safety",
				Mode:            "assertion",
				NormalizedScore: float64Ptr(1),
				Payload:         []byte(`{"pass":true}`),
				Confidence:      stringPtr("high"),
				SampleCount:     3,
				ModelCount:      1,
			},
		},
	}
	if err := repo.StoreRunAgentEvaluationResults(ctx, updatedEvaluation); err != nil {
		t.Fatalf("second StoreRunAgentEvaluationResults returned error: %v", err)
	}

	judgeResults, err := repo.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(judgeResults) != 1 {
		t.Fatalf("judge result count = %d, want 1", len(judgeResults))
	}
	if judgeResults[0].Verdict == nil || *judgeResults[0].Verdict != "pass" {
		t.Fatalf("judge verdict = %v, want pass", judgeResults[0].Verdict)
	}
	if judgeResults[0].NormalizedScore == nil || *judgeResults[0].NormalizedScore != 1 {
		t.Fatalf("judge normalized score = %v, want 1", judgeResults[0].NormalizedScore)
	}

	metricResults, err := repo.ListMetricResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListMetricResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(metricResults) != 1 {
		t.Fatalf("metric result count = %d, want 1", len(metricResults))
	}
	llmJudgeResults, err := repo.ListLLMJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListLLMJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(llmJudgeResults) != 1 {
		t.Fatalf("llm judge result count = %d, want 1", len(llmJudgeResults))
	}
	if llmJudgeResults[0].JudgeKey != "safety" {
		t.Fatalf("llm judge key = %q, want safety", llmJudgeResults[0].JudgeKey)
	}
	if metricResults[0].BooleanValue == nil || !*metricResults[0].BooleanValue {
		t.Fatalf("metric boolean value = %v, want true", metricResults[0].BooleanValue)
	}
	if metricResults[0].MetricType != string(scoring.MetricTypeBoolean) {
		t.Fatalf("metric type = %q, want boolean", metricResults[0].MetricType)
	}

	scorecard, err := repo.GetRunAgentScorecardByRunAgentID(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentScorecardByRunAgentID returned error: %v", err)
	}
	if scorecard.EvaluationSpecID != specRecord.ID {
		t.Fatalf("scorecard evaluation_spec_id = %s, want %s", scorecard.EvaluationSpecID, specRecord.ID)
	}
	if scorecard.OverallScore != nil {
		t.Fatalf("overall_score = %v, want nil", scorecard.OverallScore)
	}
	if scorecard.CorrectnessScore != nil {
		t.Fatalf("correctness_score = %v, want nil", scorecard.CorrectnessScore)
	}
	if scorecard.ReliabilityScore != nil {
		t.Fatalf("reliability_score = %v, want nil", scorecard.ReliabilityScore)
	}
	if scorecard.BehavioralScore == nil || *scorecard.BehavioralScore != 0.75 {
		t.Fatalf("behavioral_score = %v, want 0.75", scorecard.BehavioralScore)
	}
	scorecardDocument := decodeReplaySummary(t, scorecard.Scorecard)
	if scorecardDocument["status"] != "complete" {
		t.Fatalf("scorecard json status = %v, want complete", scorecardDocument["status"])
	}
}

func TestRepositoryEvaluateRunAgentUsesCanonicalEventsAndPersistsResults(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	if _, err := db.Exec(ctx, `
		DELETE FROM challenge_input_items
		WHERE challenge_input_set_id = $1
		  AND challenge_identity_id <> $2
	`, fixture.challengeInputSetID, fixture.firstChallengeIdentityID); err != nil {
		t.Fatalf("delete extra challenge input items returned error: %v", err)
	}

	specRecord, err := repo.CreateEvaluationSpec(ctx, repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: fixture.challengePackVersionID,
		Name:                   "evaluate-run-agent-spec",
		VersionNumber:          1,
		JudgeMode:              "deterministic",
		Definition: []byte(`{
			"name":"evaluate-run-agent-spec",
			"version_number":1,
			"judge_mode":"deterministic",
			"validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],
			"metrics":[{"key":"total_tokens","type":"numeric","collector":"run_total_tokens"}],
			"scorecard":{"dimensions":["correctness"]}
		}`),
	})
	if err != nil {
		t.Fatalf("CreateEvaluationSpec returned error: %v", err)
	}

	startedAt := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "event-1", runevents.EventTypeSystemRunStarted, startedAt, `{}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "event-2", runevents.EventTypeSystemRunCompleted, startedAt.Add(200*time.Millisecond), `{"final_output":"Customer one is blocked","input_tokens":11,"output_tokens":5,"total_tokens":16}`)

	evaluation, err := repo.EvaluateRunAgent(ctx, repository.EvaluateRunAgentParams{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: specRecord.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.Status != scoring.EvaluationStatusComplete {
		t.Fatalf("evaluation status = %s, want %s", evaluation.Status, scoring.EvaluationStatusComplete)
	}
	if len(evaluation.ValidatorResults) != 1 {
		t.Fatalf("validator result count = %d, want 1", len(evaluation.ValidatorResults))
	}
	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
	if len(evaluation.MetricResults) != 1 {
		t.Fatalf("metric result count = %d, want 1", len(evaluation.MetricResults))
	}
	if evaluation.MetricResults[0].NumericValue == nil || *evaluation.MetricResults[0].NumericValue != 16 {
		t.Fatalf("metric numeric value = %v, want 16", evaluation.MetricResults[0].NumericValue)
	}

	judgeResults, err := repo.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(judgeResults) != 1 {
		t.Fatalf("judge result count = %d, want 1", len(judgeResults))
	}
	if judgeResults[0].Verdict == nil || *judgeResults[0].Verdict != "pass" {
		t.Fatalf("persisted judge verdict = %v, want pass", judgeResults[0].Verdict)
	}

	metricResults, err := repo.ListMetricResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListMetricResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(metricResults) != 1 {
		t.Fatalf("metric result count = %d, want 1", len(metricResults))
	}
	if metricResults[0].NumericValue == nil || *metricResults[0].NumericValue != 16 {
		t.Fatalf("persisted metric numeric value = %v, want 16", metricResults[0].NumericValue)
	}
}

func TestRepositoryEvaluateRunAgentReturnsPartialWhenChallengeInputIsAmbiguous(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	specRecord, err := repo.CreateEvaluationSpec(ctx, repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: fixture.challengePackVersionID,
		Name:                   "ambiguous-challenge-input-spec",
		VersionNumber:          1,
		JudgeMode:              "deterministic",
		Definition: []byte(`{
			"name":"ambiguous-challenge-input-spec",
			"version_number":1,
			"judge_mode":"deterministic",
			"validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],
			"metrics":[{"key":"total_tokens","type":"numeric","collector":"run_total_tokens"}],
			"scorecard":{"dimensions":["correctness"]}
		}`),
	})
	if err != nil {
		t.Fatalf("CreateEvaluationSpec returned error: %v", err)
	}

	startedAt := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "ambiguous-event-1", runevents.EventTypeSystemRunStarted, startedAt, `{}`)
	recordRunEvent(t, ctx, repo, fixture.runID, fixture.primaryRunAgentID, "ambiguous-event-2", runevents.EventTypeSystemRunCompleted, startedAt.Add(200*time.Millisecond), `{"final_output":"Customer one is blocked","input_tokens":11,"output_tokens":5,"total_tokens":16}`)

	evaluation, err := repo.EvaluateRunAgent(ctx, repository.EvaluateRunAgentParams{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: specRecord.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.Status != scoring.EvaluationStatusPartial {
		t.Fatalf("evaluation status = %s, want %s", evaluation.Status, scoring.EvaluationStatusPartial)
	}
	if !containsString(evaluation.Warnings, "challenge input is ambiguous across multiple items") {
		t.Fatalf("warnings = %v, want ambiguity warning", evaluation.Warnings)
	}
	if len(evaluation.ValidatorResults) != 1 {
		t.Fatalf("validator result count = %d, want 1", len(evaluation.ValidatorResults))
	}
	if evaluation.ValidatorResults[0].State != scoring.OutputStateUnavailable {
		t.Fatalf("validator state = %s, want unavailable", evaluation.ValidatorResults[0].State)
	}
	if evaluation.ValidatorResults[0].Reason != "challenge input evidence is unavailable" {
		t.Fatalf("validator reason = %q, want challenge input evidence is unavailable", evaluation.ValidatorResults[0].Reason)
	}
	if len(evaluation.MetricResults) != 1 || evaluation.MetricResults[0].NumericValue == nil || *evaluation.MetricResults[0].NumericValue != 16 {
		t.Fatalf("metric results = %#v, want numeric value 16", evaluation.MetricResults)
	}

	judgeResults, err := repo.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(judgeResults) != 1 {
		t.Fatalf("judge result count = %d, want 1", len(judgeResults))
	}
	if judgeResults[0].Verdict != nil {
		t.Fatalf("persisted judge verdict = %v, want nil", judgeResults[0].Verdict)
	}
	if judgeResults[0].ChallengeIdentityID != nil {
		t.Fatalf("persisted judge challenge identity = %v, want nil", judgeResults[0].ChallengeIdentityID)
	}
	if !jsonEqual(judgeResults[0].RawOutput, []byte(`{"state":"unavailable","reason":"challenge input evidence is unavailable"}`)) {
		t.Fatalf("persisted judge raw output = %s, want unavailable reason payload", judgeResults[0].RawOutput)
	}

	metricResults, err := repo.ListMetricResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListMetricResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(metricResults) != 1 {
		t.Fatalf("metric result count = %d, want 1", len(metricResults))
	}
	if metricResults[0].NumericValue == nil || *metricResults[0].NumericValue != 16 {
		t.Fatalf("persisted metric numeric value = %v, want 16", metricResults[0].NumericValue)
	}
}

func TestRepositoryEvaluateRunAgentPersistsStructuredJSONValidatorEvidence(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	specRecord, err := repo.CreateEvaluationSpec(ctx, repository.CreateEvaluationSpecParams{
		ChallengePackVersionID: fixture.challengePackVersionID,
		Name:                   "json-validator-spec",
		VersionNumber:          1,
		JudgeMode:              "deterministic",
		Definition: []byte(`{
			"name":"json-validator-spec",
			"version_number":1,
			"judge_mode":"deterministic",
			"validators":[
				{
					"key":"schema",
					"type":"json_schema",
					"target":"final_output",
					"expected_from":"literal:{\"type\":\"object\",\"required\":[\"status\",\"score\",\"details\"],\"properties\":{\"status\":{\"type\":\"string\"},\"score\":{\"type\":\"number\"},\"details\":{\"type\":\"object\"}}}"
				},
				{
					"key":"path",
					"type":"json_path_match",
					"target":"final_output",
					"expected_from":"literal:{\"path\":\"$.details.items[0].id\",\"comparator\":\"equals\",\"value\":\"abc\"}"
				}
			],
			"scorecard":{"dimensions":["correctness"]}
		}`),
	})
	if err != nil {
		t.Fatalf("CreateEvaluationSpec returned error: %v", err)
	}

	completedAt := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	recordRunEvent(
		t,
		ctx,
		repo,
		fixture.runID,
		fixture.primaryRunAgentID,
		"json-validator-event-1",
		runevents.EventTypeSystemRunCompleted,
		completedAt,
		`{"final_output":"{\"status\":\"done\",\"score\":10.0,\"details\":{\"items\":[{\"id\":\"abc\"}]}}"}`,
	)

	evaluation, err := repo.EvaluateRunAgent(ctx, repository.EvaluateRunAgentParams{
		RunAgentID:       fixture.primaryRunAgentID,
		EvaluationSpecID: specRecord.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.Status != scoring.EvaluationStatusComplete {
		t.Fatalf("evaluation status = %s, want %s", evaluation.Status, scoring.EvaluationStatusComplete)
	}
	if len(evaluation.ValidatorResults) != 2 {
		t.Fatalf("validator result count = %d, want 2", len(evaluation.ValidatorResults))
	}
	for i, result := range evaluation.ValidatorResults {
		if result.Verdict != "pass" {
			t.Fatalf("validator[%d] verdict = %q, want pass", i, result.Verdict)
		}
	}

	judgeResults, err := repo.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, fixture.primaryRunAgentID, specRecord.ID)
	if err != nil {
		t.Fatalf("ListJudgeResultsByRunAgentAndEvaluationSpec returned error: %v", err)
	}
	if len(judgeResults) != 2 {
		t.Fatalf("judge result count = %d, want 2", len(judgeResults))
	}

	judgeByKey := make(map[string]repository.JudgeResultRecord, len(judgeResults))
	for _, result := range judgeResults {
		judgeByKey[result.JudgeKey] = result
	}

	if judgeByKey["schema"].Verdict == nil || *judgeByKey["schema"].Verdict != "pass" {
		t.Fatalf("schema judge verdict = %v, want pass", judgeByKey["schema"].Verdict)
	}
	schemaRaw := decodeJSONObject(t, judgeByKey["schema"].RawOutput)
	if schemaRaw["schema_draft"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("schema raw output draft = %#v, want %q", schemaRaw["schema_draft"], "https://json-schema.org/draft/2020-12/schema")
	}
	if schemaRaw["actual_value"] == nil || schemaRaw["expected_value"] == nil {
		t.Fatalf("schema raw output = %#v, want actual and expected values", schemaRaw)
	}

	if judgeByKey["path"].Verdict == nil || *judgeByKey["path"].Verdict != "pass" {
		t.Fatalf("path judge verdict = %v, want pass", judgeByKey["path"].Verdict)
	}
	pathRaw := decodeJSONObject(t, judgeByKey["path"].RawOutput)
	if pathRaw["path"] != "$.details.items[0].id" {
		t.Fatalf("path raw output path = %#v, want %q", pathRaw["path"], "$.details.items[0].id")
	}
	if pathRaw["comparator"] != "equals" {
		t.Fatalf("path raw output comparator = %#v, want equals", pathRaw["comparator"])
	}
	if pathRaw["actual"] != "abc" || pathRaw["expected"] != "abc" {
		t.Fatalf("path raw output = %#v, want actual and expected abc", pathRaw)
	}
}

func TestRepositoryTransitionRunAgentStatusWritesCurrentStateAndHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	readyReason := "sandbox prepared"
	readyRunAgent, err := repo.TransitionRunAgentStatus(ctx, repository.TransitionRunAgentStatusParams{
		RunAgentID: fixture.primaryRunAgentID,
		ToStatus:   domain.RunAgentStatusReady,
		Reason:     &readyReason,
	})
	if err != nil {
		t.Fatalf("TransitionRunAgentStatus to ready returned error: %v", err)
	}
	if readyRunAgent.Status != domain.RunAgentStatusReady {
		t.Fatalf("ready run-agent status = %s, want %s", readyRunAgent.Status, domain.RunAgentStatusReady)
	}
	if readyRunAgent.StartedAt != nil {
		t.Fatalf("ready run-agent started_at should still be nil")
	}

	executingRunAgent, err := repo.TransitionRunAgentStatus(ctx, repository.TransitionRunAgentStatusParams{
		RunAgentID: fixture.primaryRunAgentID,
		ToStatus:   domain.RunAgentStatusExecuting,
	})
	if err != nil {
		t.Fatalf("TransitionRunAgentStatus to executing returned error: %v", err)
	}
	if executingRunAgent.Status != domain.RunAgentStatusExecuting {
		t.Fatalf("executing run-agent status = %s, want %s", executingRunAgent.Status, domain.RunAgentStatusExecuting)
	}
	if executingRunAgent.StartedAt == nil {
		t.Fatalf("executing run-agent started_at was not set")
	}

	historyRows, err := queries.ListRunAgentStatusHistoryByRunAgentID(ctx, repositorysqlc.ListRunAgentStatusHistoryByRunAgentIDParams{
		RunAgentID: fixture.primaryRunAgentID,
	})
	if err != nil {
		t.Fatalf("ListRunAgentStatusHistoryByRunAgentID returned error: %v", err)
	}
	if len(historyRows) != 2 {
		t.Fatalf("run-agent status history count = %d, want 2", len(historyRows))
	}
	if historyRows[0].FromStatus == nil || *historyRows[0].FromStatus != string(domain.RunAgentStatusQueued) {
		t.Fatalf("first run-agent history from_status = %v, want %q", historyRows[0].FromStatus, domain.RunAgentStatusQueued)
	}
	if historyRows[0].ToStatus != string(domain.RunAgentStatusReady) {
		t.Fatalf("first run-agent history to_status = %q, want %q", historyRows[0].ToStatus, domain.RunAgentStatusReady)
	}
	if historyRows[1].FromStatus == nil || *historyRows[1].FromStatus != string(domain.RunAgentStatusReady) {
		t.Fatalf("second run-agent history from_status = %v, want %q", historyRows[1].FromStatus, domain.RunAgentStatusReady)
	}
	if historyRows[1].ToStatus != string(domain.RunAgentStatusExecuting) {
		t.Fatalf("second run-agent history to_status = %q, want %q", historyRows[1].ToStatus, domain.RunAgentStatusExecuting)
	}

	runAgents, err := repo.ListRunAgentsByRunID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("ListRunAgentsByRunID after transitions returned error: %v", err)
	}
	if runAgents[0].Status != domain.RunAgentStatusExecuting {
		t.Fatalf("persisted run-agent status = %s, want %s", runAgents[0].Status, domain.RunAgentStatusExecuting)
	}
}

func TestRepositoryTransitionRunStatusRejectsInvalidTransitionWithoutWritingHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	_, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:    fixture.runID,
		ToStatus: domain.RunStatusRunning,
	})
	if err == nil {
		t.Fatalf("TransitionRunStatus returned nil error for invalid transition")
	}
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("TransitionRunStatus error = %v, want ErrInvalidTransition", err)
	}

	historyRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{RunID: fixture.runID})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(historyRows) != 0 {
		t.Fatalf("run status history count = %d, want 0", len(historyRows))
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after invalid transition returned error: %v", err)
	}
	if persisted.Status != domain.RunStatusDraft {
		t.Fatalf("persisted run status = %s, want %s", persisted.Status, domain.RunStatusDraft)
	}
}

func TestRepositoryTransitionRunAgentStatusRejectsInvalidTransitionWithoutWritingHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	_, err := repo.TransitionRunAgentStatus(ctx, repository.TransitionRunAgentStatusParams{
		RunAgentID: fixture.primaryRunAgentID,
		ToStatus:   domain.RunAgentStatusExecuting,
	})
	if err == nil {
		t.Fatalf("TransitionRunAgentStatus returned nil error for invalid transition")
	}
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("TransitionRunAgentStatus error = %v, want ErrInvalidTransition", err)
	}

	historyRows, err := queries.ListRunAgentStatusHistoryByRunAgentID(ctx, repositorysqlc.ListRunAgentStatusHistoryByRunAgentIDParams{
		RunAgentID: fixture.primaryRunAgentID,
	})
	if err != nil {
		t.Fatalf("ListRunAgentStatusHistoryByRunAgentID returned error: %v", err)
	}
	if len(historyRows) != 0 {
		t.Fatalf("run-agent status history count = %d, want 0", len(historyRows))
	}

	runAgents, err := repo.ListRunAgentsByRunID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("ListRunAgentsByRunID after invalid transition returned error: %v", err)
	}
	if runAgents[0].Status != domain.RunAgentStatusQueued {
		t.Fatalf("persisted run-agent status = %s, want %s", runAgents[0].Status, domain.RunAgentStatusQueued)
	}
}

func TestRepositoryTransitionRunStatusRollsBackWhenHistoryInsertFails(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	queries := repositorysqlc.New(db)

	missingUserID := uuid.New()
	_, err := repo.TransitionRunStatus(ctx, repository.TransitionRunStatusParams{
		RunID:           fixture.runID,
		ToStatus:        domain.RunStatusQueued,
		ChangedByUserID: &missingUserID,
	})
	if err == nil {
		t.Fatalf("TransitionRunStatus returned nil error when history insert should fail")
	}

	persisted, err := repo.GetRunByID(ctx, fixture.runID)
	if err != nil {
		t.Fatalf("GetRunByID after rollback check returned error: %v", err)
	}
	if persisted.Status != domain.RunStatusDraft {
		t.Fatalf("persisted run status = %s, want %s after rollback", persisted.Status, domain.RunStatusDraft)
	}

	historyRows, err := queries.ListRunStatusHistoryByRunID(ctx, repositorysqlc.ListRunStatusHistoryByRunIDParams{RunID: fixture.runID})
	if err != nil {
		t.Fatalf("ListRunStatusHistoryByRunID returned error: %v", err)
	}
	if len(historyRows) != 0 {
		t.Fatalf("run status history count = %d, want 0 after rollback", len(historyRows))
	}
}

func TestRepositoryBuildRunComparisonComparableSingleParticipantRuns(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	baselineRun, baselineRunAgents := createTestRun(t, ctx, repo, fixture, 1, "baseline")
	candidateRun, candidateRunAgents := createTestRun(t, ctx, repo, fixture, 1, "candidate")
	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "compare-spec", 1)

	insertRunAgentScorecardRecord(t, ctx, db, baselineRunAgents[0].ID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.72),
		Reliability: float64Ptr(0.81),
		Latency:     float64Ptr(0.44),
		Cost:        float64Ptr(0.36),
	})
	insertRunAgentScorecardRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.84),
		Reliability: float64Ptr(0.79),
		Latency:     float64Ptr(0.40),
		Cost:        float64Ptr(0.31),
	})
	insertReplaySummaryRecord(t, ctx, db, baselineRunAgents[0].ID, replaySummaryFixture{
		Status:            "completed",
		Headline:          "Baseline completed",
		Events:            10,
		ReplaySteps:       4,
		ModelCalls:        2,
		ToolCalls:         1,
		SandboxCommands:   1,
		Outputs:           1,
		ScoringEvents:     1,
		TerminalStatus:    "completed",
		TerminalEventType: "system.run.completed",
	})
	insertReplaySummaryRecord(t, ctx, db, candidateRunAgents[0].ID, replaySummaryFixture{
		Status:            "completed",
		Headline:          "Candidate completed",
		Events:            12,
		ReplaySteps:       5,
		ModelCalls:        3,
		ToolCalls:         1,
		SandboxCommands:   1,
		Outputs:           1,
		ScoringEvents:     1,
		TerminalStatus:    "completed",
		TerminalEventType: "system.run.completed",
	})
	insertJudgeResultRecord(t, ctx, db, baselineRunAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")
	insertJudgeResultRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")

	comparison, err := repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:  baselineRun.ID,
		CandidateRunID: candidateRun.ID,
	})
	if err != nil {
		t.Fatalf("BuildRunComparison returned error: %v", err)
	}

	if comparison.Status != repository.RunComparisonStatusComparable {
		t.Fatalf("comparison status = %s, want comparable", comparison.Status)
	}
	if comparison.BaselineRunAgentID == nil || *comparison.BaselineRunAgentID != baselineRunAgents[0].ID {
		t.Fatalf("baseline selected run agent = %v, want %s", comparison.BaselineRunAgentID, baselineRunAgents[0].ID)
	}
	if comparison.CandidateRunAgentID == nil || *comparison.CandidateRunAgentID != candidateRunAgents[0].ID {
		t.Fatalf("candidate selected run agent = %v, want %s", comparison.CandidateRunAgentID, candidateRunAgents[0].ID)
	}
	if comparison.ReasonCode != nil {
		t.Fatalf("reason code = %v, want nil", comparison.ReasonCode)
	}

	summary := decodeReplaySummary(t, comparison.Summary)
	if summary["status"] != string(repository.RunComparisonStatusComparable) {
		t.Fatalf("summary status = %v, want comparable", summary["status"])
	}
	dimensionDeltas := summary["dimension_deltas"].(map[string]any)
	correctness := dimensionDeltas["correctness"].(map[string]any)
	if correctness["state"] != "available" {
		t.Fatalf("correctness state = %v, want available", correctness["state"])
	}
	delta, ok := correctness["delta"].(float64)
	if !ok {
		t.Fatalf("correctness delta type = %T, want float64", correctness["delta"])
	}
	if math.Abs(delta-0.12) > 0.0001 {
		t.Fatalf("correctness delta = %v, want ~0.12", delta)
	}
	replayDivergence := summary["replay_summary_divergence"].(map[string]any)
	if replayDivergence["state"] != "available" {
		t.Fatalf("replay summary state = %v, want available", replayDivergence["state"])
	}
}

func TestRepositoryBuildRunComparisonParticipantCountMismatch(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	candidateRun, _ := createTestRun(t, ctx, repo, fixture, 1, "candidate")

	comparison, err := repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:  fixture.runID,
		CandidateRunID: candidateRun.ID,
	})
	if err != nil {
		t.Fatalf("BuildRunComparison returned error: %v", err)
	}

	if comparison.Status != repository.RunComparisonStatusNotComparable {
		t.Fatalf("comparison status = %s, want not_comparable", comparison.Status)
	}
	if comparison.ReasonCode == nil || *comparison.ReasonCode != "participant_count_mismatch" {
		t.Fatalf("reason code = %v, want participant_count_mismatch", comparison.ReasonCode)
	}
}

func TestRepositoryBuildRunComparisonExplicitParticipantSelectionForMultiAgentRuns(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	candidateRun, candidateRunAgents := createTestRun(t, ctx, repo, fixture, 2, "candidate")
	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "compare-explicit", 1)

	insertRunAgentScorecardRecord(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.71),
		Reliability: float64Ptr(0.88),
		Latency:     float64Ptr(0.39),
		Cost:        float64Ptr(0.26),
	})
	insertRunAgentScorecardRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.76),
		Reliability: float64Ptr(0.84),
		Latency:     float64Ptr(0.33),
		Cost:        float64Ptr(0.22),
	})
	insertReplaySummaryRecord(t, ctx, db, fixture.primaryRunAgentID, replaySummaryFixture{
		Status:            "completed",
		Headline:          "Baseline lane 0",
		Events:            9,
		ReplaySteps:       4,
		ModelCalls:        2,
		ToolCalls:         1,
		Outputs:           1,
		ScoringEvents:     1,
		TerminalStatus:    "completed",
		TerminalEventType: "system.run.completed",
	})
	insertReplaySummaryRecord(t, ctx, db, candidateRunAgents[0].ID, replaySummaryFixture{
		Status:            "completed",
		Headline:          "Candidate lane 0",
		Events:            11,
		ReplaySteps:       5,
		ModelCalls:        3,
		ToolCalls:         1,
		Outputs:           1,
		ScoringEvents:     1,
		TerminalStatus:    "completed",
		TerminalEventType: "system.run.completed",
	})
	insertJudgeResultRecord(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")
	insertJudgeResultRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")

	comparison, err := repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:       fixture.runID,
		CandidateRunID:      candidateRun.ID,
		BaselineRunAgentID:  &fixture.primaryRunAgentID,
		CandidateRunAgentID: &candidateRunAgents[0].ID,
	})
	if err != nil {
		t.Fatalf("BuildRunComparison returned error: %v", err)
	}

	if comparison.Status != repository.RunComparisonStatusComparable {
		t.Fatalf("comparison status = %s, want comparable", comparison.Status)
	}
}

func TestRepositoryBuildRunComparisonEvaluationSpecMismatch(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	baselineRun, baselineRunAgents := createTestRun(t, ctx, repo, fixture, 1, "baseline")
	candidateRun, candidateRunAgents := createTestRun(t, ctx, repo, fixture, 1, "candidate")
	baselineSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "baseline-spec", 1)
	candidateSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "candidate-spec", 1)

	insertRunAgentScorecardRecord(t, ctx, db, baselineRunAgents[0].ID, baselineSpecID, scorecardFixture{Correctness: float64Ptr(0.80)})
	insertRunAgentScorecardRecord(t, ctx, db, candidateRunAgents[0].ID, candidateSpecID, scorecardFixture{Correctness: float64Ptr(0.81)})

	comparison, err := repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:  baselineRun.ID,
		CandidateRunID: candidateRun.ID,
	})
	if err != nil {
		t.Fatalf("BuildRunComparison returned error: %v", err)
	}

	if comparison.ReasonCode == nil || *comparison.ReasonCode != "evaluation_spec_mismatch" {
		t.Fatalf("reason code = %v, want evaluation_spec_mismatch", comparison.ReasonCode)
	}
}

func TestRepositoryBuildRunComparisonMissingReplayDoesNotBlock(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	baselineRun, baselineRunAgents := createTestRun(t, ctx, repo, fixture, 1, "baseline")
	candidateRun, candidateRunAgents := createTestRun(t, ctx, repo, fixture, 1, "candidate")
	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "replay-optional", 1)

	insertRunAgentScorecardRecord(t, ctx, db, baselineRunAgents[0].ID, evaluationSpecID, scorecardFixture{Correctness: float64Ptr(0.66)})
	insertRunAgentScorecardRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, scorecardFixture{Correctness: float64Ptr(0.68)})
	insertJudgeResultRecord(t, ctx, db, baselineRunAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")
	insertJudgeResultRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")
	insertReplaySummaryRecord(t, ctx, db, baselineRunAgents[0].ID, replaySummaryFixture{
		Status:            "completed",
		Headline:          "Baseline",
		Events:            6,
		ReplaySteps:       2,
		ModelCalls:        1,
		Outputs:           1,
		ScoringEvents:     1,
		TerminalStatus:    "completed",
		TerminalEventType: "system.run.completed",
	})

	comparison, err := repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:  baselineRun.ID,
		CandidateRunID: candidateRun.ID,
	})
	if err != nil {
		t.Fatalf("BuildRunComparison returned error: %v", err)
	}

	if comparison.Status != repository.RunComparisonStatusComparable {
		t.Fatalf("comparison status = %s, want comparable", comparison.Status)
	}
	summary := decodeReplaySummary(t, comparison.Summary)
	replayDivergence := summary["replay_summary_divergence"].(map[string]any)
	if replayDivergence["state"] != "unavailable" {
		t.Fatalf("replay summary state = %v, want unavailable", replayDivergence["state"])
	}
}

func TestRepositoryBuildRunComparisonMissingScorecard(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	baselineRun, baselineRunAgents := createTestRun(t, ctx, repo, fixture, 1, "baseline")
	candidateRun, _ := createTestRun(t, ctx, repo, fixture, 1, "candidate")
	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "missing-scorecard", 1)

	insertRunAgentScorecardRecord(t, ctx, db, baselineRunAgents[0].ID, evaluationSpecID, scorecardFixture{Correctness: float64Ptr(0.91)})

	comparison, err := repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:  baselineRun.ID,
		CandidateRunID: candidateRun.ID,
	})
	if err != nil {
		t.Fatalf("BuildRunComparison returned error: %v", err)
	}

	if comparison.ReasonCode == nil || *comparison.ReasonCode != "missing_scorecard" {
		t.Fatalf("reason code = %v, want missing_scorecard", comparison.ReasonCode)
	}
}

func TestRepositoryUpsertRunComparisonReleaseGateUpdatesExistingPolicyIdentity(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	comparison := createComparableRunComparison(t, ctx, db, repo, fixture)

	first, err := repo.UpsertRunComparisonReleaseGate(ctx, repository.UpsertRunComparisonReleaseGateParams{
		RunComparisonID:   comparison.ID,
		PolicyKey:         "default",
		PolicyVersion:     1,
		PolicyFingerprint: "policy-a",
		PolicySnapshot:    json.RawMessage(`{"policy_key":"default","policy_version":1}`),
		Verdict:           "pass",
		ReasonCode:        "within_thresholds",
		Summary:           "passed",
		EvidenceStatus:    "sufficient",
		EvaluationDetails: json.RawMessage(`{"triggered_conditions":[]}`),
		SourceFingerprint: "source-a",
	})
	if err != nil {
		t.Fatalf("first UpsertRunComparisonReleaseGate returned error: %v", err)
	}

	second, err := repo.UpsertRunComparisonReleaseGate(ctx, repository.UpsertRunComparisonReleaseGateParams{
		RunComparisonID:   comparison.ID,
		PolicyKey:         "default",
		PolicyVersion:     1,
		PolicyFingerprint: "policy-a",
		PolicySnapshot:    json.RawMessage(`{"policy_key":"default","policy_version":1}`),
		Verdict:           "warn",
		ReasonCode:        "threshold_warn_latency",
		Summary:           "warned",
		EvidenceStatus:    "sufficient",
		EvaluationDetails: json.RawMessage(`{"triggered_conditions":["threshold_warn_latency"]}`),
		SourceFingerprint: "source-b",
	})
	if err != nil {
		t.Fatalf("second UpsertRunComparisonReleaseGate returned error: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("release gate id changed on upsert: %s != %s", first.ID, second.ID)
	}

	gates, err := repo.ListRunComparisonReleaseGates(ctx, comparison.ID)
	if err != nil {
		t.Fatalf("ListRunComparisonReleaseGates returned error: %v", err)
	}
	if len(gates) != 1 {
		t.Fatalf("release gate count = %d, want 1", len(gates))
	}
	if gates[0].Verdict != "warn" {
		t.Fatalf("verdict = %q, want warn", gates[0].Verdict)
	}
	if gates[0].ReasonCode != "threshold_warn_latency" {
		t.Fatalf("reason code = %q, want threshold_warn_latency", gates[0].ReasonCode)
	}
}

func TestRepositoryUpsertRunComparisonReleaseGateSupportsMultiplePolicies(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	comparison := createComparableRunComparison(t, ctx, db, repo, fixture)

	_, err := repo.UpsertRunComparisonReleaseGate(ctx, repository.UpsertRunComparisonReleaseGateParams{
		RunComparisonID:   comparison.ID,
		PolicyKey:         "default",
		PolicyVersion:     1,
		PolicyFingerprint: "policy-a",
		PolicySnapshot:    json.RawMessage(`{"policy_key":"default","policy_version":1}`),
		Verdict:           "pass",
		ReasonCode:        "within_thresholds",
		Summary:           "passed",
		EvidenceStatus:    "sufficient",
		EvaluationDetails: json.RawMessage(`{"triggered_conditions":[]}`),
		SourceFingerprint: "source-a",
	})
	if err != nil {
		t.Fatalf("first UpsertRunComparisonReleaseGate returned error: %v", err)
	}

	_, err = repo.UpsertRunComparisonReleaseGate(ctx, repository.UpsertRunComparisonReleaseGateParams{
		RunComparisonID:   comparison.ID,
		PolicyKey:         "strict",
		PolicyVersion:     1,
		PolicyFingerprint: "policy-b",
		PolicySnapshot:    json.RawMessage(`{"policy_key":"strict","policy_version":1}`),
		Verdict:           "insufficient_evidence",
		ReasonCode:        "comparison_evidence_missing",
		Summary:           "insufficient",
		EvidenceStatus:    "insufficient",
		EvaluationDetails: json.RawMessage(`{"missing_fields":["replay_summary_divergence"]}`),
		SourceFingerprint: "source-b",
	})
	if err != nil {
		t.Fatalf("second UpsertRunComparisonReleaseGate returned error: %v", err)
	}

	gates, err := repo.ListRunComparisonReleaseGates(ctx, comparison.ID)
	if err != nil {
		t.Fatalf("ListRunComparisonReleaseGates returned error: %v", err)
	}
	if len(gates) != 2 {
		t.Fatalf("release gate count = %d, want 2", len(gates))
	}
}

func createComparableRunComparison(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	repo *repository.Repository,
	fixture testFixture,
) repository.RunComparison {
	t.Helper()

	baselineRun, baselineRunAgents := createTestRun(t, ctx, repo, fixture, 1, "baseline")
	candidateRun, candidateRunAgents := createTestRun(t, ctx, repo, fixture, 1, "candidate")
	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "release-gate-spec", 1)

	insertRunAgentScorecardRecord(t, ctx, db, baselineRunAgents[0].ID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.72),
		Reliability: float64Ptr(0.81),
		Latency:     float64Ptr(0.44),
		Cost:        float64Ptr(0.36),
	})
	insertRunAgentScorecardRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, scorecardFixture{
		Correctness: float64Ptr(0.74),
		Reliability: float64Ptr(0.80),
		Latency:     float64Ptr(0.45),
		Cost:        float64Ptr(0.37),
	})
	insertReplaySummaryRecord(t, ctx, db, baselineRunAgents[0].ID, replaySummaryFixture{
		Status:            "completed",
		Headline:          "Baseline completed",
		Events:            10,
		ReplaySteps:       4,
		ModelCalls:        2,
		ToolCalls:         1,
		SandboxCommands:   1,
		Outputs:           1,
		ScoringEvents:     1,
		TerminalStatus:    "completed",
		TerminalEventType: "system.run.completed",
	})
	insertReplaySummaryRecord(t, ctx, db, candidateRunAgents[0].ID, replaySummaryFixture{
		Status:            "completed",
		Headline:          "Candidate completed",
		Events:            11,
		ReplaySteps:       4,
		ModelCalls:        2,
		ToolCalls:         1,
		SandboxCommands:   1,
		Outputs:           1,
		ScoringEvents:     1,
		TerminalStatus:    "completed",
		TerminalEventType: "system.run.completed",
	})
	insertJudgeResultRecord(t, ctx, db, baselineRunAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")
	insertJudgeResultRecord(t, ctx, db, candidateRunAgents[0].ID, evaluationSpecID, fixture.firstChallengeIdentityID, "exact")

	comparison, err := repo.BuildRunComparison(ctx, repository.BuildRunComparisonParams{
		BaselineRunID:  baselineRun.ID,
		CandidateRunID: candidateRun.ID,
	})
	if err != nil {
		t.Fatalf("BuildRunComparison returned error: %v", err)
	}
	if comparison.Status != repository.RunComparisonStatusComparable {
		t.Fatalf("comparison status = %s, want comparable", comparison.Status)
	}
	return comparison
}

type testFixture struct {
	organizationID            uuid.UUID
	workspaceID               uuid.UUID
	userID                    uuid.UUID
	challengePackVersionID    uuid.UUID
	challengeInputSetID       uuid.UUID
	runtimeProfileID          uuid.UUID
	agentBuildID              uuid.UUID
	agentBuildVersionID       uuid.UUID
	agentDeploymentID         uuid.UUID
	agentDeploymentSnapshotID uuid.UUID
	firstChallengeIdentityID  uuid.UUID
	providerAccountID         uuid.UUID
	modelAliasID              uuid.UUID
	modelCatalogEntryID       uuid.UUID
	runID                     uuid.UUID
	runName                   string
	primaryRunAgentID         uuid.UUID
	secondaryRunAgentID       uuid.UUID
}

func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New returned error: %v", err)
	}
	t.Cleanup(db.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		t.Fatalf("db.Ping returned error: %v", err)
	}

	var runsTable *string
	if err := db.QueryRow(ctx, "SELECT to_regclass('public.runs')::text").Scan(&runsTable); err != nil {
		t.Fatalf("checking runs table returned error: %v", err)
	}
	if runsTable == nil || *runsTable == "" {
		t.Fatalf("runs table is missing; run `make db-up` and `make db-migrate` from the repository root first")
	}

	return db
}

func seedFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) testFixture {
	t.Helper()

	if _, err := db.Exec(ctx, "TRUNCATE TABLE challenge_packs, model_catalog_entries, organizations, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("reset fixture data returned error: %v", err)
	}

	organizationID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	challengePackID := uuid.New()
	challengePackVersionID := uuid.New()
	challengeInputSetID := uuid.New()
	firstChallengeIdentityID := uuid.New()
	secondChallengeIdentityID := uuid.New()
	firstChallengeVersionID := uuid.New()
	secondChallengeVersionID := uuid.New()
	firstChallengeInputItemID := uuid.New()
	secondChallengeInputItemID := uuid.New()
	runtimeProfileID := uuid.New()
	providerAccountID := uuid.New()
	modelCatalogEntryID := uuid.New()
	modelAliasID := uuid.New()
	agentBuildID := uuid.New()
	agentBuildVersionID := uuid.New()
	agentDeploymentID := uuid.New()
	agentDeploymentSnapshotID := uuid.New()

	if _, err := db.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, $2, $3)
	`, organizationID, "Test Org", "test-org"); err != nil {
		t.Fatalf("insert organization returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug)
		VALUES ($1, $2, $3, $4)
	`, workspaceID, organizationID, "Test Workspace", "test-workspace"); err != nil {
		t.Fatalf("insert workspace returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, workos_user_id, email, display_name)
		VALUES ($1, $2, $3, $4)
	`, userID, "workos-user-1", "owner@example.com", "Owner"); err != nil {
		t.Fatalf("insert user returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_packs (id, slug, name, family)
		VALUES ($1, $2, $3, $4)
	`, challengePackID, "benchmark-pack", "Benchmark Pack", "reasoning"); err != nil {
		t.Fatalf("insert challenge pack returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_pack_versions (
			id,
			challenge_pack_id,
			version_number,
			lifecycle_status,
			manifest_checksum,
			manifest
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, challengePackVersionID, challengePackID, 1, "runnable", "manifest-checksum", []byte(`{"tool_policy":{"allowed_tool_kinds":["file"]}}`)); err != nil {
		t.Fatalf("insert challenge pack version returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_identities (
			id,
			challenge_pack_id,
			challenge_key,
			name,
			category,
			difficulty,
			description
		)
		VALUES
			($1, $2, $3, $4, $5, $6, $7),
			($8, $2, $9, $10, $11, $12, $13)
	`, firstChallengeIdentityID, challengePackID, "first-ticket", "First Ticket", "support", "easy", "Handle the first ticket", secondChallengeIdentityID, "second-ticket", "Second Ticket", "support", "medium", "Handle the second ticket"); err != nil {
		t.Fatalf("insert challenge identities returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
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
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9),
			($10, $2, $3, $11, $12, $13, $14, $15, $16)
	`, firstChallengeVersionID, challengePackVersionID, challengePackID, firstChallengeIdentityID, 0, "Ticket One", "support", "easy", []byte(`{"instructions":"Solve the first ticket"}`), secondChallengeVersionID, secondChallengeIdentityID, 1, "Ticket Two", "support", "medium", []byte(`{"instructions":"Solve the second ticket"}`)); err != nil {
		t.Fatalf("insert challenge pack version challenges returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_input_sets (
			id,
			challenge_pack_version_id,
			input_key,
			name,
			input_checksum
		)
		VALUES ($1, $2, $3, $4, $5)
	`, challengeInputSetID, challengePackVersionID, "default-inputs", "Default Inputs", "input-checksum"); err != nil {
		t.Fatalf("insert challenge input set returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO challenge_input_items (
			id,
			challenge_input_set_id,
			challenge_pack_version_id,
			challenge_identity_id,
			item_key,
			payload
		)
		VALUES
			($1, $2, $3, $4, $5, $6),
			($7, $2, $3, $8, $9, $10)
	`, firstChallengeInputItemID, challengeInputSetID, challengePackVersionID, firstChallengeIdentityID, "prompt.txt", []byte(`{"content":"Customer one is blocked"}`), secondChallengeInputItemID, secondChallengeIdentityID, "prompt.txt", []byte(`{"content":"Customer two needs follow-up"}`)); err != nil {
		t.Fatalf("insert challenge input items returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO runtime_profiles (
			id,
			organization_id,
			workspace_id,
			name,
			slug,
			execution_target
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, runtimeProfileID, organizationID, workspaceID, "Native Runtime", "native-runtime", "native"); err != nil {
		t.Fatalf("insert runtime profile returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO provider_accounts (
			id,
			organization_id,
			workspace_id,
			provider_key,
			name,
			credential_reference,
			limits_config
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, providerAccountID, organizationID, workspaceID, "openai", "Workspace OpenAI", "secret://openai", []byte(`{"rpm":60}`)); err != nil {
		t.Fatalf("insert provider account returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO model_catalog_entries (
			id,
			provider_key,
			provider_model_id,
			display_name,
			model_family,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, modelCatalogEntryID, "openai", "gpt-4.1", "GPT-4.1", "gpt-4.1", []byte(`{"tier":"standard"}`)); err != nil {
		t.Fatalf("insert model catalog entry returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO model_aliases (
			id,
			organization_id,
			workspace_id,
			provider_account_id,
			model_catalog_entry_id,
			alias_key,
			display_name
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, modelAliasID, organizationID, workspaceID, providerAccountID, modelCatalogEntryID, "primary-model", "Primary Model"); err != nil {
		t.Fatalf("insert model alias returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO agent_builds (
			id,
			organization_id,
			workspace_id,
			name,
			slug,
			created_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, agentBuildID, organizationID, workspaceID, "Support Agent", "support-agent", userID); err != nil {
		t.Fatalf("insert agent build returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, agentBuildVersionID, agentBuildID, 1, "ready", []byte(`{"strategy":"inspect files before responding"}`), "You are a precise support benchmark agent.", []byte(`{"type":"object","properties":{"answer":{"type":"string"}}}`), []byte(`{}`), userID); err != nil {
		t.Fatalf("insert agent build version returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, agentDeploymentID, organizationID, workspaceID, agentBuildID, agentBuildVersionID, runtimeProfileID, providerAccountID, modelAliasID, "Support Agent Deployment", "support-agent-deployment", "native", []byte(`{}`)); err != nil {
		t.Fatalf("insert agent deployment returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, agentDeploymentSnapshotID, organizationID, workspaceID, agentBuildID, agentDeploymentID, agentBuildVersionID, runtimeProfileID, providerAccountID, modelAliasID, "native", "snapshot-hash-1", []byte(`{"temperature":0.1}`)); err != nil {
		t.Fatalf("insert agent deployment snapshot returned error: %v", err)
	}

	queries := repositorysqlc.New(db)
	runRow, err := queries.CreateRun(ctx, repositorysqlc.CreateRunParams{
		OrganizationID:         organizationID,
		WorkspaceID:            workspaceID,
		ChallengePackVersionID: challengePackVersionID,
		ChallengeInputSetID:    &challengeInputSetID,
		CreatedByUserID:        &userID,
		Name:                   "Regression Fixture Run",
		Status:                 string(domain.RunStatusDraft),
		ExecutionMode:          "comparison",
		ExecutionPlan:          []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	queuedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	secondaryRunAgent, err := queries.CreateRunAgent(ctx, repositorysqlc.CreateRunAgentParams{
		OrganizationID:            organizationID,
		WorkspaceID:               workspaceID,
		RunID:                     runRow.ID,
		AgentDeploymentID:         agentDeploymentID,
		AgentDeploymentSnapshotID: agentDeploymentSnapshotID,
		LaneIndex:                 1,
		Label:                     "secondary-lane",
		Status:                    string(domain.RunAgentStatusQueued),
		QueuedAt:                  queuedAt,
	})
	if err != nil {
		t.Fatalf("CreateRunAgent for secondary lane returned error: %v", err)
	}

	primaryRunAgent, err := queries.CreateRunAgent(ctx, repositorysqlc.CreateRunAgentParams{
		OrganizationID:            organizationID,
		WorkspaceID:               workspaceID,
		RunID:                     runRow.ID,
		AgentDeploymentID:         agentDeploymentID,
		AgentDeploymentSnapshotID: agentDeploymentSnapshotID,
		LaneIndex:                 0,
		Label:                     "primary-lane",
		Status:                    string(domain.RunAgentStatusQueued),
		QueuedAt:                  queuedAt,
	})
	if err != nil {
		t.Fatalf("CreateRunAgent for primary lane returned error: %v", err)
	}

	return testFixture{
		organizationID:            organizationID,
		workspaceID:               workspaceID,
		userID:                    userID,
		challengePackVersionID:    challengePackVersionID,
		challengeInputSetID:       challengeInputSetID,
		runtimeProfileID:          runtimeProfileID,
		agentBuildID:              agentBuildID,
		agentBuildVersionID:       agentBuildVersionID,
		agentDeploymentID:         agentDeploymentID,
		agentDeploymentSnapshotID: agentDeploymentSnapshotID,
		firstChallengeIdentityID:  firstChallengeIdentityID,
		providerAccountID:         providerAccountID,
		modelAliasID:              modelAliasID,
		modelCatalogEntryID:       modelCatalogEntryID,
		runID:                     runRow.ID,
		runName:                   runRow.Name,
		primaryRunAgentID:         primaryRunAgent.ID,
		secondaryRunAgentID:       secondaryRunAgent.ID,
	}
}

func recordRunEvent(
	t *testing.T,
	ctx context.Context,
	repo *repository.Repository,
	runID uuid.UUID,
	runAgentID uuid.UUID,
	eventID string,
	eventType runevents.Type,
	occurredAt time.Time,
	payload string,
) {
	t.Helper()

	if _, err := repo.RecordRunEvent(ctx, repository.RecordRunEventParams{
		Event: runevents.Envelope{
			EventID:       eventID,
			SchemaVersion: runevents.SchemaVersionV1,
			RunID:         runID,
			RunAgentID:    runAgentID,
			EventType:     eventType,
			Source:        runevents.SourceNativeEngine,
			OccurredAt:    occurredAt,
			Payload:       []byte(payload),
		},
	}); err != nil {
		t.Fatalf("RecordRunEvent(%s) returned error: %v", eventID, err)
	}
}

func decodeReplaySummary(t *testing.T, payload []byte) map[string]any {
	t.Helper()

	var summary map[string]any
	if err := json.Unmarshal(payload, &summary); err != nil {
		t.Fatalf("unmarshal replay summary: %v", err)
	}
	return summary
}

func decodeJSONObject(t *testing.T, payload []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal json object: %v", err)
	}
	return decoded
}

func jsonEqual(left []byte, right []byte) bool {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false
	}

	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false
	}

	leftCanonical, err := json.Marshal(leftValue)
	if err != nil {
		return false
	}
	rightCanonical, err := json.Marshal(rightValue)
	if err != nil {
		return false
	}

	return string(leftCanonical) == string(rightCanonical)
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func float64Ptr(value float64) *float64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

type scorecardFixture struct {
	Overall          *float64
	Correctness      *float64
	Reliability      *float64
	Latency          *float64
	Cost             *float64
	Behavioral       *float64
	CorrectnessState string
	ReliabilityState string
	LatencyState     string
	CostState        string
	BehavioralState  string
}

type replaySummaryFixture struct {
	Status            string
	Headline          string
	Events            int64
	ReplaySteps       int64
	ModelCalls        int64
	ToolCalls         int64
	SandboxCommands   int64
	Outputs           int64
	ScoringEvents     int64
	TerminalStatus    string
	TerminalEventType string
}

func createTestRun(
	t *testing.T,
	ctx context.Context,
	repo *repository.Repository,
	fixture testFixture,
	participantCount int,
	name string,
) (domain.Run, []domain.RunAgent) {
	t.Helper()

	runAgents := make([]repository.CreateQueuedRunAgentParams, 0, participantCount)
	for i := 0; i < participantCount; i++ {
		runAgents = append(runAgents, repository.CreateQueuedRunAgentParams{
			AgentDeploymentID:         fixture.agentDeploymentID,
			AgentDeploymentSnapshotID: fixture.agentDeploymentSnapshotID,
			LaneIndex:                 int32(i),
			Label:                     fmt.Sprintf("%s-lane-%d", name, i),
		})
	}

	executionMode := "comparison"
	if participantCount == 1 {
		executionMode = "single_agent"
	}

	result, err := repo.CreateQueuedRun(ctx, repository.CreateQueuedRunParams{
		OrganizationID:         fixture.organizationID,
		WorkspaceID:            fixture.workspaceID,
		ChallengePackVersionID: fixture.challengePackVersionID,
		ChallengeInputSetID:    &fixture.challengeInputSetID,
		CreatedByUserID:        &fixture.userID,
		Name:                   fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		ExecutionMode:          executionMode,
		ExecutionPlan:          []byte(`{"participants":[]}`),
		RunAgents:              runAgents,
	})
	if err != nil {
		t.Fatalf("CreateQueuedRun returned error: %v", err)
	}
	return result.Run, result.RunAgents
}

func insertEvaluationSpecRecord(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	challengePackVersionID uuid.UUID,
	name string,
	version int32,
) uuid.UUID {
	t.Helper()

	evaluationSpecID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO evaluation_specs (
			id,
			challenge_pack_version_id,
			name,
			version_number,
			judge_mode,
			definition
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, evaluationSpecID, challengePackVersionID, name, version, "deterministic", []byte(`{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"runtime_limits":{"max_duration_ms":60000,"max_cost_usd":10},"scorecard":{"dimensions":["correctness","reliability","latency","cost"],"normalization":{"latency":{"target_ms":1000},"cost":{"target_usd":1}}}}`)); err != nil {
		t.Fatalf("insert evaluation spec returned error: %v", err)
	}
	return evaluationSpecID
}

func insertRunAgentScorecardRecord(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	runAgentID uuid.UUID,
	evaluationSpecID uuid.UUID,
	fixture scorecardFixture,
) {
	t.Helper()

	scorecardID := uuid.New()
	scorecardDocument := map[string]any{
		"run_agent_id":       runAgentID,
		"evaluation_spec_id": evaluationSpecID,
		"status":             "complete",
		"dimensions": map[string]any{
			"correctness": map[string]any{"state": scorecardState(fixture.CorrectnessState, fixture.Correctness), "score": fixture.Correctness},
			"reliability": map[string]any{"state": scorecardState(fixture.ReliabilityState, fixture.Reliability), "score": fixture.Reliability},
			"latency":     map[string]any{"state": scorecardState(fixture.LatencyState, fixture.Latency), "score": fixture.Latency},
			"cost":        map[string]any{"state": scorecardState(fixture.CostState, fixture.Cost), "score": fixture.Cost},
		},
	}
	if fixture.Behavioral != nil || fixture.BehavioralState != "" {
		scorecardDocument["dimensions"].(map[string]any)["behavioral"] = map[string]any{
			"state": scorecardState(fixture.BehavioralState, fixture.Behavioral),
			"score": fixture.Behavioral,
		}
	}
	scorecardJSON, err := json.Marshal(scorecardDocument)
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_scorecards (
			id,
			run_agent_id,
			evaluation_spec_id,
			overall_score,
			correctness_score,
			reliability_score,
			latency_score,
			cost_score,
			behavioral_score,
			scorecard
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, scorecardID, runAgentID, evaluationSpecID, fixture.Overall, fixture.Correctness, fixture.Reliability, fixture.Latency, fixture.Cost, fixture.Behavioral, scorecardJSON); err != nil {
		t.Fatalf("insert run-agent scorecard returned error: %v", err)
	}
}

func insertReplaySummaryRecord(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	runAgentID uuid.UUID,
	fixture replaySummaryFixture,
) {
	t.Helper()

	replayID := uuid.New()
	summary := map[string]any{
		"schema_version": "2026-03-16",
		"status":         fixture.Status,
		"headline":       fixture.Headline,
		"counts": map[string]any{
			"events":           fixture.Events,
			"replay_steps":     fixture.ReplaySteps,
			"model_calls":      fixture.ModelCalls,
			"tool_calls":       fixture.ToolCalls,
			"sandbox_commands": fixture.SandboxCommands,
			"outputs":          fixture.Outputs,
			"scoring_events":   fixture.ScoringEvents,
		},
		"terminal_state": map[string]any{
			"status":     fixture.TerminalStatus,
			"event_type": fixture.TerminalEventType,
		},
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal replay summary: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO run_agent_replays (
			id,
			run_agent_id,
			summary,
			latest_sequence_number,
			event_count
		)
		VALUES ($1, $2, $3, $4, $5)
	`, replayID, runAgentID, summaryJSON, fixture.Events, fixture.Events); err != nil {
		t.Fatalf("insert run-agent replay returned error: %v", err)
	}
}

func insertJudgeResultRecord(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	runAgentID uuid.UUID,
	evaluationSpecID uuid.UUID,
	challengeIdentityID uuid.UUID,
	judgeKey string,
) {
	t.Helper()

	if _, err := db.Exec(ctx, `
		INSERT INTO judge_results (
			id,
			run_agent_id,
			evaluation_spec_id,
			challenge_identity_id,
			judge_key,
			verdict,
			normalized_score,
			raw_output
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, uuid.New(), runAgentID, evaluationSpecID, challengeIdentityID, judgeKey, "pass", 1.0, []byte(`{"state":"available"}`)); err != nil {
		t.Fatalf("insert judge result returned error: %v", err)
	}
}

func scorecardState(explicit string, score *float64) string {
	if explicit != "" {
		return explicit
	}
	if score == nil {
		return "unavailable"
	}
	return "available"
}
