package vibe

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
)

type AdvancedRequest struct {
	Revision    int64     `json:"revision"`
	ArtifactID  uuid.UUID `json:"artifact_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

// Canonical JSON, independent of PostgreSQL JSONB whitespace or object order.
func compositionHash(b []byte) (string, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return "", err
	}
	return Hash(raw(v)), nil
}

type CaseMapping struct {
	CaseKey      string `json:"case_key"`
	InputSetKey  string `json:"input_set_key"`
	ChallengeKey string `json:"challenge_key"`
}

func packSource(ctx context.Context, tx pgx.Tx, v Session, draft, ws uuid.UUID, a Artifact, composition json.RawMessage, advanced bool) error {
	var c challengepack.Composition
	if err := json.Unmarshal(composition, &c); err != nil {
		return err
	}
	b, err := challengepack.ComposeBundle(c, challengepack.ResolvedPieces{})
	if err != nil {
		return err
	}
	keys := []CaseMapping{}
	for _, set := range b.InputSets {
		for _, item := range set.Cases {
			keys = append(keys, CaseMapping{item.EffectiveKey(), set.Key, item.ChallengeKey})
		}
	}
	hash, err := compositionHash(composition)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO vibe_pack_sources(draft_id,session_id,artifact_id,workspace_id,composition_hash,case_keys,advanced,original_filename,original_media_type,original_hash,original_content)
 SELECT $1,$2,$3,$4,$5,$6,$7,o.filename,o.media_type,o.content_sha256,o.content FROM (SELECT 1) dummy LEFT JOIN vibe_import_originals o ON o.session_id=$2 AND o.artifact_id=$3`, draft, v.ID, a.ID, ws, hash, raw(keys), advanced)
	return err
}
func (s *Service) SaveAdvanced(ctx context.Context, actor string, id uuid.UUID, in AdvancedRequest) (uuid.UUID, error) {
	var draft uuid.UUID
	err := s.Store.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", id))
		if err != nil {
			return err
		}
		if v.Actor != actor {
			return fault("not_found", "Project is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		if err = authorize(ctx, tx, actor, &in.WorkspaceID, true); err != nil {
			return err
		}
		uid, e := uuid.Parse(strings.TrimPrefix(actor, "user:"))
		if e != nil {
			return fault("forbidden", "Sign in to save an advanced pack.")
		}
		if v.WorkspaceID != nil && *v.WorkspaceID != in.WorkspaceID {
			return fault("workspace_conflict", "Choose this project's workspace.")
		}
		err = tx.QueryRow(ctx, "SELECT draft_id FROM vibe_pack_sources WHERE session_id=$1 AND artifact_id=$2 AND workspace_id=$3 AND advanced", id, in.ArtifactID, in.WorkspaceID).Scan(&draft)
		if err == nil {
			return nil
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if v.Revision != in.Revision {
			return fault("revision_conflict", "Reload before saving this version.")
		}
		var busy bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('QUEUED','RUNNING','FINALIZING','CANCELLING','AWAITING_APPROVAL'))", id).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return fault("operation_running", "Finish or stop the current run before attaching a workspace.")
		}
		var artifact *Artifact
		for i := range v.Document.Artifacts {
			if v.Document.Artifacts[i].ID == in.ArtifactID {
				artifact = &v.Document.Artifacts[i]
			}
		}
		if artifact == nil || len(artifact.Blueprint) == 0 {
			return fault("artifact_required", "Choose an exact pack version.")
		}
		compiler, ok := s.Compiler.(interface {
			Advanced(json.RawMessage, Limits) (json.RawMessage, error)
		})
		if !ok {
			return fault("unsupported_capability", "Advanced pack conversion is unavailable. Download the original file.")
		}
		composition, err := compiler.Advanced(artifact.Blueprint, s.Config.Limits(v.Anonymous))
		if err != nil {
			return fault("unsupported_capability", "This source cannot be edited losslessly in the current builder. Download its original file. "+err.Error())
		}
		var c challengepack.Composition
		if err = json.Unmarshal(composition, &c); err != nil {
			return err
		}
		mode := c.Version.ExecutionMode
		if mode == "" {
			mode = "native"
		}
		draft = uuid.New()
		if _, err = tx.Exec(ctx, "INSERT INTO challenge_pack_drafts(id,workspace_id,name,execution_mode,composition,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6)", draft, in.WorkspaceID, c.Pack.Name, mode, composition, uid); err != nil {
			return err
		}
		if err = packSource(ctx, tx, v, draft, in.WorkspaceID, *artifact, composition, true); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE vibe_sessions SET workspace_id=$2,revision=revision+1,updated_at=now() WHERE id=$1", id, in.WorkspaceID); err != nil {
			return err
		}
		return event(ctx, tx, id, nil, "advanced.saved")
	})
	return draft, err
}
func (s *Store) SavedOriginal(ctx context.Context, actor string, draft uuid.UUID) (ImportOriginal, error) {
	var out ImportOriginal
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var ws uuid.UUID
		err := tx.QueryRow(ctx, "SELECT workspace_id FROM vibe_pack_sources WHERE draft_id=$1", draft).Scan(&ws)
		if err == pgx.ErrNoRows {
			return fault("not_found", "Original copy is unavailable.")
		}
		if err != nil {
			return err
		}
		if err = authorize(ctx, tx, actor, &ws, false); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, "SELECT artifact_id,original_filename,original_media_type,original_hash,original_content FROM vibe_pack_sources WHERE draft_id=$1 AND original_content IS NOT NULL", draft).Scan(&out.ArtifactID, &out.Filename, &out.MediaType, &out.Hash, &out.Content)
		if err == pgx.ErrNoRows {
			return fault("not_found", "This draft has no uploaded original.")
		}
		out.ID = draft
		out.Size = len(out.Content)
		return err
	})
	return out, err
}
