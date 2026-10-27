package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func appendSDKExport(ctx context.Context, tx pgx.Tx, out *ExportPreparation) error {
	in := out.Selection
	rows, err := tx.Query(ctx, `SELECT body FROM vibe_sdk_reports WHERE session_id=$1 AND ($2 OR id=$3) ORDER BY created_at,id LIMIT 101`, out.SessionID, in.Scope == "history", sdkExportID(in.ResultRef))
	if err != nil {
		return err
	}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			break
		}
		var report SDKReport
		if err = json.Unmarshal(b, &report); err != nil {
			break
		}
		if in.Scope == "selected" && report.Binding.ArtifactID != in.ArtifactID {
			err = fault("invalid_request", "SDK result belongs to different tests.")
			break
		}
		out.SDKReports = append(out.SDKReports, report)
		ref := report.Binding.ResultRef
		a := report.Binding.ArtifactID
		out.Items = append(out.Items, ExportItem{Kind: "sdk_report", ID: report.ID.String(), ResultRef: &ref, ArtifactID: &a, Hash: report.ContentHash})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if len(out.SDKReports) > SDKReportCount {
		return fault("export_limit", "Too many SDK reports for this export.")
	}
	if in.Scope == "selected" && in.ResultRef != nil && in.ResultRef.Kind == ResultSDKReport && len(out.SDKReports) != 1 {
		return fault("not_found", "SDK result is unavailable in this project.")
	}
	for _, r := range out.SDKReports {
		reviews, e := tx.Query(ctx, `SELECT id,body FROM vibe_grade_reviews WHERE session_id=$1 AND result_kind='sdk_report' AND result_id=$2 ORDER BY created_at,id LIMIT 5001`, out.SessionID, r.ID)
		if e != nil {
			return e
		}
		for reviews.Next() {
			var id uuid.UUID
			var b []byte
			if e = reviews.Scan(&id, &b); e != nil {
				break
			}
			var review GradeReview
			if e = json.Unmarshal(b, &review); e != nil {
				break
			}
			ref := r.Binding.ResultRef
			artifact := review.ArtifactID
			changed := false
			out.Items = append(out.Items, ExportItem{Kind: "grade_review", ID: id.String(), Hash: Hash(raw(review)), ResultRef: &ref, ArtifactID: &artifact, EvidenceChanged: &changed})
			if len(out.Items) > 5000 {
				e = fault("export_limit", "This history exceeds one export.")
				break
			}
		}
		if e == nil {
			e = reviews.Err()
		}
		reviews.Close()
		if e != nil {
			return e
		}
	}
	return nil
}
func sdkExportID(ref *ResultRef) *uuid.UUID {
	if ref != nil && ref.Kind == ResultSDKReport {
		return &ref.ID
	}
	return nil
}
