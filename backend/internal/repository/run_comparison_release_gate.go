package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RunComparisonReleaseGate struct {
	ID                uuid.UUID
	RunComparisonID   uuid.UUID
	PolicyKey         string
	PolicyVersion     int
	PolicyFingerprint string
	PolicySnapshot    json.RawMessage
	Verdict           string
	ReasonCode        string
	Summary           string
	EvidenceStatus    string
	EvaluationDetails json.RawMessage
	SourceFingerprint string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type UpsertRunComparisonReleaseGateParams struct {
	RunComparisonID   uuid.UUID
	PolicyKey         string
	PolicyVersion     int
	PolicyFingerprint string
	PolicySnapshot    json.RawMessage
	Verdict           string
	ReasonCode        string
	Summary           string
	EvidenceStatus    string
	EvaluationDetails json.RawMessage
	SourceFingerprint string
}

func (r *Repository) UpsertRunComparisonReleaseGate(ctx context.Context, params UpsertRunComparisonReleaseGateParams) (RunComparisonReleaseGate, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO run_comparison_release_gates (
			run_comparison_id,
			policy_key,
			policy_version,
			policy_fingerprint,
			policy_snapshot,
			verdict,
			reason_code,
			summary,
			evidence_status,
			evaluation_details,
			source_fingerprint
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (run_comparison_id, policy_key, policy_version, policy_fingerprint)
		DO UPDATE SET
			policy_snapshot = EXCLUDED.policy_snapshot,
			verdict = EXCLUDED.verdict,
			reason_code = EXCLUDED.reason_code,
			summary = EXCLUDED.summary,
			evidence_status = EXCLUDED.evidence_status,
			evaluation_details = EXCLUDED.evaluation_details,
			source_fingerprint = EXCLUDED.source_fingerprint
		RETURNING
			id,
			run_comparison_id,
			policy_key,
			policy_version,
			policy_fingerprint,
			policy_snapshot,
			verdict,
			reason_code,
			summary,
			evidence_status,
			evaluation_details,
			source_fingerprint,
			created_at,
			updated_at
	`,
		params.RunComparisonID,
		params.PolicyKey,
		params.PolicyVersion,
		params.PolicyFingerprint,
		params.PolicySnapshot,
		params.Verdict,
		params.ReasonCode,
		params.Summary,
		params.EvidenceStatus,
		params.EvaluationDetails,
		params.SourceFingerprint,
	)

	var record RunComparisonReleaseGate
	if err := row.Scan(
		&record.ID,
		&record.RunComparisonID,
		&record.PolicyKey,
		&record.PolicyVersion,
		&record.PolicyFingerprint,
		&record.PolicySnapshot,
		&record.Verdict,
		&record.ReasonCode,
		&record.Summary,
		&record.EvidenceStatus,
		&record.EvaluationDetails,
		&record.SourceFingerprint,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return RunComparisonReleaseGate{}, fmt.Errorf("upsert run comparison release gate: %w", err)
	}

	return record, nil
}

func (r *Repository) ListRunComparisonReleaseGates(ctx context.Context, runComparisonID uuid.UUID) ([]RunComparisonReleaseGate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			run_comparison_id,
			policy_key,
			policy_version,
			policy_fingerprint,
			policy_snapshot,
			verdict,
			reason_code,
			summary,
			evidence_status,
			evaluation_details,
			source_fingerprint,
			created_at,
			updated_at
		FROM run_comparison_release_gates
		WHERE run_comparison_id = $1
		ORDER BY updated_at DESC, policy_key ASC, policy_version ASC
	`, runComparisonID)
	if err != nil {
		return nil, fmt.Errorf("list run comparison release gates: %w", err)
	}
	defer rows.Close()

	result := make([]RunComparisonReleaseGate, 0)
	for rows.Next() {
		var record RunComparisonReleaseGate
		if err := rows.Scan(
			&record.ID,
			&record.RunComparisonID,
			&record.PolicyKey,
			&record.PolicyVersion,
			&record.PolicyFingerprint,
			&record.PolicySnapshot,
			&record.Verdict,
			&record.ReasonCode,
			&record.Summary,
			&record.EvidenceStatus,
			&record.EvaluationDetails,
			&record.SourceFingerprint,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan run comparison release gate: %w", err)
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run comparison release gates: %w", err)
	}
	return result, nil
}
