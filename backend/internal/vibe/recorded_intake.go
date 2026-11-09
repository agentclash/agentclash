package vibe

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type EvidenceIntakeReceipt struct {
	ClientID   uuid.UUID `json:"client_id"`
	EvidenceID uuid.UUID `json:"evidence_id"`
	ArtifactID uuid.UUID `json:"artifact_id"`
}

func (s *Store) RecordedEvidenceReceipt(ctx context.Context, actor string, session, client uuid.UUID) (EvidenceIntakeReceipt, error) {
	var r EvidenceIntakeReceipt
	if _, err := s.GetSession(ctx, actor, session); err != nil {
		return r, err
	}
	r.ClientID = client
	err := s.DB.QueryRow(ctx, "SELECT evidence_id,artifact_id FROM vibe_evidence_receipts WHERE session_id=$1 AND client_id=$2", session, client).Scan(&r.EvidenceID, &r.ArtifactID)
	return r, err
}

// Current projects require explicit expectations. Intake records evidence only;
// evaluator execution uses the ordinary quoted check admission path.
func (s *Service) addRecordedEvidence(ctx context.Context, actor string, initial Session, in EvidenceInput) error {
	if in.ClientID == nil || *in.ClientID == uuid.Nil {
		return fault("invalid_evidence", "A request ID is required to save recorded replies safely.")
	}
	l := s.Config.Limits(initial.Anonymous)
	if err := s.Gate.Check(ctx, actor, l); err != nil {
		return err
	}
	if err := validateExpectations(in.Expectations, l); err != nil {
		return err
	}
	hash := Hash(raw(in))
	return s.Store.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", initial.ID))
		if err != nil {
			return err
		}
		if v.Actor != actor {
			return fault("not_found", "Project is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		var previous string
		err = tx.QueryRow(ctx, "SELECT request_hash FROM vibe_evidence_receipts WHERE session_id=$1 AND client_id=$2", v.ID, *in.ClientID).Scan(&previous)
		if err == nil {
			if previous != hash {
				return fault("idempotency_conflict", "This request ID already saved different replies or expectations.")
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if v.Revision != in.Revision {
			return fault("revision_conflict", "Reload the project before saving these replies.")
		}
		var busy bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state NOT IN ('COMPLETED','PARTIAL','FAILED','CANCELLED','EXPIRED'))", v.ID).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return fault("operation_running", "Wait for the current response or stop it before editing.")
		}
		var e EvidenceSet
		if in.ParentID != nil {
			old := findEvidence(v.Document, in.ParentID)
			if old == nil {
				return fault("not_found", "The original replies are unavailable.")
			}
			e = *old
			e.ID = uuid.New()
			e.ParentID = in.ParentID
			e.CreatedAt = timestamp()
			// Deep copy before applying corrections; original messages are immutable.
			e.Conversations = append([]EvidenceConversation(nil), old.Conversations...)
			for i := range e.Conversations {
				e.Conversations[i].Messages = append([]EvidenceMessage(nil), old.Conversations[i].Messages...)
			}
		} else {
			e, err = ParseEvidence(in.Content, in.Label, l)
			if err != nil {
				return err
			}
		}
		for id, role := range in.Roles {
			if role != "user" && role != "assistant" && role != "system" && role != "tool" {
				return fault("invalid_evidence", "Choose a recognized speaker role.")
			}
			found := false
			for i := range e.Conversations {
				for j := range e.Conversations[i].Messages {
					m := &e.Conversations[i].Messages[j]
					if m.ID == id {
						m.Role = role
						found = true
					}
				}
			}
			if !found {
				return fault("invalid_evidence", "The speaker correction refers to a missing message.")
			}
		}
		if err = e.ValidateReady(); err != nil {
			return err
		}
		if len(v.Document.Artifacts) >= MaxRevisions {
			return fault("document_limit", "This project has too many saved versions.")
		}
		if err = appendEvidence(&v, e, l, s.Config.TestingLocally()); err != nil {
			return err
		}
		a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", Title: e.Label, CreatedAt: timestamp(), Provenance: "user_provided", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: in.Expectations}}
		v.Document.Artifacts = append(v.Document.Artifacts, a)
		if err = s.Store.updateDocument(ctx, tx, v); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO vibe_evidence_receipts(session_id,client_id,request_hash,evidence_id,artifact_id) VALUES($1,$2,$3,$4,$5)", v.ID, *in.ClientID, hash, e.ID, a.ID); err != nil {
			return err
		}
		return event(ctx, tx, v.ID, nil, "evidence.created")
	})
}
