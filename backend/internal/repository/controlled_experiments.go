package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/agentclash/agentclash/runtime/scoring"
	"strings"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrControlledSettingsChanged = errors.New("controlled settings changed; prepare a new experiment")
var ErrControlledModeUnsupported = errors.New("controlled execution mode is unsupported")
var ErrExperimentRequestConflict = errors.New("experiment request id already names different input")

type ControlledSettings struct {
	Version    int             `json:"version"`
	SnapshotID uuid.UUID       `json:"snapshot_id"`
	Hash       string          `json:"hash"`
	Settings   json.RawMessage `json:"settings"`
}

type settingsReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Only non-secret values actually affecting the covered request owners enter this
// versioned receipt. Credential values, live budgets and revocation stay live.
func readControlledSettings(ctx context.Context, db settingsReader, workspace, snapshot, pack uuid.UUID, maxIterations *int32) (ControlledSettings, error) {
	var body []byte
	var kind, mode string
	err := db.QueryRow(ctx, `SELECT s.deployment_type,COALESCE(p.manifest #>> '{version,execution_mode}','native'),jsonb_build_object(
 'version',1,'snapshot_id',s.id,'pack_version_id',p.id,'pack_manifest',p.manifest,'deployment_type',s.deployment_type,
 'mode',COALESCE(p.manifest #>> '{version,execution_mode}','native'),
 'build_version_id',s.source_agent_build_version_id,'source_agent_spec',CASE WHEN s.deployment_type='native' THEN s.source_agent_spec ELSE NULL END,
 'snapshot_config',s.snapshot_config,'requested_model',CASE WHEN s.deployment_type='native' THEN s.source_model_id ELSE NULL END,
 'profile',CASE WHEN s.deployment_type='native' THEN jsonb_build_object('execution_target',r.execution_target,'trace_mode',r.trace_mode,
 'max_iterations',COALESCE($4::integer,r.max_iterations),
 'max_tool_calls',r.max_tool_calls,'step_timeout_seconds',r.step_timeout_seconds,'run_timeout_seconds',r.run_timeout_seconds,'profile_config',r.profile_config) ELSE NULL END,
 'provider',CASE WHEN s.deployment_type='native' THEN jsonb_build_object('account_id',a.id,'workspace_id',a.workspace_id,'organization_id',a.organization_id,'provider_key',a.provider_key,'base_url',a.base_url) ELSE NULL END,
 'connector',CASE WHEN c.snapshot_id IS NOT NULL THEN jsonb_build_object('adapter',c.adapter,'config',c.config,'target_origin',c.target_origin,'secret_id',c.secret_id,'secret_revision',c.secret_revision) ELSE NULL END
 ) FROM agent_deployment_snapshots s JOIN runtime_profiles r ON r.id=s.source_runtime_profile_id
 JOIN challenge_pack_versions p ON p.id=$3
 LEFT JOIN provider_accounts a ON a.id=s.source_provider_account_id
 LEFT JOIN connector_snapshots c ON c.snapshot_id=s.id
 JOIN agent_deployments d ON d.id=s.agent_deployment_id
 WHERE s.workspace_id=$1 AND s.id=$2 AND d.status='active' AND d.archived_at IS NULL
 AND r.archived_at IS NULL AND (s.deployment_type<>'native' OR (a.status='active' AND a.archived_at IS NULL))
 AND (s.deployment_type='native' OR c.snapshot_id IS NOT NULL)`, workspace, snapshot, pack, maxIterations).Scan(&kind, &mode, &body)
	if err != nil {
		return ControlledSettings{}, fmt.Errorf("%w: %v", ErrControlledSettingsChanged, err)
	}
	if kind == "native" && mode != "native" && mode != "prompt_eval" && mode != "" {
		return ControlledSettings{}, ErrControlledModeUnsupported
	}
	var receipt struct {
		Manifest json.RawMessage `json:"pack_manifest"`
		Provider struct {
			Key string `json:"provider_key"`
		} `json:"provider"`
	}
	if json.Unmarshal(body, &receipt) != nil {
		return ControlledSettings{}, ErrControlledModeUnsupported
	}
	if kind == "native" {
		switch receipt.Provider.Key {
		case "openai", "custom", "anthropic", "gemini", "xai", "openrouter", "mistral":
		default:
			return ControlledSettings{}, fmt.Errorf("%w: provider has no covered dispatch owner", ErrControlledModeUnsupported)
		}
	}
	spec, err := scoring.LoadEvaluationSpec(receipt.Manifest)
	if err != nil {
		return ControlledSettings{}, fmt.Errorf("%w: evaluator definition unavailable", ErrControlledModeUnsupported)
	}
	for _, d := range spec.Scorecard.Dimensions {
		if d.Source == scoring.DimensionSourceHumanPreference {
			return ControlledSettings{}, fmt.Errorf("%w: live human-preference inputs cannot be fixed", ErrControlledModeUnsupported)
		}
	}
	for _, judge := range spec.LLMJudges {
		models := judge.Models
		if strings.TrimSpace(judge.Model) != "" {
			models = []string{judge.Model}
		}
		for _, model := range models {
			key := scoring.InferJudgeProviderKey(model)
			if key == "" {
				key = receipt.Provider.Key
			}
			if kind != "native" || key != receipt.Provider.Key {
				return ControlledSettings{}, fmt.Errorf("%w: judge must use the guarded target provider account", ErrControlledModeUnsupported)
			}
		}
	}
	// DB JSONB provides deterministic key ordering; retain those bytes and hash them,
	// rather than reinterpreting unsupported sampling parameters as applied knobs.
	hash := sha256.Sum256(body)
	return ControlledSettings{Version: 1, SnapshotID: snapshot, Hash: hex.EncodeToString(hash[:]), Settings: body}, nil
}
func (r *Repository) PreviewControlledSettings(ctx context.Context, workspace, snapshot, pack uuid.UUID, maxIterations *int32) (ControlledSettings, error) {
	return readControlledSettings(ctx, r.db, workspace, snapshot, pack, maxIterations)
}

func controlledMaxIterations(plan json.RawMessage) *int32 {
	var value struct {
		RuntimeLimits struct {
			MaxIterations *int32 `json:"max_iterations"`
		} `json:"runtime_limits"`
	}
	if json.Unmarshal(plan, &value) != nil {
		return nil
	}
	return value.RuntimeLimits.MaxIterations
}

// Locks mutable setting owners before comparing and retaining the effective
// snapshot. Child creation and these receipts share the same transaction.
func admitControlledSettings(ctx context.Context, tx pgx.Tx, p CreateQueuedRunParams) (map[uuid.UUID]ControlledSettings, error) {
	out := map[uuid.UUID]ControlledSettings{}
	if len(p.ExpectedSettings) == 0 && len(p.ExpectedDeploymentSnapshots) == 0 {
		return out, nil
	}
	for _, agent := range p.RunAgents {
		expected, ok := p.ExpectedDeploymentSnapshots[agent.AgentDeploymentID]
		if !ok || expected != agent.AgentDeploymentSnapshotID {
			return nil, ErrControlledSettingsChanged
		}
		var current uuid.UUID
		err := tx.QueryRow(ctx, `SELECT s.id FROM agent_deployments d JOIN agent_deployment_snapshots s ON s.agent_deployment_id=d.id WHERE d.id=$1 AND d.workspace_id=$2 AND d.status='active' AND d.archived_at IS NULL ORDER BY s.created_at DESC,s.id DESC LIMIT 1 FOR UPDATE OF d FOR SHARE OF s`, agent.AgentDeploymentID, p.WorkspaceID).Scan(&current)
		if err != nil || current != expected {
			return nil, ErrControlledSettingsChanged
		}
		if len(p.ExpectedSettings) == 0 {
			continue
		}
		if p.RaceContext || p.ChallengeInputSetID == nil {
			return nil, ErrControlledModeUnsupported
		}
		// Separate locks avoid FOR SHARE on nullable outer-join relations.
		if _, err = tx.Exec(ctx, `SELECT r.id FROM runtime_profiles r JOIN agent_deployment_snapshots s ON s.source_runtime_profile_id=r.id WHERE s.id=$1 FOR SHARE OF r`, expected); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `SELECT a.id FROM provider_accounts a JOIN agent_deployment_snapshots s ON s.source_provider_account_id=a.id WHERE s.id=$1 FOR SHARE OF a`, expected); err != nil {
			return nil, err
		}
		settings, err := readControlledSettings(ctx, tx, p.WorkspaceID, expected, p.ChallengePackVersionID, controlledMaxIterations(p.ExecutionPlan))
		if err != nil {
			return nil, err
		}
		if settings.Hash != p.ExpectedSettings[expected] {
			return nil, ErrControlledSettingsChanged
		}
		out[expected] = settings
	}
	return out, nil
}
func persistControlledSettings(ctx context.Context, tx pgx.Tx, result CreateQueuedRunResult, settings map[uuid.UUID]ControlledSettings) error {
	for _, agent := range result.RunAgents {
		if value, ok := settings[agent.AgentDeploymentSnapshotID]; ok {
			if _, err := tx.Exec(ctx, `INSERT INTO run_agent_controlled_settings(run_agent_id,snapshot_id,settings_hash,settings) VALUES($1,$2,$3,$4)`, agent.ID, value.SnapshotID, value.Hash, value.Settings); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *Repository) installControlledGuard(ctx context.Context, e *RunAgentExecutionContext, reader settingsReader) error {
	var expected string
	err := reader.QueryRow(ctx, `SELECT settings_hash FROM run_agent_controlled_settings WHERE run_agent_id=$1`, e.RunAgent.ID).Scan(&expected)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	runID, snapshot, workspace, pack := e.RunAgent.ID, e.Deployment.AgentDeploymentSnapshotID, e.Run.WorkspaceID, e.Run.ChallengePackVersionID
	maxIterations := controlledMaxIterations(e.Run.ExecutionPlan)
	validate := func(ctx context.Context, current settingsReader) error {
		value, err := readControlledSettings(ctx, current, workspace, snapshot, pack, maxIterations)
		if err != nil {
			return err
		}
		if value.Hash != expected {
			return ErrControlledSettingsChanged
		}
		return nil
	}
	if err = validate(ctx, reader); err != nil {
		return err
	}
	e.ValidateControlledDispatch = func(ctx context.Context) error { return validate(ctx, r.db) }
	e.AcknowledgeControlledDispatch = func(ctx context.Context) error {
		if err := validate(ctx, r.db); err != nil {
			return err
		}
		_, err := r.db.Exec(ctx, `UPDATE run_agent_controlled_settings SET dispatch_validated_at=COALESCE(dispatch_validated_at,now()),dispatch_guard_version=1 WHERE run_agent_id=$1 AND settings_hash=$2`, runID, expected)
		return err
	}
	return nil
}

// Recovery reads durable canonical admission before re-running mutable admission
// decisions. The API authorizes workspace first; keys are scoped to workspace.
func (r *Repository) RecoverEvalSessionAdmission(ctx context.Context, workspace, request uuid.UUID, hash string) (CreateEvalSessionWithQueuedRunsResult, bool, error) {
	var id uuid.UUID
	var saved string
	err := r.db.QueryRow(ctx, `SELECT eval_session_id,request_hash FROM eval_session_admissions WHERE workspace_id=$1 AND request_id=$2`, workspace, request).Scan(&id, &saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return CreateEvalSessionWithQueuedRunsResult{}, false, nil
	}
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	if saved != hash {
		return CreateEvalSessionWithQueuedRunsResult{}, false, ErrExperimentRequestConflict
	}
	value, err := r.GetEvalSessionWithRuns(ctx, id)
	return CreateEvalSessionWithQueuedRunsResult{Session: value.Session, Runs: value.Runs}, true, err
}
func recoverEvalSessionTx(ctx context.Context, tx pgx.Tx, workspace, request uuid.UUID, hash string) (CreateEvalSessionWithQueuedRunsResult, bool, error) {
	var id uuid.UUID
	var saved string
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, workspace.String()+":"+request.String()); err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	err := tx.QueryRow(ctx, `SELECT eval_session_id,request_hash FROM eval_session_admissions WHERE workspace_id=$1 AND request_id=$2`, workspace, request).Scan(&id, &saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return CreateEvalSessionWithQueuedRunsResult{}, false, nil
	}
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	if saved != hash {
		return CreateEvalSessionWithQueuedRunsResult{}, false, ErrExperimentRequestConflict
	}
	q := repositorysqlc.New(tx)
	row, err := q.GetEvalSessionByID(ctx, repositorysqlc.GetEvalSessionByIDParams{ID: id})
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	session, err := mapEvalSession(row)
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	rows, err := q.ListRunsByEvalSessionID(ctx, repositorysqlc.ListRunsByEvalSessionIDParams{EvalSessionID: &id})
	if err != nil {
		return CreateEvalSessionWithQueuedRunsResult{}, false, err
	}
	out := CreateEvalSessionWithQueuedRunsResult{Session: session, Runs: []domain.Run{}}
	for _, row := range rows {
		run, err := mapRun(row)
		if err != nil {
			return out, false, err
		}
		out.Runs = append(out.Runs, run)
	}
	return out, true, nil
}

func (r *Repository) ControlledCaseIDs(ctx context.Context, pack, input uuid.UUID, challenges []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT i.id FROM challenge_input_items i JOIN challenge_pack_version_challenges c ON c.challenge_pack_version_id=i.challenge_pack_version_id AND c.challenge_identity_id=i.challenge_identity_id WHERE i.challenge_pack_version_id=$1 AND i.challenge_input_set_id=$2 AND i.challenge_identity_id=ANY($3) ORDER BY c.execution_order,i.item_key`, pack, input, challenges)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
