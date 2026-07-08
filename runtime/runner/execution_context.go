package runner

import (
	"encoding/json"
	"time"

	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
)

const maxExecutionPlanMaxIterations = 1000

// ExecutionContext is the frozen, backend-neutral shape needed to execute one
// run agent. Hosted repositories, local stores, and future desktop runtimes can
// build this value from different persistence layers.
type ExecutionContext struct {
	Run                  domain.Run
	RunAgent             domain.RunAgent
	RunAgents            []domain.RunAgent
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
	ID                  uuid.UUID                        `json:"id"`
	ChallengeIdentityID uuid.UUID                        `json:"challenge_identity_id"`
	RegressionCaseID    *uuid.UUID                       `json:"regression_case_id,omitempty"`
	ChallengeKey        string                           `json:"challenge_key"`
	CaseKey             string                           `json:"case_key"`
	ItemKey             string                           `json:"item_key"`
	Payload             json.RawMessage                  `json:"payload,omitempty"`
	Inputs              []challengepack.CaseInput        `json:"inputs,omitempty"`
	Expectations        []challengepack.CaseExpectation  `json:"expectations,omitempty"`
	UserSimulator       *challengepack.UserSimulatorSpec `json:"user_simulator,omitempty"`
	Artifacts           []challengepack.ArtifactRef      `json:"artifacts,omitempty"`
	Assets              []challengepack.AssetReference   `json:"assets,omitempty"`
}

type ChallengeInputItemExecutionContext struct {
	ID                  uuid.UUID       `json:"id"`
	ChallengeIdentityID uuid.UUID       `json:"challenge_identity_id"`
	RegressionCaseID    *uuid.UUID      `json:"regression_case_id,omitempty"`
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
	// ModelID is the provider model id chosen for this deployment (e.g.
	// "claude-sonnet-4-20250514"), sent directly to the provider.
	ModelID string
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

func StepTimeout(executionContext ExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds) * time.Second
}

func RunTimeout(executionContext ExecutionContext) time.Duration {
	if executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds) * time.Second
}

func MaxIterationsLimit(executionContext ExecutionContext) int {
	if override := ExecutionPlanMaxIterations(executionContext.Run.ExecutionPlan); override != nil {
		return int(*override)
	}
	return int(executionContext.Deployment.RuntimeProfile.MaxIterations)
}

func ExecutionPlanMaxIterations(executionPlan json.RawMessage) *int32 {
	var document struct {
		RuntimeLimits struct {
			MaxIterations *int32 `json:"max_iterations"`
		} `json:"runtime_limits"`
	}
	if len(executionPlan) == 0 {
		return nil
	}
	if err := json.Unmarshal(executionPlan, &document); err != nil {
		return nil
	}
	if document.RuntimeLimits.MaxIterations == nil {
		return nil
	}
	value := *document.RuntimeLimits.MaxIterations
	if value <= 0 || value > maxExecutionPlanMaxIterations {
		return nil
	}
	return &value
}
