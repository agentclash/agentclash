package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/runner"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RunAgentExecutionContext = runner.ExecutionContext
type ChallengePackVersionExecutionContext = runner.ChallengePackVersionExecutionContext
type ChallengeDefinitionExecutionContext = runner.ChallengeDefinitionExecutionContext
type ChallengeInputSetExecutionContext = runner.ChallengeInputSetExecutionContext
type ChallengeCaseExecutionContext = runner.ChallengeCaseExecutionContext
type ChallengeInputItemExecutionContext = runner.ChallengeInputItemExecutionContext
type AgentDeploymentExecutionContext = runner.AgentDeploymentExecutionContext
type AgentBuildVersionExecutionContext = runner.AgentBuildVersionExecutionContext
type RuntimeProfileExecutionContext = runner.RuntimeProfileExecutionContext
type ProviderAccountExecutionContext = runner.ProviderAccountExecutionContext

func (r *Repository) GetRunAgentExecutionContextByID(ctx context.Context, runAgentID uuid.UUID) (RunAgentExecutionContext, error) {
	row, err := r.queries.GetRunAgentExecutionContextByID(ctx, repositorysqlc.GetRunAgentExecutionContextByIDParams{ID: runAgentID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, getErr := r.GetRunAgentByID(ctx, runAgentID); getErr != nil {
				return RunAgentExecutionContext{}, getErr
			}

			return RunAgentExecutionContext{}, FrozenExecutionContextError{
				RunAgentID: runAgentID,
				Reason:     "required execution context references could not be resolved",
			}
		}

		return RunAgentExecutionContext{}, fmt.Errorf("get run-agent execution context by id: %w", err)
	}

	executionContext, err := mapRunAgentExecutionContext(row)
	if err != nil {
		return RunAgentExecutionContext{}, fmt.Errorf("map run-agent execution context: %w", err)
	}

	runAgents, err := r.ListRunAgentsByRunID(ctx, executionContext.Run.ID)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}
	executionContext.RunAgents = runAgents

	return executionContext, nil
}

func mapRunAgentExecutionContext(row repositorysqlc.GetRunAgentExecutionContextByIDRow) (RunAgentExecutionContext, error) {
	runStatus, err := domain.ParseRunStatus(row.RunStatus)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}
	officialPackMode, err := domain.ParseOfficialPackMode(row.RunOfficialPackMode)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}

	runAgentStatus, err := domain.ParseRunAgentStatus(row.RunAgentStatus)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}

	runCreatedAt, err := requiredTime("runs.created_at", row.RunCreatedAt)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}
	runUpdatedAt, err := requiredTime("runs.updated_at", row.RunUpdatedAt)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}
	runAgentCreatedAt, err := requiredTime("run_agents.created_at", row.RunAgentCreatedAt)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}
	runAgentUpdatedAt, err := requiredTime("run_agents.updated_at", row.RunAgentUpdatedAt)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}

	if row.RunChallengeInputSetID != nil && row.ChallengeInputSetID == nil {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "run challenge input set reference is missing",
		}
	}
	if row.SnapshotSourceProviderAccountID != nil && row.ProviderAccountID == nil {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "snapshot provider account reference is missing",
		}
	}
	if row.SnapshotDeploymentType == "native" && strings.TrimSpace(row.SnapshotSourceModelID) == "" {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "snapshot model id is missing",
		}
	}
	if row.SnapshotDeploymentType == "hosted_external" {
		if row.RuntimeProfileExecutionTarget != "hosted_external" {
			return RunAgentExecutionContext{}, FrozenExecutionContextError{
				RunAgentID: row.RunAgentID,
				Reason: fmt.Sprintf(
					"hosted deployment snapshot requires runtime execution_target=hosted_external, got %q",
					row.RuntimeProfileExecutionTarget,
				),
			}
		}
	} else if row.RuntimeProfileExecutionTarget == "hosted_external" {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason: fmt.Sprintf(
				"runtime execution_target=hosted_external requires hosted deployment snapshot, got %q",
				row.SnapshotDeploymentType,
			),
		}
	}
	if row.SnapshotDeploymentType == "hosted_external" && row.SnapshotEndpointUrl == nil {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "hosted deployment snapshot is missing endpoint_url",
		}
	}
	challengeDefinitionsJSON, err := decodeExecutionContextJSON("challenge_pack_challenges", row.ChallengePackChallenges)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}

	var challengeDefinitions []ChallengeDefinitionExecutionContext
	if err := json.Unmarshal(challengeDefinitionsJSON, &challengeDefinitions); err != nil {
		return RunAgentExecutionContext{}, fmt.Errorf("decode challenge pack challenges: %w", err)
	}

	challengeInputItemsJSON, err := decodeExecutionContextJSON("challenge_input_set_items", row.ChallengeInputSetItems)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}

	var challengeInputItems []ChallengeInputItemExecutionContext
	if err := json.Unmarshal(challengeInputItemsJSON, &challengeInputItems); err != nil {
		return RunAgentExecutionContext{}, fmt.Errorf("decode challenge input set items: %w", err)
	}
	challengeCases, err := decodeChallengeCases(challengeInputItems)
	if err != nil {
		return RunAgentExecutionContext{}, fmt.Errorf("decode challenge input set cases: %w", err)
	}

	agentBuildVersion, err := decodeAgentBuildVersionExecutionContext(
		row.SnapshotSourceAgentBuildVersionID,
		row.SnapshotSourceAgentSpec,
	)
	if err != nil {
		return RunAgentExecutionContext{}, err
	}

	executionContext := RunAgentExecutionContext{
		Run: domain.Run{
			ID:                     row.RunID,
			OrganizationID:         row.RunOrganizationID,
			WorkspaceID:            row.RunWorkspaceID,
			ChallengePackVersionID: derefUUID(row.RunChallengePackVersionID),
			ChallengeInputSetID:    cloneUUIDPtr(row.RunChallengeInputSetID),
			OfficialPackMode:       officialPackMode,
			CreatedByUserID:        cloneUUIDPtr(row.RunCreatedByUserID),
			Name:                   row.RunName,
			Status:                 runStatus,
			ExecutionMode:          row.RunExecutionMode,
			TemporalWorkflowID:     cloneStringPtr(row.RunTemporalWorkflowID),
			TemporalRunID:          cloneStringPtr(row.RunTemporalRunID),
			ExecutionPlan:          cloneJSON(row.RunExecutionPlan),
			QueuedAt:               optionalTime(row.RunQueuedAt),
			StartedAt:              optionalTime(row.RunStartedAt),
			FinishedAt:             optionalTime(row.RunFinishedAt),
			CancelledAt:            optionalTime(row.RunCancelledAt),
			FailedAt:               optionalTime(row.RunFailedAt),
			RaceContext:            row.RunRaceContext,
			RaceContextMinStepGap:  cloneInt32Ptr(row.RunRaceContextMinStepGap),
			CreatedAt:              runCreatedAt,
			UpdatedAt:              runUpdatedAt,
		},
		RunAgent: domain.RunAgent{
			ID:                        row.RunAgentID,
			OrganizationID:            row.RunAgentOrganizationID,
			WorkspaceID:               row.RunAgentWorkspaceID,
			RunID:                     row.RunAgentRunID,
			AgentDeploymentID:         derefUUID(row.RunAgentAgentDeploymentID),
			AgentDeploymentSnapshotID: derefUUID(row.RunAgentAgentDeploymentSnapshotID),
			LaneIndex:                 row.RunAgentLaneIndex,
			Label:                     row.RunAgentLabel,
			Status:                    runAgentStatus,
			QueuedAt:                  optionalTime(row.RunAgentQueuedAt),
			StartedAt:                 optionalTime(row.RunAgentStartedAt),
			FinishedAt:                optionalTime(row.RunAgentFinishedAt),
			FailureReason:             cloneStringPtr(row.RunAgentFailureReason),
			CreatedAt:                 runAgentCreatedAt,
			UpdatedAt:                 runAgentUpdatedAt,
		},
		ChallengePackVersion: ChallengePackVersionExecutionContext{
			ID:               row.ChallengePackVersionID,
			ChallengePackID:  row.ChallengePackID,
			VersionNumber:    row.ChallengePackVersionNumber,
			ManifestChecksum: row.ChallengePackManifestChecksum,
			Manifest:         cloneJSON(row.ChallengePackManifest),
			Challenges:       challengeDefinitions,
		},
		Deployment: AgentDeploymentExecutionContext{
			AgentDeploymentID:         row.SnapshotAgentDeploymentID,
			AgentDeploymentSnapshotID: row.SnapshotID,
			AgentBuildID:              row.SnapshotAgentBuildID,
			AgentBuildVersionID:       row.SnapshotSourceAgentBuildVersionID,
			DeploymentType:            row.SnapshotDeploymentType,
			EndpointURL:               cloneStringPtr(row.SnapshotEndpointUrl),
			SnapshotHash:              row.SnapshotHash,
			SnapshotConfig:            cloneJSON(row.SnapshotConfig),
			AgentBuildVersion:         agentBuildVersion,
			RuntimeProfile: RuntimeProfileExecutionContext{
				ID:                 row.RuntimeProfileID,
				Name:               row.RuntimeProfileName,
				Slug:               row.RuntimeProfileSlug,
				ExecutionTarget:    row.RuntimeProfileExecutionTarget,
				TraceMode:          row.RuntimeProfileTraceMode,
				MaxIterations:      row.RuntimeProfileMaxIterations,
				MaxToolCalls:       row.RuntimeProfileMaxToolCalls,
				StepTimeoutSeconds: row.RuntimeProfileStepTimeoutSeconds,
				RunTimeoutSeconds:  row.RuntimeProfileRunTimeoutSeconds,
				ProfileConfig:      cloneJSON(row.RuntimeProfileProfileConfig),
			},
			ModelID: row.SnapshotSourceModelID,
		},
	}

	if row.ChallengeInputSetID != nil {
		// Safe because challenge_input_sets.input_key/name/input_checksum are NOT NULL in the schema.
		executionContext.ChallengeInputSet = &ChallengeInputSetExecutionContext{
			ID:                     *row.ChallengeInputSetID,
			ChallengePackVersionID: *row.ChallengeInputSetChallengePackVersionID,
			InputKey:               *row.ChallengeInputSetInputKey,
			Name:                   *row.ChallengeInputSetName,
			Description:            cloneStringPtr(row.ChallengeInputSetDescription),
			InputChecksum:          *row.ChallengeInputSetInputChecksum,
			Cases:                  challengeCases,
			Items:                  challengeInputItems,
		}
	}

	if row.ProviderAccountID != nil {
		// Safe because these provider_accounts columns are NOT NULL in the schema.
		executionContext.Deployment.ProviderAccount = &ProviderAccountExecutionContext{
			ID:                  *row.ProviderAccountID,
			WorkspaceID:         cloneUUIDPtr(row.ProviderAccountWorkspaceID),
			ProviderKey:         *row.ProviderAccountProviderKey,
			Name:                *row.ProviderAccountName,
			CredentialReference: *row.ProviderAccountCredentialReference,
			LimitsConfig:        cloneJSON(row.ProviderAccountLimitsConfig),
		}
	}

	return executionContext, nil
}

func decodeChallengeCases(items []ChallengeInputItemExecutionContext) ([]ChallengeCaseExecutionContext, error) {
	cases := make([]ChallengeCaseExecutionContext, 0, len(items))
	for _, item := range items {
		caseDoc, err := decodeStoredCaseDocument(item.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode case document for %s/%s: %w", item.ChallengeKey, item.ItemKey, err)
		}
		caseKey := strings.TrimSpace(caseDoc.CaseKey)
		if caseKey == "" {
			caseKey = item.ItemKey
		}
		payload := cloneJSON(item.Payload)
		if caseDoc.SchemaVersion != 0 {
			payload, err = marshalCasePayload(caseDoc.Payload)
			if err != nil {
				return nil, fmt.Errorf("marshal canonical case payload for %s/%s: %w", item.ChallengeKey, item.ItemKey, err)
			}
		}
		cases = append(cases, ChallengeCaseExecutionContext{
			ID:                  item.ID,
			ChallengeIdentityID: item.ChallengeIdentityID,
			RegressionCaseID:    cloneUUIDPtr(item.RegressionCaseID),
			ChallengeKey:        item.ChallengeKey,
			CaseKey:             caseKey,
			ItemKey:             item.ItemKey,
			Payload:             payload,
			Inputs:              append([]challengepack.CaseInput(nil), caseDoc.Inputs...),
			Expectations:        append([]challengepack.CaseExpectation(nil), caseDoc.Expectations...),
			UserSimulator:       challengepack.CloneUserSimulatorSpec(caseDoc.UserSimulator),
			Artifacts:           append([]challengepack.ArtifactRef(nil), caseDoc.Artifacts...),
			Assets:              append([]challengepack.AssetReference(nil), caseDoc.Assets...),
		})
	}
	return cases, nil
}

func decodeStoredCaseDocument(payload json.RawMessage) (challengepack.StoredCaseDocument, error) {
	trimmed := bytesTrimSpace(payload)
	if len(trimmed) == 0 {
		return challengepack.StoredCaseDocument{}, nil
	}

	var envelope challengepack.StoredCaseDocument
	if err := json.Unmarshal(trimmed, &envelope); err == nil && envelope.SchemaVersion != 0 {
		if envelope.Payload == nil {
			envelope.Payload = map[string]any{}
		}
		return envelope, nil
	}

	var legacyPayload map[string]any
	if err := json.Unmarshal(trimmed, &legacyPayload); err != nil {
		return challengepack.StoredCaseDocument{}, err
	}
	return challengepack.StoredCaseDocument{Payload: legacyPayload}, nil
}

func bytesTrimSpace(payload json.RawMessage) []byte {
	return bytes.TrimSpace(payload)
}

func marshalCasePayload(value any) (json.RawMessage, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func decodeAgentBuildVersionExecutionContext(buildVersionID uuid.UUID, payload []byte) (AgentBuildVersionExecutionContext, error) {
	agentSpecJSON, err := decodeExecutionContextJSON("snapshot_source_agent_spec", payload)
	if err != nil {
		return AgentBuildVersionExecutionContext{}, err
	}

	var decoded struct {
		AgentKind       string          `json:"agent_kind"`
		InterfaceSpec   json.RawMessage `json:"interface_spec"`
		PolicySpec      json.RawMessage `json:"policy_spec"`
		ReasoningSpec   json.RawMessage `json:"reasoning_spec"`
		MemorySpec      json.RawMessage `json:"memory_spec"`
		WorkflowSpec    json.RawMessage `json:"workflow_spec"`
		GuardrailSpec   json.RawMessage `json:"guardrail_spec"`
		ModelSpec       json.RawMessage `json:"model_spec"`
		OutputSchema    json.RawMessage `json:"output_schema"`
		TraceContract   json.RawMessage `json:"trace_contract"`
		PublicationSpec json.RawMessage `json:"publication_spec"`
	}

	if err := json.Unmarshal(agentSpecJSON, &decoded); err != nil {
		return AgentBuildVersionExecutionContext{}, fmt.Errorf("decode snapshot source agent spec: %w", err)
	}

	return AgentBuildVersionExecutionContext{
		ID:              buildVersionID,
		AgentKind:       decoded.AgentKind,
		AgentSpec:       cloneJSON(agentSpecJSON),
		InterfaceSpec:   defaultExecutionContextJSON(decoded.InterfaceSpec),
		PolicySpec:      defaultExecutionContextJSON(decoded.PolicySpec),
		ReasoningSpec:   defaultExecutionContextJSON(decoded.ReasoningSpec),
		MemorySpec:      defaultExecutionContextJSON(decoded.MemorySpec),
		WorkflowSpec:    defaultExecutionContextJSON(decoded.WorkflowSpec),
		GuardrailSpec:   defaultExecutionContextJSON(decoded.GuardrailSpec),
		ModelSpec:       defaultExecutionContextJSON(decoded.ModelSpec),
		OutputSchema:    defaultExecutionContextJSON(decoded.OutputSchema),
		TraceContract:   defaultExecutionContextJSON(decoded.TraceContract),
		PublicationSpec: defaultExecutionContextJSON(decoded.PublicationSpec),
	}, nil
}

func defaultExecutionContextJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return cloneJSON(value)
}

func decodeExecutionContextJSON(field string, value interface{}) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return []byte("null"), nil
	case []byte:
		return append([]byte(nil), typed...), nil
	case string:
		return []byte(typed), nil
	default:
		return nil, fmt.Errorf("unsupported %s type %T", field, value)
	}
}
