package vibe

import "github.com/google/uuid"

// Result identity, provenance and capture are independent. A reference never
// grants access: each source resolver authorizes its original evidence.
type ResultOrigin string

const (
	OriginPrototype ResultOrigin = "prototype"
	OriginRecorded  ResultOrigin = "recorded"
	OriginImported  ResultOrigin = "imported"
	OriginLive      ResultOrigin = "live"
	OriginRegraded  ResultOrigin = "regraded"
)

type Fingerprints struct {
	Model  *string `json:"model"`
	Build  *string `json:"build"`
	Test   *string `json:"test"`
	Judge  *string `json:"judge"`
	Config *string `json:"config"`
}
type DimensionCapture struct {
	Scope           string `json:"scope"`
	Instrumentation string `json:"instrumentation"`
}
type CaptureScope struct {
	Messages  DimensionCapture `json:"messages"`
	Tools     DimensionCapture `json:"tools"`
	Retrieval DimensionCapture `json:"retrieval"`
}
type EvidenceProvenance struct {
	Origin              ResultOrigin `json:"origin"`
	Fingerprints        Fingerprints `json:"fingerprints"`
	CaptureScope        CaptureScope `json:"capture_scope"`
	ApplicationRevision *string      `json:"application_revision"`
	SourceResultRef     *ResultRef   `json:"source_result_ref,omitempty"`
}
type EvidenceMetadata struct {
	ResultRef ResultRef `json:"result_ref"`
	EvidenceProvenance
}
type ProjectResultBinding struct {
	SessionID              uuid.UUID  `json:"session_id"`
	ArtifactID             uuid.UUID  `json:"artifact_id"`
	ResultRef              ResultRef  `json:"result_ref"`
	ChallengePackVersionID *uuid.UUID `json:"challenge_pack_version_id,omitempty"`
}
type CaseEvidenceBinding struct {
	CaseKey      string     `json:"case_key"`
	SourceCaseID string     `json:"source_case_id"`
	RunAgentID   *uuid.UUID `json:"run_agent_id,omitempty"`
}
type ResultScope struct {
	ArtifactID           uuid.UUID  `json:"artifact_id"`
	CaseKeys             []string   `json:"case_keys"`
	RetrySourceResultRef *ResultRef `json:"retry_source_result_ref,omitempty"`
}

func UnknownCaptureScope() CaptureScope {
	d := DimensionCapture{Scope: "unknown", Instrumentation: "unknown"}
	return CaptureScope{Messages: d, Tools: d, Retrieval: d}
}
func (c CaptureScope) Valid() bool {
	for _, d := range []DimensionCapture{c.Messages, c.Tools, c.Retrieval} {
		if d.Scope != "unknown" && d.Scope != "partial" && d.Scope != "complete" {
			return false
		}
		if d.Instrumentation != "unknown" && d.Instrumentation != "not_instrumented" && d.Instrumentation != "instrumented" {
			return false
		}
		if d.Instrumentation == "not_instrumented" && d.Scope != "unknown" {
			return false
		}
		if d.Scope == "complete" && d.Instrumentation != "instrumented" {
			return false
		}
	}
	return true
}

// Hash inputs use Go's deterministic JSON encoding; every new hash explicitly
// carries its domain. Existing grading/target receipts remain available verbatim.
func evidenceFingerprint(domain string, value any) *string {
	v := "vibe:" + domain + ":v1:sha256:" + Hash(raw(value))
	return &v
}
func freezeResultEvidence(p *Plan) {
	if p.Artifact == nil || p.Submission.Kind != "check" && p.Submission.Kind != "retest" {
		return
	}
	if p.Provenance != nil {
		return
	}
	provenance := EvidenceProvenance{Origin: OriginPrototype, CaptureScope: UnknownCaptureScope()}
	if p.Evidence != nil || p.Source != nil && p.Source.Kind == "provided_conversations" {
		provenance.Origin = OriginRecorded
	}
	if p.RegradeOf != nil {
		provenance.Origin = OriginRegraded
		provenance.SourceResultRef = &ResultRef{Kind: "vibe_operation", ID: *p.RegradeOf}
	}
	if p.TargetConfig != nil {
		provenance.Fingerprints.Model = evidenceFingerprint("model-execution", p.TargetConfig.ModelExecution)
		provenance.Fingerprints.Build = evidenceFingerprint("build", struct {
			Instructions string
			References   any
		}{p.TargetConfig.InstructionsHash, p.Artifact.ReferenceInputs})
	}
	if p.Grading != nil && gradingSupported(p.Grading) {
		provenance.Fingerprints.Test = evidenceFingerprint("tests", struct {
			Tests, Criteria, References string
			Cases                       []string
		}{p.Grading.TestsHash, p.Grading.CriteriaHash, p.Grading.ReferenceHash, p.Cases})
		provenance.Fingerprints.Judge = evidenceFingerprint("judge", p.Grading.Hash)
		// Recorded source execution settings are unknown even when its judge is known.
		if p.TargetConfig != nil {
			provenance.Fingerprints.Config = evidenceFingerprint("execution-config", struct {
				Target    ModelExecution
				Evaluator ModelExecution
				Limits    Limits
			}{p.TargetConfig.ModelExecution, p.Grading.Evaluator, p.limits()})
		}
	}
	p.Provenance = &provenance
}
func populateResultEvidence(o *Operation, p Plan) {
	if p.Artifact == nil || p.Artifact.ID == uuid.Nil || p.Submission.Kind != "check" && p.Submission.Kind != "retest" {
		return
	}
	// Summary reads intentionally omit source material and execution limits.
	// An old plan without a frozen producer record cannot safely reconstruct
	// build/config identity from that projection or from today's defaults.
	if p.Provenance == nil {
		p.Provenance = &EvidenceProvenance{Origin: OriginPrototype, CaptureScope: UnknownCaptureScope()}
		if p.Source != nil && p.Source.Kind == "provided_conversations" {
			p.Provenance.Origin = OriginRecorded
		}
		if p.RegradeOf != nil {
			p.Provenance.Origin = OriginRegraded
			p.Provenance.SourceResultRef = &ResultRef{Kind: ResultVibeOperation, ID: *p.RegradeOf}
		}
	}
	o.EvidenceMetadata = &EvidenceMetadata{ResultRef: ResultRef{Kind: "vibe_operation", ID: o.ID}, EvidenceProvenance: *p.Provenance}
	o.ResultScope = &ResultScope{ArtifactID: p.Artifact.ID, CaseKeys: append([]string{}, p.Cases...), RetrySourceResultRef: p.Submission.RetrySourceResultRef}
}
