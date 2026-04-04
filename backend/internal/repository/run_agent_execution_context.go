package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RunAgentExecutionContext struct {
	Run                  domain.Run
	RunAgent             domain.RunAgent
	ChallengePackVersion ChallengePackVersionExecutionContext
	ChallengeInputSet    *ChallengeInputSetExecutionContext
	Deployment           AgentDeploymentExecutionContext
}

type ChallengePackVersionExecutionContext struct {
	ID               uuid.UUID
	ChallengePackID  uuid.UUID
	VersionNumber    int32
	ManifestChecksum string
	Manifest         json.RawMessage
	Challenges       []ChallengeDefinitionExecutionContext
}

type ChallengeDefinitionExecutionContext struct {
	ID                  uuid.UUID       `json:"id"`
	ChallengeIdentityID uuid.UUID       `json:"challenge_identity_id"`
	ChallengeKey        string          `json:"challenge_key"`
	ExecutionOrder      int32           `json:"execution_order"`
	Title               string          `json:"title"`
	Category            string          `json:"category"`
	Difficulty          string          `json:"difficulty"`
	Definition          json.RawMessage `json:"definition"`
}

type ChallengeInputSetExecutionContext struct {
	ID                     uuid.UUID
	ChallengePackVersionID uuid.UUID
	InputKey               string
	Name                   string
	Description            *string
	InputChecksum          string
	Cases                  []ChallengeCaseExecutionContext
	Items                  []ChallengeInputItemExecutionContext
}

type ChallengeCaseExecutionContext struct {
	ID                  uuid.UUID                       `json:"id"`
	ChallengeIdentityID uuid.UUID                       `json:"challenge_identity_id"`
	ChallengeKey        string                          `json:"challenge_key"`
	CaseKey             string                          `json:"case_key"`
	ItemKey             string                          `json:"item_key"`
	Payload             json.RawMessage                 `json:"payload,omitempty"`
	Inputs              []challengepack.CaseInput       `json:"inputs,omitempty"`
	Expectations        []challengepack.CaseExpectation `json:"expectations,omitempty"`
	Artifacts           []challengepack.ArtifactRef     `json:"artifacts,omitempty"`
	Assets              []challengepack.AssetReference  `json:"assets,omitempty"`
}

type ChallengeInputItemExecutionContext struct {
	ID                  uuid.UUID       `json:"id"`
	ChallengeIdentityID uuid.UUID       `json:"challenge_identity_id"`
	ChallengeKey        string          `json:"challenge_key"`
	ItemKey             string          `json:"item_key"`
	Payload             json.RawMessage `json:"payload"`
}

type AgentDeploymentExecutionContext struct {
	AgentDeploymentID         uuid.UUID
	AgentDeploymentSnapshotID uuid.UUID
	AgentBuildID              uuid.UUID
	AgentBuildVersionID       uuid.UUID
	DeploymentType            string
	EndpointURL               *string
	SnapshotHash              string
	SnapshotConfig            json.RawMessage
	AgentBuildVersion         AgentBuildVersionExecutionContext
	RuntimeProfile            RuntimeProfileExecutionContext
	ProviderAccount           *ProviderAccountExecutionContext
	ModelAlias                *ModelAliasExecutionContext
}

type AgentBuildVersionExecutionContext struct {
	ID              uuid.UUID
	AgentKind       string
	AgentSpec       json.RawMessage
	InterfaceSpec   json.RawMessage
	PolicySpec      json.RawMessage
	ReasoningSpec   json.RawMessage
	MemorySpec      json.RawMessage
	WorkflowSpec    json.RawMessage
	GuardrailSpec   json.RawMessage
	ModelSpec       json.RawMessage
	OutputSchema    json.RawMessage
	TraceContract   json.RawMessage
	PublicationSpec json.RawMessage
}

type RuntimeProfileExecutionContext struct {
	ID                 uuid.UUID
	Name               string
	Slug               string
	ExecutionTarget    string
	TraceMode          string
	MaxIterations      int32
	MaxToolCalls       int32
	StepTimeoutSeconds int32
	RunTimeoutSeconds  int32
	ProfileConfig      json.RawMessage
}

type ProviderAccountExecutionContext struct {
	ID                  uuid.UUID
	WorkspaceID         *uuid.UUID
	ProviderKey         string
	Name                string
	CredentialReference string
	LimitsConfig        json.RawMessage
}

type ModelAliasExecutionContext struct {
	ID                uuid.UUID
	WorkspaceID       *uuid.UUID
	ProviderAccountID *uuid.UUID
	AliasKey          string
	DisplayName       string
	ModelCatalogEntry ModelCatalogEntryExecutionContext
}

type ModelCatalogEntryExecutionContext struct {
	ID              uuid.UUID
	ProviderKey     string
	ProviderModelID string
	DisplayName     string
	ModelFamily     string
	Modality        string
	Metadata        json.RawMessage
}

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

	return executionContext, nil
}

func mapRunAgentExecutionContext(row repositorysqlc.GetRunAgentExecutionContextByIDRow) (RunAgentExecutionContext, error) {
	runStatus, err := domain.ParseRunStatus(row.RunStatus)
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
	if row.SnapshotSourceModelAliasID != nil && row.ModelAliasID == nil {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "snapshot model alias reference is missing",
		}
	}
	if row.ModelAliasID != nil && row.ModelCatalogEntryID == nil {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "snapshot model catalog entry reference is missing",
		}
	}
	if row.SnapshotDeploymentType != row.RuntimeProfileExecutionTarget {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason: fmt.Sprintf(
				"snapshot deployment_type=%q does not match runtime execution_target=%q",
				row.SnapshotDeploymentType,
				row.RuntimeProfileExecutionTarget,
			),
		}
	}
	if row.SnapshotDeploymentType == "hosted_external" && row.SnapshotEndpointUrl == nil {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "hosted deployment snapshot is missing endpoint_url",
		}
	}
	if row.ProviderAccountID != nil && row.ModelAliasProviderAccountID != nil && *row.ProviderAccountID != *row.ModelAliasProviderAccountID {
		return RunAgentExecutionContext{}, FrozenExecutionContextError{
			RunAgentID: row.RunAgentID,
			Reason:     "snapshot provider account does not match model alias provider account",
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
			ChallengePackVersionID: row.RunChallengePackVersionID,
			ChallengeInputSetID:    cloneUUIDPtr(row.RunChallengeInputSetID),
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
			CreatedAt:              runCreatedAt,
			UpdatedAt:              runUpdatedAt,
		},
		RunAgent: domain.RunAgent{
			ID:                        row.RunAgentID,
			OrganizationID:            row.RunAgentOrganizationID,
			WorkspaceID:               row.RunAgentWorkspaceID,
			RunID:                     row.RunAgentRunID,
			AgentDeploymentID:         row.RunAgentAgentDeploymentID,
			AgentDeploymentSnapshotID: row.RunAgentAgentDeploymentSnapshotID,
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

	if row.ModelAliasID != nil {
		// Safe because these model_aliases/model_catalog_entries columns are NOT NULL in the schema.
		executionContext.Deployment.ModelAlias = &ModelAliasExecutionContext{
			ID:                *row.ModelAliasID,
			WorkspaceID:       cloneUUIDPtr(row.ModelAliasWorkspaceID),
			ProviderAccountID: cloneUUIDPtr(row.ModelAliasProviderAccountID),
			AliasKey:          *row.ModelAliasAliasKey,
			DisplayName:       *row.ModelAliasDisplayName,
			ModelCatalogEntry: ModelCatalogEntryExecutionContext{
				ID:              *row.ModelCatalogEntryID,
				ProviderKey:     *row.ModelCatalogProviderKey,
				ProviderModelID: *row.ModelCatalogProviderModelID,
				DisplayName:     *row.ModelCatalogDisplayName,
				ModelFamily:     *row.ModelCatalogModelFamily,
				Modality:        *row.ModelCatalogModality,
				Metadata:        cloneJSON(row.ModelCatalogMetadata),
			},
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
			ChallengeKey:        item.ChallengeKey,
			CaseKey:             caseKey,
			ItemKey:             item.ItemKey,
			Payload:             payload,
			Inputs:              append([]challengepack.CaseInput(nil), caseDoc.Inputs...),
			Expectations:        append([]challengepack.CaseExpectation(nil), caseDoc.Expectations...),
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
