package vibe

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ArchiveSource struct {
	SessionID  uuid.UUID `json:"session_id"`
	ArtifactID uuid.UUID `json:"artifact_id"`
}

// ContinueArchive adopts only the selected immutable pack and instructions.
// Old dialogue, grades, confirmations and operation receipts remain archived.
func (s *Service) ContinueArchive(ctx context.Context, actor string, sourceID, artifactID, clientID uuid.UUID) (Session, error) {
	if clientID == uuid.Nil {
		return Session{}, fault("invalid_request", "A copy request ID is required.")
	}
	var result Session
	err := s.Store.transaction(ctx, func(tx pgx.Tx) error {
		old, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", sourceID))
		if err != nil || old.Actor != actor {
			return fault("not_found", "The original conversation is unavailable.")
		}
		if err = authorize(ctx, tx, actor, old.WorkspaceID, false); err != nil {
			return err
		}
		if old.Document.FormatVersion == 1 {
			return fault("invalid_request", "This agent already uses V1.")
		}
		var selected *Artifact
		for _, a := range old.Document.Artifacts {
			if a.ID == artifactID {
				copy := a
				selected = &copy
				break
			}
		}
		if selected == nil || !selected.IsTestSuite() {
			return fault("invalid_request", "Choose a compatible test pack to continue. Recorded replies and planning-only drafts can still be exported.")
		}
		id := deterministicID(sourceID, "continue-v1:"+clientID.String())
		if _, err = s.Compiler.Compile(selected.Blueprint, old.Document.Models.Evaluator, id, s.Config.Limits(old.Anonymous)); err != nil {
			return err
		}
		source := &ArchiveSource{SessionID: sourceID, ArtifactID: artifactID}
		copied := Artifact{ID: deterministicID(id, "artifact"), Kind: "test_suite", Title: selected.Title,
			Summary:     "Copied from your saved conversation. Run it here for new results.",
			AgentPrompt: selected.AgentPrompt, Blueprint: selected.Blueprint, Sample: selected.Sample,
			ScopeNote: prototypeScope, Provenance: "imported", CreatedAt: timestamp()}
		d := Document{FormatVersion: 1, TestJourney: true, ContinuedFrom: source, Evaluation: &EvaluationContext{ID: id, ChatID: id, Door: "test"},
			Models: old.Document.Models, Messages: []Message{}, Requirements: []Requirement{}, Artifacts: []Artifact{copied}, ActiveArtifactID: &copied.ID}
		result, err = s.Store.createSessionTx(ctx, tx, actor, old.WorkspaceID, id, d)
		if err == nil && (result.Document.ContinuedFrom == nil || *result.Document.ContinuedFrom != *source) {
			return fault("idempotency_conflict", "That copy request already selected a different version.")
		}
		return err
	})
	return result, err
}
