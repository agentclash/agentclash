package vibe

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (p Plan) isRegrade() bool { return p.RegradeOf != nil || p.RegradeSource != nil }
func (s *Service) prepareSDKRegrade(ctx context.Context, actor string, v Session, sub Submission, p Plan) (Operation, error) {
	if !s.Config.GroundedJudging {
		return Operation{}, fault("regrade_unavailable", "Hosted grading is unavailable.")
	}
	if sub.RegradeSourceResultRef == nil || sub.RegradeSourceResultRef.Kind != ResultSDKReport || sub.RegradeSourceResultRef.ID == uuid.Nil || sub.BaselineID != nil || sub.ArtifactID == nil || sub.SourceEvidenceHash == "" || sub.RunQuoteID == nil && !sub.estimateOnly {
		return Operation{}, fault("invalid_request", "Choose an exact SDK report and request a grading estimate first.")
	}
	report, err := s.Store.SDKReport(ctx, actor, v.ID, sub.RegradeSourceResultRef.ID)
	if err != nil {
		return Operation{}, err
	}
	if report.ContentHash != sub.SourceEvidenceHash || report.Binding.ArtifactID != *sub.ArtifactID {
		return Operation{}, fault("evidence_changed", "Choose the exact imported report and tested version.")
	}
	a, err := sdkArtifact(v, *sub.ArtifactID)
	if err != nil {
		return Operation{}, err
	}
	p.Artifact = &a
	p.AuthoringVersion = 0
	p.Document = Document{}
	p.RegradeSource = &ResultRecovery{Source: report.Binding.ResultRef, ArtifactID: a.ID, EvidenceHash: report.ContentHash}
	p.Source = &EvaluationSource{Kind: "prompt", ArtifactID: a.ID, Label: "Imported SDK replies (unverified application execution)", Comparison: "regraded"}
	l := p.limits()
	compiled, err := s.Compiler.Compile(a.Blueprint, sub.Models.Evaluator, a.ID, l)
	if err != nil {
		return Operation{}, fault("regrade_unavailable", "These tests require evidence not supported by hosted text regrading. The original report is unchanged.")
	}
	judges := len(compiled.Bundle.Version.EvaluationSpec.LLMJudges)
	p.ChecksPerCase = judges + len(compiled.Bundle.Version.EvaluationSpec.Validators)
	seen := map[string]bool{}
	for _, binding := range report.Cases {
		if seen[binding.CaseKey] {
			return Operation{}, fault("regrade_unavailable", "Repeated case keys require source-specific execution support. The original report is unchanged.")
		}
		seen[binding.CaseKey] = true
		c, e := SDKReportCase(report, binding.SourceCaseID, binding.CaseKey)
		if e != nil {
			return Operation{}, e
		}
		if c.Output == "" {
			return Operation{}, fault("evidence_unavailable", "Every selected imported case needs a retained final reply for hosted text regrading.")
		}
		p.Cases = append(p.Cases, c.CaseKey)
		p.SavedResults = append(p.SavedResults, CaseResult{CaseKey: c.CaseKey, Version: c.Version, Output: c.Output})
		p.CasePreviews = append(p.CasePreviews, c)
	}
	if len(p.Cases) == 0 || len(p.Cases) > l.Cases || judges == 0 {
		return Operation{}, fault("regrade_unavailable", "This report has no supported bounded set of replies and AI checks to regrade.")
	}
	if _, err = selectCompiledCases(compiled.Cases, p.Cases); err != nil {
		return Operation{}, err
	}
	p.Calls = len(p.Cases) * judges
	if p.Calls > l.ModelCalls {
		return Operation{}, fault("operation_limit", "This regrade exceeds the call allowance.")
	}
	profile, err := s.Config.Profile(sub.Models.Evaluator)
	if err != nil {
		return Operation{}, err
	}
	cost, err := profile.BoundCost(profile.inputLimit(l), l.OutputTokens)
	if err != nil {
		return Operation{}, err
	}
	p.MaxCost = cost * int64(p.Calls)
	if err = s.freezeGrading(&p); err != nil {
		return Operation{}, err
	}
	freezeResultEvidence(&p)
	p.Provenance.Origin = OriginRegraded
	p.Provenance.SourceResultRef = &report.Binding.ResultRef
	// No new application execution was observed. Only the new grading/test receipt
	// is verified; execution fingerprint claims remain attached to the SDK source.
	p.Provenance.Fingerprints.Model = nil
	p.Provenance.Fingerprints.Build = nil
	p.Provenance.Fingerprints.Config = nil
	p.Provenance.CaptureScope = UnknownCaptureScope()
	return s.Store.Submit(ctx, actor, v.ID, sub, p, s.Config)
}
func validateSDKRegradeAdmission(ctx context.Context, tx pgx.Tx, v Session, p Plan) error {
	if p.RegradeSource == nil {
		return nil
	}
	if p.Submission.Purpose != "regrade" || p.Submission.RegradeSourceResultRef == nil || *p.Submission.RegradeSourceResultRef != p.RegradeSource.Source {
		return fault("invalid_request", "Invalid SDK regrade source.")
	}
	r, err := sdkReportTx(ctx, tx, v.ID, p.RegradeSource.Source.ID)
	if err != nil {
		return err
	}
	if r.ContentHash != p.RegradeSource.EvidenceHash || r.Binding.ArtifactID != p.RegradeSource.ArtifactID || p.Artifact == nil || p.Artifact.ID != r.Binding.ArtifactID {
		return fault("evidence_changed", "The selected SDK regrade source is unavailable.")
	}
	return nil
}
