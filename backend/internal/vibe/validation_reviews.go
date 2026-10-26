package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type ValidationReview struct {
	ID                  uuid.UUID         `json:"id"`
	OperationID         uuid.UUID         `json:"operation_id"`
	CandidateArtifactID uuid.UUID         `json:"candidate_artifact_id"`
	PriorArtifactID     *uuid.UUID        `json:"prior_artifact_id,omitempty"`
	Stage               string            `json:"stage"`
	Validation          SuiteValidation   `json:"validation"`
	Cases               []SuiteReviewCase `json:"cases"`
	Rules               []PolicyRule      `json:"rules"`
	Error               *Fault            `json:"error,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
}

func (s *Store) persistValidationReview(ctx context.Context, o Operation, stage string, a *Artifact, input SuiteReviewInput, validation *SuiteValidation, issue error) error {
	// Provider completion may arrive with a canceled request context. Persist only
	// bounded diagnostics, never a newly accepted artifact, through this path.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	review := ValidationReview{ID: deterministicID(o.ID, "validation:"+stage+":"+input.BlueprintHash+":"+input.PolicyHash), OperationID: o.ID, CandidateArtifactID: a.ID, PriorArtifactID: a.ParentID, Stage: stage, Cases: input.Cases, Rules: input.Policy.Rules, CreatedAt: operationTime(o)}
	if validation != nil && issue == nil {
		review.Validation = *validation
	} else {
		review.Validation = SuiteValidation{Status: SuiteUnavailable, BlueprintHash: input.BlueprintHash, PolicyHash: input.PolicyHash, ValidatorVersion: input.ValidatorVersion, Cases: []SuiteCaseReview{}, Rules: []SuiteRuleReview{}}
		if review.Validation.ValidatorVersion == "" {
			review.Validation.ValidatorVersion = SuiteValidatorVersion
		}
		review.Error = &Fault{Code: "validation_unavailable", Message: "The proposed tests could not be validated. Your working version is unchanged."}
	}
	oversized := len(raw(review)) > 128<<10
	if oversized {
		review.Cases = nil
		review.Rules = nil
		review.Validation = SuiteValidation{Status: SuiteUnavailable, BlueprintHash: input.BlueprintHash, PolicyHash: input.PolicyHash, ValidatorVersion: input.ValidatorVersion, Cases: []SuiteCaseReview{}, Rules: []SuiteRuleReview{}}
		review.Error = &Fault{Code: "validation_unavailable", Message: "The review exceeds the bounded display limit. Your working version is unchanged."}
	}
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		locked, err := lockOperation(ctx, tx, o.ID, projectWrite)
		if err != nil {
			return err
		}
		if _, err = scanSession(tx.QueryRow(ctx, sessionSelect, locked.SessionID)); err != nil {
			return err
		}
		var previous []byte
		err = tx.QueryRow(ctx, "SELECT body FROM vibe_validation_reviews WHERE id=$1", review.ID).Scan(&previous)
		if err == nil {
			if Hash(previous) != Hash(raw(review)) {
				var stored ValidationReview
				if e := json.Unmarshal(previous, &stored); e != nil {
					return e
				}
				if Hash(raw(stored)) != Hash(raw(review)) {
					return fault("idempotency_conflict", "The saved validation review differs from this replay.")
				}
			}
			return nil
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO vibe_validation_reviews(id,session_id,operation_id,stage_key,candidate_hash,policy_hash,body) VALUES($1,$2,$3,$4,$5,$6,$7)", review.ID, locked.SessionID, o.ID, stage, input.BlueprintHash, input.PolicyHash, raw(review)); err != nil {
			return err
		}
		return event(ctx, tx, locked.SessionID, &o.ID, "validation.reviewed")
	})
	if err != nil {
		return err
	}
	if oversized {
		return review.Error
	}
	return nil
}

func populateValidationReviews(ctx context.Context, tx pgx.Tx, v *Session) error {
	v.ValidationReviews = []ValidationReview{}
	rows, err := tx.Query(ctx, "SELECT body FROM vibe_validation_reviews WHERE session_id=$1 ORDER BY created_at DESC,id DESC LIMIT 10", v.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return err
		}
		var review ValidationReview
		if err = json.Unmarshal(data, &review); err != nil {
			return err
		}
		v.ValidationReviews = append(v.ValidationReviews, review)
	}
	return rows.Err()
}
