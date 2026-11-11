package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ChallengePackDraftStatus is the status of one challenge_pack_drafts row.
type ChallengePackDraftStatus string

const (
	// ChallengePackDraftStatusDraft is the first status. Only a draft in this
	// status takes edits and can publish.
	ChallengePackDraftStatusDraft ChallengePackDraftStatus = "draft"
	// ChallengePackDraftStatusPublished means that the draft published one
	// pack version. It is final: to change the pack, make a new draft from
	// that version (edit-in-builder).
	ChallengePackDraftStatusPublished ChallengePackDraftStatus = "published"
	// The DB CHECK also permits "discarded". No writer sets it, so it is not
	// in the transition table.
)

// challengePackDraftTransitions names each status change that a writer can
// make.
//
// Writers:
//   - PublishChallengePackBundle (with a source draft) and
//     MarkChallengePackDraftPublished: draft -> published.
var challengePackDraftTransitions = map[ChallengePackDraftStatus]map[ChallengePackDraftStatus]struct{}{
	ChallengePackDraftStatusDraft: {
		ChallengePackDraftStatusPublished: {},
	},
	ChallengePackDraftStatusPublished: {},
}

func ParseChallengePackDraftStatus(raw string) (ChallengePackDraftStatus, error) {
	status := ChallengePackDraftStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("invalid challenge pack draft status: %q", raw)
	}
	return status, nil
}

func (s ChallengePackDraftStatus) Valid() bool {
	_, ok := challengePackDraftTransitions[s]
	return ok
}

func (s ChallengePackDraftStatus) CanTransitionTo(next ChallengePackDraftStatus) bool {
	nextStatuses, ok := challengePackDraftTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

// Terminal reports whether the status is final.
func (s ChallengePackDraftStatus) Terminal() bool {
	nextStatuses, ok := challengePackDraftTransitions[s]
	return ok && len(nextStatuses) == 0
}

// Editable reports whether a draft in this status takes changes to its name,
// execution mode or composition. Only a draft that is not final takes them.
// A status that is not in the table is not editable.
func (s ChallengePackDraftStatus) Editable() bool {
	return s.Valid() && !s.Terminal()
}

// ChallengePackDraftFinalError reports an edit or a publish of a draft whose
// status does not permit it. It matches ErrInvalidTransition, so the API
// returns HTTP 409.
type ChallengePackDraftFinalError struct {
	ID     uuid.UUID
	Status ChallengePackDraftStatus
}

func (e ChallengePackDraftFinalError) Error() string {
	return fmt.Sprintf("challenge pack draft %s is %s and cannot change", e.ID, e.Status)
}

func (e ChallengePackDraftFinalError) Is(target error) bool {
	return target == ErrInvalidTransition
}

// lockPublishableChallengePackDraftTx locks the source draft of a publish at
// the start of the publish transaction. A second publish of the same draft
// waits for the first, then fails here before it writes a pack version.
func lockPublishableChallengePackDraftTx(ctx context.Context, tx pgx.Tx, draftID, workspaceID uuid.UUID) error {
	var raw string
	err := tx.QueryRow(ctx, `
		SELECT status
		FROM challenge_pack_drafts
		WHERE id = $1
		  AND workspace_id = $2
		FOR UPDATE
	`, draftID, workspaceID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrChallengePackDraftNotFound
		}
		return fmt.Errorf("lock challenge pack draft: %w", err)
	}
	status := ChallengePackDraftStatus(raw)
	if !status.CanTransitionTo(ChallengePackDraftStatusPublished) {
		return ChallengePackDraftFinalError{ID: draftID, Status: status}
	}
	return nil
}

// markChallengePackDraftPublishedTx links the source draft to the version it
// just published, in the publish transaction, so a pack version never exists
// without its draft bookkeeping. The UPDATE guards on the draft status, so a
// draft publishes once. Call lockPublishableChallengePackDraftTx first in the
// same transaction.
func markChallengePackDraftPublishedTx(ctx context.Context, tx pgx.Tx, draftID, workspaceID, packID, versionID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE challenge_pack_drafts
		SET status = 'published',
			last_published_version_id = $3,
			challenge_pack_id = $4
		WHERE id = $1
		  AND workspace_id = $2
		  AND status = 'draft'
	`, draftID, workspaceID, versionID, packID)
	if err != nil {
		return fmt.Errorf("mark challenge pack draft published: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// The lock at the start of the transaction makes this unreachable.
		// Report it as a lost transition, not as a missing draft.
		return TransitionConflictError{
			Entity:   "challenge pack draft",
			ID:       draftID,
			Expected: string(ChallengePackDraftStatusDraft),
		}
	}
	return nil
}

type MarkChallengePackDraftPublishedParams struct {
	DraftID                uuid.UUID
	WorkspaceID            uuid.UUID
	ChallengePackID        uuid.UUID
	ChallengePackVersionID uuid.UUID
}

// MarkChallengePackDraftPublished links a draft to a version that a publisher
// wrote outside of PublishChallengePackBundle. It uses the same guards as the
// publish transaction, so it refuses a draft that is already published.
func (r *Repository) MarkChallengePackDraftPublished(ctx context.Context, params MarkChallengePackDraftPublishedParams) (ChallengePackDraft, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ChallengePackDraft{}, fmt.Errorf("begin mark challenge pack draft published transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := lockPublishableChallengePackDraftTx(ctx, tx, params.DraftID, params.WorkspaceID); err != nil {
		return ChallengePackDraft{}, err
	}
	if err := markChallengePackDraftPublishedTx(ctx, tx, params.DraftID, params.WorkspaceID, params.ChallengePackID, params.ChallengePackVersionID); err != nil {
		return ChallengePackDraft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ChallengePackDraft{}, fmt.Errorf("commit mark challenge pack draft published: %w", err)
	}
	return r.GetChallengePackDraftByID(ctx, params.DraftID)
}
