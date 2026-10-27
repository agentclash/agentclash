package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrConnectorUnapproved = errors.New("connector approval is absent, revoked, or its workspace secret has changed")
var ErrConnectorOutcomeUnknown = errors.New("connector outcome unknown; execution will not be automatically repeated")

type CreateConnectorParams struct {
	WorkspaceID, ActorUserID uuid.UUID
	Name                     string
	Config                   connectors.Config
}

// CreateConnector materializes only canonical registry records. The build is a
// protocol descriptor, never evidence of the external application's revision.
func (r *Repository) CreateConnector(ctx context.Context, p CreateConnectorParams) (connectors.Binding, error) {
	if err := p.Config.Validate(); err != nil {
		return connectors.Binding{}, err
	}
	if !IsValidSecretKey(p.Config.SecretKey) || strings.TrimSpace(p.Name) == "" || len(p.Name) > 200 {
		return connectors.Binding{}, errors.New("connector name and workspace secret key are required")
	}
	origin, err := connectors.Origin(p.Config.EndpointURL)
	if err != nil {
		return connectors.Binding{}, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return connectors.Binding{}, err
	}
	defer tx.Rollback(ctx)
	var org uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT organization_id FROM workspaces WHERE id=$1`, p.WorkspaceID).Scan(&org); err != nil {
		return connectors.Binding{}, err
	}
	b := connectors.Binding{SnapshotID: uuid.New(), DeploymentID: uuid.New(), WorkspaceID: p.WorkspaceID, Config: p.Config, TargetOrigin: origin}
	if err = tx.QueryRow(ctx, `SELECT id,revision FROM workspace_secrets WHERE workspace_id=$1 AND key=$2 FOR SHARE`, p.WorkspaceID, p.Config.SecretKey).Scan(&b.SecretID, &b.SecretRevision); err != nil {
		return b, ErrConnectorUnapproved
	}
	build, version, profile := uuid.New(), uuid.New(), uuid.New()
	slug := "connector-" + b.DeploymentID.String()
	_, err = tx.Exec(ctx, `INSERT INTO agent_builds(id,organization_id,workspace_id,name,slug,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6)`, build, org, p.WorkspaceID, p.Name, slug, p.ActorUserID)
	if err != nil {
		return b, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO agent_build_versions(id,agent_build_id,version_number,version_status,agent_kind,created_by_user_id) VALUES($1,$2,1,'ready','hosted_external',$3)`, version, build, p.ActorUserID)
	if err != nil {
		return b, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO runtime_profiles(id,organization_id,workspace_id,name,slug,execution_target,trace_mode,run_timeout_seconds,step_timeout_seconds) VALUES($1,$2,$3,$4,$5,'hosted_external','preferred',$6,$6)`, profile, org, p.WorkspaceID, p.Name, slug, p.Config.TimeoutSeconds)
	if err != nil {
		return b, err
	}
	raw, _ := json.Marshal(p.Config)
	_, err = tx.Exec(ctx, `INSERT INTO agent_deployments(id,organization_id,workspace_id,agent_build_id,current_build_version_id,runtime_profile_id,name,slug,deployment_type,endpoint_url,deployment_config) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'hosted_external',$9,$10)`, b.DeploymentID, org, p.WorkspaceID, build, version, profile, p.Name, slug, p.Config.EndpointURL, raw)
	if err != nil {
		return b, err
	}
	frozen, _ := json.Marshal(struct {
		Config   connectors.Config
		SecretID uuid.UUID
		Revision int64
	}{p.Config, b.SecretID, b.SecretRevision})
	hash := sha256.Sum256(frozen)
	_, err = tx.Exec(ctx, `INSERT INTO agent_deployment_snapshots(id,organization_id,workspace_id,agent_build_id,agent_deployment_id,source_agent_build_version_id,source_runtime_profile_id,deployment_type,endpoint_url,snapshot_hash,snapshot_config) VALUES($1,$2,$3,$4,$5,$6,$7,'hosted_external',$8,$9,$10)`, b.SnapshotID, org, p.WorkspaceID, build, b.DeploymentID, version, profile, p.Config.EndpointURL, hex.EncodeToString(hash[:]), raw)
	if err != nil {
		return b, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connector_snapshots(snapshot_id,workspace_id,organization_id,adapter,config,target_origin,secret_id,secret_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, b.SnapshotID, p.WorkspaceID, org, p.Config.Adapter, raw, origin, b.SecretID, b.SecretRevision)
	if err != nil {
		return b, err
	}
	return b, tx.Commit(ctx)
}

// Approval compares the reviewed immutable tuple. Rotation requires a new
// snapshot; approving an old config never silently adopts a replacement secret.
func (r *Repository) ApproveConnector(ctx context.Context, b connectors.Binding, actor uuid.UUID) (connectors.Binding, error) {
	tag, err := r.db.Exec(ctx, `UPDATE connector_snapshots c SET approved_by=$5,approved_at=now(),revoked_at=NULL FROM workspace_secrets s WHERE c.snapshot_id=$1 AND c.workspace_id=$2 AND c.target_origin=$3 AND c.secret_id=$4 AND c.secret_revision=$6 AND s.id=c.secret_id AND s.workspace_id=c.workspace_id AND s.revision=c.secret_revision`, b.SnapshotID, b.WorkspaceID, b.TargetOrigin, b.SecretID, actor, b.SecretRevision)
	if err != nil {
		return b, err
	}
	if tag.RowsAffected() != 1 {
		return b, ErrConnectorUnapproved
	}
	return r.GetConnector(ctx, b.WorkspaceID, b.SnapshotID)
}
func (r *Repository) RevokeConnector(ctx context.Context, workspace, snapshot uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE connector_snapshots SET revoked_at=now() WHERE snapshot_id=$1 AND workspace_id=$2`, snapshot, workspace)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConnectorUnapproved
	}
	return nil
}
func (r *Repository) GetConnector(ctx context.Context, workspace, snapshot uuid.UUID) (connectors.Binding, error) {
	var b connectors.Binding
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT c.snapshot_id,d.agent_deployment_id,c.workspace_id,c.config,c.target_origin,c.secret_id,c.secret_revision,(c.approved_at IS NOT NULL AND c.revoked_at IS NULL AND s.id IS NOT NULL) FROM connector_snapshots c JOIN agent_deployment_snapshots d ON d.id=c.snapshot_id LEFT JOIN workspace_secrets s ON s.id=c.secret_id AND s.workspace_id=c.workspace_id AND s.revision=c.secret_revision WHERE c.workspace_id=$1 AND c.snapshot_id=$2`, workspace, snapshot).Scan(&b.SnapshotID, &b.DeploymentID, &b.WorkspaceID, &raw, &b.TargetOrigin, &b.SecretID, &b.SecretRevision, &b.Approved)
	if err != nil {
		return b, err
	}
	err = json.Unmarshal(raw, &b.Config)
	return b, err
}
func (r *Repository) ListConnectors(ctx context.Context, workspace uuid.UUID) ([]connectors.Binding, error) {
	rows, err := r.db.Query(ctx, `SELECT snapshot_id FROM connector_snapshots WHERE workspace_id=$1 ORDER BY snapshot_id`, workspace)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	out := []connectors.Binding{}
	for _, id := range ids {
		b, err := r.GetConnector(ctx, workspace, id)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// ResolveApprovedConnector has no env://, secret://, provider-account or process
// fallback. Exact UUID, revision, workspace and origin must still match.
func (r *Repository) ResolveApprovedConnector(ctx context.Context, snapshot, workspace uuid.UUID) (connectors.Binding, string, error) {
	b, err := r.GetConnector(ctx, workspace, snapshot)
	if err != nil || !b.Approved {
		return b, "", ErrConnectorUnapproved
	}
	if r.cipher == nil {
		return b, "", ErrSecretsCipherUnset
	}
	origin, err := connectors.Origin(b.Config.EndpointURL)
	if err != nil || origin != b.TargetOrigin {
		return b, "", ErrConnectorUnapproved
	}
	if err = b.Config.Validate(); err != nil {
		return b, "", err
	}
	var encrypted []byte
	err = r.db.QueryRow(ctx, `SELECT s.encrypted_value FROM workspace_secrets s JOIN connector_snapshots c ON c.secret_id=s.id AND c.secret_revision=s.revision AND c.workspace_id=s.workspace_id WHERE c.snapshot_id=$1 AND c.workspace_id=$2 AND c.target_origin=$3 AND c.approved_at IS NOT NULL AND c.revoked_at IS NULL`, snapshot, workspace, origin).Scan(&encrypted)
	if err != nil {
		return b, "", ErrConnectorUnapproved
	}
	plaintext, err := r.cipher.Decrypt(encrypted)
	if err != nil {
		return b, "", errors.New("workspace credential could not be decrypted")
	}
	return b, string(plaintext), nil
}

// ValidateConnectorAdmission also refuses legacy unapproved hosted snapshots.
// Native deployments retain their existing entitlement/provider/spend owners.
func validateConnectorAdmission(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, workspace, snapshot uuid.UUID) error {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT d.deployment_type='native' OR EXISTS(SELECT 1 FROM connector_snapshots c JOIN workspace_secrets s ON s.id=c.secret_id AND s.revision=c.secret_revision AND s.workspace_id=c.workspace_id WHERE c.snapshot_id=d.id AND c.approved_at IS NOT NULL AND c.revoked_at IS NULL) FROM agent_deployment_snapshots d WHERE d.id=$1 AND d.workspace_id=$2`, snapshot, workspace).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrConnectorUnapproved
	}
	return nil
}

type ConnectorDispatch struct {
	ID         uuid.UUID
	State      string
	DeadlineAt time.Time
	Response   json.RawMessage
}

func (r *Repository) ClaimConnectorDispatch(ctx context.Context, runAgent, snapshot, sourceCase uuid.UUID, caseKey, challengeKey string, input []byte, deadline time.Time) (ConnectorDispatch, bool, error) {
	hash := sha256.Sum256(input)
	digest := hex.EncodeToString(hash[:])
	var d ConnectorDispatch
	// Persist intent BEFORE the network. ON CONFLICT never rewrites prior intent.
	err := r.db.QueryRow(ctx, `INSERT INTO connector_dispatches(run_agent_id,snapshot_id,case_key,input_hash,state,deadline_at,source_case_id,challenge_key) SELECT ra.id,$2,$3,$4,'sending',$5,$6,$7 FROM run_agents ra WHERE ra.id=$1 AND ra.agent_deployment_snapshot_id=$2 ON CONFLICT(run_agent_id,source_case_id) DO NOTHING RETURNING id,state,deadline_at,response`, runAgent, snapshot, caseKey, digest, deadline, sourceCase, challengeKey).Scan(&d.ID, &d.State, &d.DeadlineAt, &d.Response)
	if err == nil {
		return d, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return d, false, err
	}
	var previousHash string
	var previousSnapshot uuid.UUID
	err = r.db.QueryRow(ctx, `SELECT id,state,deadline_at,response,input_hash,snapshot_id FROM connector_dispatches WHERE run_agent_id=$1 AND source_case_id=$2`, runAgent, sourceCase).Scan(&d.ID, &d.State, &d.DeadlineAt, &d.Response, &previousHash, &previousSnapshot)
	if err != nil {
		return d, false, err
	}
	if previousHash != digest || previousSnapshot != snapshot {
		return d, false, errors.New("connector dispatch identity conflict")
	}
	return d, false, nil
}
func (r *Repository) UnknownConnectorDispatch(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE connector_dispatches SET state='unknown',updated_at=now() WHERE id=$1 AND state='sending'`, id)
	return err
}

// CommitConnectorResponse atomically stores mapped output, canonical evidence,
// replay and completion. A crash can never leave completed output without evidence.
func (r *Repository) CommitConnectorResponse(ctx context.Context, id uuid.UUID, response json.RawMessage) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var run, agent uuid.UUID
	var key, challengeKey, state string
	var sourceCase uuid.UUID
	err = tx.QueryRow(ctx, `SELECT ra.run_id,d.run_agent_id,d.case_key,d.state,d.challenge_key,d.source_case_id FROM connector_dispatches d JOIN run_agents ra ON ra.id=d.run_agent_id WHERE d.id=$1 FOR UPDATE OF d`, id).Scan(&run, &agent, &key, &state, &challengeKey, &sourceCase)
	if err != nil {
		return err
	}
	if state == "completed" {
		return nil
	}
	if state != "sending" {
		return ErrConnectorOutcomeUnknown
	}
	payload, err := json.Marshal(map[string]any{"case_key": key, "challenge_key": challengeKey, "source_case_id": sourceCase, "final_output": response, "output": response, "adapter": "http_json", "external_charges_included": false})
	if err != nil {
		return err
	}
	_, err = r.recordHostedRunEventTx(ctx, tx, RecordHostedRunEventParams{Event: runevents.Envelope{EventID: "connector:" + id.String(), SchemaVersion: runevents.SchemaVersionV1, RunID: run, RunAgentID: agent, EventType: runevents.EventTypeSystemOutputFinalized, Source: runevents.SourceHostedExternal, OccurredAt: time.Now().UTC(), Payload: payload, Summary: runevents.SummaryMetadata{EvidenceLevel: runevents.EvidenceLevelHostedBlackBox}}})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE connector_dispatches SET state='completed',response=$2,updated_at=now() WHERE id=$1`, id, response)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ConnectorDeadline(ctx context.Context, agent uuid.UUID, deadline time.Time) (time.Time, error) {
	var frozen time.Time
	err := r.db.QueryRow(ctx, `INSERT INTO connector_run_deadlines(run_agent_id,deadline_at) VALUES($1,$2) ON CONFLICT(run_agent_id) DO UPDATE SET run_agent_id=EXCLUDED.run_agent_id RETURNING deadline_at`, agent, deadline).Scan(&frozen)
	return frozen, err
}
