package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

type evalSessionAdmissionRecovery interface {
	RecoverEvalSessionAdmission(context.Context, uuid.UUID, uuid.UUID, string) (repository.CreateEvalSessionWithQueuedRunsResult, bool, error)
}
type controlledSettingsRepository interface {
	PreviewControlledSettings(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *int32) (repository.ControlledSettings, error)
	ControlledCaseIDs(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) ([]uuid.UUID, error)
}
type EvalSessionPreview struct {
	DeploymentSnapshotIDs map[uuid.UUID]uuid.UUID         `json:"deployment_snapshot_ids"`
	ControlledSettings    map[uuid.UUID]string            `json:"controlled_settings"`
	Participants          []repository.ControlledSettings `json:"participants"`
	CaseIDs               []uuid.UUID                     `json:"case_ids"`
	ChildRuns             int                             `json:"child_runs"`
	ParticipantSamples    int                             `json:"participant_samples"`
	CaseSamples           int                             `json:"case_samples"`
	EstimatedUSD          *float64                        `json:"estimated_usd"`
	PriceAssumptions      string                          `json:"price_assumptions"`
	Limits                []string                        `json:"limits"`
	ChangedVariables      []string                        `json:"changed_variables"`
}

func (m *RunCreationManager) previewControlledChildren(ctx context.Context, input CreateEvalSessionInput, runs []repository.CreateQueuedRunParams) (*EvalSessionPreview, error) {
	repo, ok := m.repo.(controlledSettingsRepository)
	if !ok {
		return nil, fmt.Errorf("controlled preview unavailable")
	}
	if input.ChallengeInputSetID == nil {
		return nil, repository.ErrControlledModeUnsupported
	}
	// Child construction resolves the authoritative input set, including the legacy
	// single-input-set fallback. Preview always returns its exact canonical IDs.
	if len(runs) == 0 || runs[0].ChallengeInputSetID == nil {
		return nil, repository.ErrControlledModeUnsupported
	}
	challenges := []uuid.UUID{}
	for _, c := range runs[0].CaseSelections {
		challenges = append(challenges, c.ChallengeIdentityID)
	}
	ids, err := repo.ControlledCaseIDs(ctx, input.ChallengePackVersionID, *runs[0].ChallengeInputSetID, challenges)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("fixed scope has no canonical cases")
	}
	out := &EvalSessionPreview{DeploymentSnapshotIDs: map[uuid.UUID]uuid.UUID{}, ControlledSettings: map[uuid.UUID]string{}, Participants: []repository.ControlledSettings{}, CaseIDs: ids, ChildRuns: len(runs), ChangedVariables: []string{"repetition"}, PriceAssumptions: "Estimate assumes 1,000 input and 1,000 output tokens per case per allowed iteration, using static model prices. It excludes evaluator/tool/application charges and is not a maximum.", Limits: []string{"Up to 100 participant samples across all repetitions fit a complete canonical result view.", "Canonical workspace entitlement and spend limits apply at admission; live credential and revocation rules still apply.", "Provider defaults and model internals are not fixed. Saved sampling fields are not applied as provider request controls. Seed acknowledgment is unavailable.", "Deterministic evaluators and LLM judges using the guarded target account are supported; external/default judge routing and live human-preference inputs are unsupported."}}
	total := 0.0
	known := true
	for _, run := range runs {
		for _, agent := range run.RunAgents {
			if expected, ok := input.DeploymentSnapshotIDs[agent.AgentDeploymentID]; ok && expected != agent.AgentDeploymentSnapshotID {
				return nil, repository.ErrControlledSettingsChanged
			}
			settings, err := repo.PreviewControlledSettings(ctx, input.WorkspaceID, agent.AgentDeploymentSnapshotID, input.ChallengePackVersionID, input.MaxIterations)
			if err != nil {
				return nil, err
			}
			if _, ok := out.ControlledSettings[settings.SnapshotID]; !ok {
				out.Participants = append(out.Participants, settings)
			}
			out.ControlledSettings[settings.SnapshotID] = settings.Hash
			out.DeploymentSnapshotIDs[agent.AgentDeploymentID] = settings.SnapshotID
			out.ParticipantSamples++
			out.CaseSamples += len(ids)
			var v struct {
				Model    string `json:"requested_model"`
				Provider struct {
					Key string `json:"provider_key"`
				} `json:"provider"`
				Profile struct {
					Iterations int `json:"max_iterations"`
				} `json:"profile"`
				Mode string `json:"mode"`
			}
			_ = json.Unmarshal(settings.Settings, &v)
			in, output, ok := provider.StaticModelPrice(v.Provider.Key, v.Model)
			if !ok {
				known = false
			} else {
				iterations := v.Profile.Iterations
				if v.Mode == "prompt_eval" {
					iterations = 1
				}
				total += (in + output) * 0.001 * float64(len(ids)*iterations)
			}
		}
	}
	if known {
		out.EstimatedUSD = &total
	} else {
		out.PriceAssumptions = "A reliable total estimate is unavailable for one or more selected targets. External application, provider, evaluator and tool charges may apply; this is not a free run or a spending maximum."
	}
	if len(out.Participants) > 1 {
		out.ChangedVariables = append(out.ChangedVariables, "deployment snapshot and its declared model/build/settings")
	}
	return out, nil
}
