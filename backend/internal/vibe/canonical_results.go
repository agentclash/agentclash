package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

// Canonical metadata is an authorized projection, not an Operation or a raw-wire archive.
type CanonicalCase struct {
	CaseEvidenceBinding
	ChallengeKey string `json:"challenge_key"`
	InputSetKey  string `json:"input_set_key"`
}
type CanonicalEvent struct {
	ID          string  `json:"id"`
	SourceID    *string `json:"source_event_id,omitempty"`
	Sequence    int64   `json:"sequence"`
	Type        string  `json:"type"`
	PayloadHash string  `json:"normalized_payload_sha256"`
}
type CanonicalParticipant struct {
	RunID        uuid.UUID        `json:"run_id"`
	RunAgentID   uuid.UUID        `json:"run_agent_id"`
	SnapshotID   uuid.UUID        `json:"snapshot_id"`
	DeploymentID uuid.UUID        `json:"deployment_id"`
	Adapter      string           `json:"adapter"`
	Status       string           `json:"status"`
	Label        string           `json:"label"`
	InputSetID   uuid.UUID        `json:"challenge_input_set_id"`
	Cases        []CanonicalCase  `json:"cases"`
	Fingerprints Fingerprints     `json:"fingerprints"`
	Events       []CanonicalEvent `json:"events"`
}
type CanonicalResult struct {
	Binding          ProjectResultBinding   `json:"binding"`
	EvidenceMetadata EvidenceMetadata       `json:"evidence_metadata"`
	Name             string                 `json:"name"`
	Status           string                 `json:"status"`
	CreatedAt        time.Time              `json:"created_at"`
	Participants     []CanonicalParticipant `json:"participants"`
	EvidenceHash     string                 `json:"evidence_hash"`
	Omissions        []string               `json:"omissions"`
}

var canonicalOmissions = []string{"Raw canonical event and replay bodies are omitted from this metadata export.", "HTTP responses retain mapped JSON only; full upstream HTTP bodies and exact callback wire bytes are unavailable.", "External application charges and remote cancellation acknowledgments are not captured. Tool/retrieval capture and external model/build identity remain unknown."}

const canonicalMetadataMaxBytes = 8 << 20

func canonicalFingerprint(domain string, v any) *string {
	s := "canonical:" + domain + ":v1:sha256:" + Hash(raw(v))
	return &s
}

type canonicalTestsIdentity struct {
	Version          uuid.UUID
	InputSet         uuid.UUID
	ManifestChecksum string
	InputChecksum    string
	OrderedCaseIDs   []uuid.UUID
}
type canonicalJudgeIdentity struct {
	Version        uuid.UUID
	EvaluationSpec json.RawMessage
}
type canonicalBuildIdentity struct {
	Version         uuid.UUID
	SourceAgentSpec json.RawMessage
}

func (s *Store) CanonicalResult(ctx context.Context, actor string, session uuid.UUID, ref ResultRef, artifact *uuid.UUID) (CanonicalResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CanonicalResult{}, err
	}
	defer tx.Rollback(ctx)
	v, err := sdkAccess(ctx, tx, actor, session, false)
	if err != nil {
		return CanonicalResult{}, err
	}
	out, err := canonicalResultTx(ctx, tx, v, ref, artifact)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) CanonicalResults(ctx context.Context, actor string, session uuid.UUID) ([]CanonicalResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	v, err := sdkAccess(ctx, tx, actor, session, false)
	if err != nil {
		return nil, err
	}
	out, err := canonicalResultsTx(ctx, tx, v)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
func canonicalResultsTx(ctx context.Context, tx pgx.Tx, v Session) ([]CanonicalResult, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT r.id,r.eval_session_id,p.artifact_id,r.created_at FROM runs r JOIN vibe_published_case_bindings b ON b.challenge_pack_version_id=r.challenge_pack_version_id JOIN vibe_pack_sources p ON p.draft_id=b.draft_id AND p.artifact_id=b.artifact_id WHERE p.session_id=$1 AND p.workspace_id=$2 AND r.workspace_id=$2 ORDER BY r.created_at DESC,r.id,p.artifact_id LIMIT 101`, v.ID, v.WorkspaceID)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		id, artifact uuid.UUID
		eval         *uuid.UUID
	}
	cs := []candidate{}
	for rows.Next() {
		var c candidate
		var created time.Time
		if err = rows.Scan(&c.id, &c.eval, &c.artifact, &created); err != nil {
			break
		}
		cs = append(cs, c)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(cs) > 100 {
		return nil, fault("export_limit", "Canonical history exceeds this view. Choose an exact result.")
	}
	out := []CanonicalResult{}
	metadataBytes := 0
	seen := map[string]bool{}
	for _, c := range cs {
		refs := []ResultRef{{Kind: ResultRun, ID: c.id}}
		if c.eval != nil {
			refs = append(refs, ResultRef{Kind: ResultEvalSession, ID: *c.eval})
		}
		for _, ref := range refs {
			key := string(ref.Kind) + ref.ID.String() + c.artifact.String()
			if seen[key] {
				continue
			}
			seen[key] = true
			r, e := canonicalResultTx(ctx, tx, v, ref, &c.artifact)
			if e != nil {
				var f *Fault
				if errors.As(e, &f) && f.Code == "not_found" {
					continue
				}
				return nil, e
			}
			metadataBytes += len(raw(r))
			if metadataBytes > canonicalMetadataMaxBytes {
				return nil, fault("export_limit", "Canonical history exceeds one metadata view. Choose an exact result.")
			}
			out = append(out, r)
		}
	}
	return out, nil
}
func canonicalResultTx(ctx context.Context, tx pgx.Tx, v Session, ref ResultRef, artifact *uuid.UUID) (CanonicalResult, error) {
	out := CanonicalResult{Binding: ProjectResultBinding{SessionID: v.ID, ResultRef: ref}, EvidenceMetadata: EvidenceMetadata{ResultRef: ref, EvidenceProvenance: EvidenceProvenance{Origin: OriginLive, CaptureScope: UnknownCaptureScope()}}, Participants: []CanonicalParticipant{}, Omissions: append([]string(nil), canonicalOmissions...)}
	unavailable := func() (CanonicalResult, error) {
		return out, fault("not_found", "Canonical result is unavailable for this project and published version.")
	}
	if v.WorkspaceID == nil || ref.ID == uuid.Nil || (ref.Kind != ResultRun && ref.Kind != ResultEvalSession) {
		return unavailable()
	}
	if err := authorize(ctx, tx, v.Actor, v.WorkspaceID, false); err != nil {
		return out, err
	}
	if artifact != nil {
		if _, err := sdkArtifact(v, *artifact); err != nil {
			return out, err
		}
	}
	type runRow struct {
		id, workspace, version, input     uuid.UUID
		name, status                      string
		created                           time.Time
		plan, manifest                    []byte
		checksum, inputChecksum, inputKey string
	}
	query := `SELECT r.id,r.workspace_id,r.challenge_pack_version_id,r.challenge_input_set_id,r.name,r.status,r.created_at,r.execution_plan,p.manifest,p.manifest_checksum,i.input_checksum,i.input_key FROM runs r JOIN challenge_pack_versions p ON p.id=r.challenge_pack_version_id JOIN challenge_input_sets i ON i.id=r.challenge_input_set_id WHERE `
	if ref.Kind == ResultRun {
		query += `r.id=$1`
	} else {
		query += `r.eval_session_id=$1`
	}
	query += ` ORDER BY r.created_at,r.id LIMIT 101`
	rows, err := tx.Query(ctx, query, ref.ID)
	if err != nil {
		return out, err
	}
	runs := []runRow{}
	for rows.Next() {
		var r runRow
		if err = rows.Scan(&r.id, &r.workspace, &r.version, &r.input, &r.name, &r.status, &r.created, &r.plan, &r.manifest, &r.checksum, &r.inputChecksum, &r.inputKey); err != nil {
			break
		}
		runs = append(runs, r)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(runs) == 0 || len(runs) > 100 {
		return unavailable()
	}
	// Never amplify the legacy zero-child eval-session ACL hole, or hide unmapped/foreign children.
	if ref.Kind == ResultEvalSession {
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM runs WHERE eval_session_id=$1`, ref.ID).Scan(&count); err != nil {
			return out, err
		}
		if count != len(runs) {
			return unavailable()
		}
		if err = tx.QueryRow(ctx, `SELECT status,created_at FROM eval_sessions WHERE id=$1`, ref.ID).Scan(&out.Status, &out.CreatedAt); err != nil {
			return unavailable()
		}
		out.Name = "Evaluation session"
	} else {
		out.Status = runs[0].status
		out.Name = runs[0].name
		out.CreatedAt = runs[0].created
	}
	candidates := map[uuid.UUID]bool{}
	for _, a := range v.Document.Artifacts {
		if artifact == nil || a.ID == *artifact {
			candidates[a.ID] = true
		}
	}
	eventCount := 0
	for _, r := range runs {
		if r.workspace != *v.WorkspaceID {
			return unavailable()
		}
		if out.Binding.ChallengePackVersionID == nil {
			id := r.version
			out.Binding.ChallengePackVersionID = &id
		} else if *out.Binding.ChallengePackVersionID != r.version {
			return unavailable()
		}
		type caseRow struct {
			id             uuid.UUID
			key, challenge string
		}
		cases := []caseRow{}
		cr, e := tx.Query(ctx, `SELECT i.id,i.item_key,ci.challenge_key FROM challenge_input_items i JOIN challenge_pack_version_challenges c ON c.challenge_pack_version_id=i.challenge_pack_version_id AND c.challenge_identity_id=i.challenge_identity_id JOIN challenge_identities ci ON ci.id=i.challenge_identity_id WHERE i.challenge_input_set_id=$1 AND (NOT EXISTS(SELECT 1 FROM run_case_selections WHERE run_id=$2) OR EXISTS(SELECT 1 FROM run_case_selections WHERE run_id=$2 AND challenge_identity_id=i.challenge_identity_id)) ORDER BY c.execution_order,i.item_key`, r.input, r.id)
		if e != nil {
			return out, e
		}
		for cr.Next() {
			var c caseRow
			if e = cr.Scan(&c.id, &c.key, &c.challenge); e != nil {
				break
			}
			cases = append(cases, c)
		}
		if e == nil {
			e = cr.Err()
		}
		cr.Close()
		if e != nil {
			return out, e
		}
		if len(cases) == 0 || len(cases) > 200 {
			return unavailable()
		}
		ids := []uuid.UUID{}
		for _, c := range cases {
			ids = append(ids, c.id)
			for a := range candidates {
				var matches bool
				e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_published_case_bindings b JOIN vibe_pack_sources p ON p.draft_id=b.draft_id AND p.artifact_id=b.artifact_id WHERE p.session_id=$1 AND p.workspace_id=$2 AND b.artifact_id=$3 AND b.challenge_pack_version_id=$4 AND b.published_case_id=$5 AND b.input_set_key=$6 AND b.challenge_key=$7 AND b.case_key=$8)`, v.ID, v.WorkspaceID, a, r.version, c.id, r.inputKey, c.challenge, c.key).Scan(&matches)
				if e != nil {
					return out, e
				}
				if !matches {
					delete(candidates, a)
				}
			}
		}
		ar, e := tx.Query(ctx, `SELECT a.id,a.agent_deployment_id,a.agent_deployment_snapshot_id,a.label,a.status,s.deployment_type,s.snapshot_hash,s.source_agent_build_version_id,s.source_agent_spec,c.config,c.secret_id,c.secret_revision FROM run_agents a JOIN agent_deployment_snapshots s ON s.id=a.agent_deployment_snapshot_id AND s.workspace_id=a.workspace_id LEFT JOIN connector_snapshots c ON c.snapshot_id=s.id WHERE a.run_id=$1 AND a.workspace_id=$2 ORDER BY a.lane_index LIMIT 101`, r.id, r.workspace)
		if e != nil {
			return out, e
		}
		type agentData struct {
			p            CanonicalParticipant
			typ, hash    string
			build        uuid.UUID
			spec, config []byte
			secret       *uuid.UUID
			revision     *int64
		}
		agents := []agentData{}
		for ar.Next() {
			var a agentData
			a.p.RunID = r.id
			a.p.InputSetID = r.input
			if e = ar.Scan(&a.p.RunAgentID, &a.p.DeploymentID, &a.p.SnapshotID, &a.p.Label, &a.p.Status, &a.typ, &a.hash, &a.build, &a.spec, &a.config, &a.secret, &a.revision); e != nil {
				break
			}
			agents = append(agents, a)
		}
		if e == nil {
			e = ar.Err()
		}
		ar.Close()
		if e != nil {
			return out, e
		}
		if len(agents) == 0 || len(agents)+len(out.Participants) > 100 {
			return unavailable()
		}
		var participantCount int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM run_agents WHERE run_id=$1`, r.id).Scan(&participantCount); e != nil {
			return out, e
		}
		if participantCount != len(agents) {
			return unavailable()
		}
		for _, a := range agents {
			p := a.p
			p.Adapter = "native"
			p.Cases = []CanonicalCase{}
			p.Events = []CanonicalEvent{}
			for _, c := range cases {
				id := p.RunAgentID
				p.Cases = append(p.Cases, CanonicalCase{CaseEvidenceBinding: CaseEvidenceBinding{CaseKey: c.key, SourceCaseID: c.id.String(), RunAgentID: &id}, ChallengeKey: c.challenge, InputSetKey: r.inputKey})
			}
			p.Fingerprints.Test = canonicalFingerprint("tests", canonicalTestsIdentity{r.version, r.input, r.checksum, r.inputChecksum, ids})
			var manifest struct {
				EvaluationSpec json.RawMessage `json:"evaluation_spec"`
			}
			if json.Unmarshal(r.manifest, &manifest) == nil && len(manifest.EvaluationSpec) > 0 && string(manifest.EvaluationSpec) != "null" {
				p.Fingerprints.Judge = canonicalFingerprint("judge", canonicalJudgeIdentity{r.version, manifest.EvaluationSpec})
			}
			if a.typ == "native" {
				if len(a.spec) > 0 && string(a.spec) != "{}" {
					p.Fingerprints.Build = canonicalFingerprint("build", canonicalBuildIdentity{a.build, a.spec})
				}
			} else {
				p.Adapter = "hosted"
				if len(a.config) > 0 && a.secret != nil && a.revision != nil {
					var c struct {
						Adapter string `json:"adapter"`
					}
					if json.Unmarshal(a.config, &c) != nil {
						return unavailable()
					}
					p.Adapter = c.Adapter
					// The enclosing runtime-profile deadline is mutable, not admitted snapshot data.
					// SnapshotID retains frozen mapping/auth identity; a partial config hash would overclaim.
				}
			}
			er, e := tx.Query(ctx, `SELECT e.id::text,h.source_event_id,e.sequence_number,e.event_type,e.payload FROM run_events e LEFT JOIN hosted_event_receipts h ON h.run_agent_id=e.run_agent_id AND h.sequence_number=e.sequence_number WHERE e.run_agent_id=$1 ORDER BY e.sequence_number LIMIT 5001`, p.RunAgentID)
			if e != nil {
				return out, e
			}
			for er.Next() {
				var event CanonicalEvent
				var payload []byte
				if e = er.Scan(&event.ID, &event.SourceID, &event.Sequence, &event.Type, &payload); e != nil {
					break
				}
				event.PayloadHash = Hash(payload)
				p.Events = append(p.Events, event)
			}
			if e == nil {
				e = er.Err()
			}
			er.Close()
			if e != nil {
				return out, e
			}
			if len(p.Events) > 5000 {
				return out, fault("export_limit", "Canonical evidence exceeds one metadata view.")
			}
			eventCount += len(p.Events)
			if eventCount > 5000 {
				return out, fault("export_limit", "Canonical evidence exceeds one metadata view.")
			}
			out.Participants = append(out.Participants, p)
		}
	}
	if len(candidates) != 1 {
		return unavailable()
	}
	for id := range candidates {
		out.Binding.ArtifactID = id
	}
	// Participant hashes remain authoritative. A mixed aggregate never claims one configuration.
	if len(out.Participants) > 0 {
		out.EvidenceMetadata.Fingerprints = out.Participants[0].Fingerprints
		for _, p := range out.Participants[1:] {
			a, b := &out.EvidenceMetadata.Fingerprints, p.Fingerprints
			for _, pair := range []struct {
				x **string
				y *string
			}{{&a.Model, b.Model}, {&a.Build, b.Build}, {&a.Test, b.Test}, {&a.Judge, b.Judge}, {&a.Config, b.Config}} {
				if *pair.x == nil || pair.y == nil || **pair.x != *pair.y {
					*pair.x = nil
				}
			}
		}
	}
	encoded := raw(out)
	if len(encoded) > canonicalMetadataMaxBytes {
		return out, fault("export_limit", "Canonical evidence exceeds one metadata view.")
	}
	out.EvidenceHash = Hash(encoded)
	return out, nil
}
func appendCanonicalExport(ctx context.Context, tx pgx.Tx, v Session, out *ExportPreparation) error {
	var results []CanonicalResult
	var err error
	if out.Selection.Scope == "history" {
		results, err = canonicalResultsTx(ctx, tx, v)
	} else if ref := out.Selection.ResultRef; ref != nil && (ref.Kind == ResultRun || ref.Kind == ResultEvalSession) {
		var r CanonicalResult
		r, err = canonicalResultTx(ctx, tx, v, *ref, &out.ArtifactID)
		if err == nil {
			results = []CanonicalResult{r}
		}
	}
	if err != nil {
		return err
	}
	for _, r := range results {
		out.CanonicalResults = append(out.CanonicalResults, r)
		ref, a := r.Binding.ResultRef, r.Binding.ArtifactID
		out.Items = append(out.Items, ExportItem{Kind: "canonical_result", ID: string(ref.Kind) + ":" + ref.ID.String() + ":" + a.String(), ResultRef: &ref, ArtifactID: &a, Hash: r.EvidenceHash})
	}
	if len(results) > 0 {
		out.Omissions = append(out.Omissions, canonicalOmissions...)
	}
	return nil
}
