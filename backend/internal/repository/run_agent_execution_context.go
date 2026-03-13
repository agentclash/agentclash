package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
}

type ChallengeInputSetExecutionContext struct {
	ID                     uuid.UUID
	ChallengePackVersionID uuid.UUID
	InputKey               string
	Name                   string
	Description            *string
	InputChecksum          string
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
	RuntimeProfile            RuntimeProfileExecutionContext
	ProviderAccount           *ProviderAccountExecutionContext
	ModelAlias                *ModelAliasExecutionContext
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
				Reason:     "required frozen references could not be resolved",
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
		executionContext.ChallengeInputSet = &ChallengeInputSetExecutionContext{
			ID:                     *row.ChallengeInputSetID,
			ChallengePackVersionID: *row.ChallengeInputSetChallengePackVersionID,
			InputKey:               *row.ChallengeInputSetInputKey,
			Name:                   *row.ChallengeInputSetName,
			Description:            cloneStringPtr(row.ChallengeInputSetDescription),
			InputChecksum:          *row.ChallengeInputSetInputChecksum,
		}
	}

	if row.ProviderAccountID != nil {
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
