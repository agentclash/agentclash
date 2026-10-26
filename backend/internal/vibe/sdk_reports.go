package vibe

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
)

const SDKReportSourceBytes = 1 << 20
const SDKReportMetadataBytes = 64 << 10
const SDKReportRequestBytes = 6*SDKReportSourceBytes + SDKReportMetadataBytes + 4096
const SDKReportCount = 100

//go:embed sdk_schema/eval-report.schema.json
var sdkReportSchema []byte

//go:embed sdk_schema/agent-result.schema.json
var sdkAgentSchema []byte
var sdkSchemas = sync.OnceValues(func() (map[string]*jsonschema.Resolved, error) {
	out := map[string]*jsonschema.Resolved{}
	for k, b := range map[string][]byte{"eval_report": sdkReportSchema, "agent_result": sdkAgentSchema} {
		var s jsonschema.Schema
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, err
		}
		r, err := s.Resolve(nil)
		if err != nil {
			return nil, err
		}
		out[k] = r
	}
	return out, nil
})

type SDKCaseBinding struct {
	SourceCaseID    string        `json:"source_case_id"`
	CaseKey         string        `json:"case_key"`
	InputSetKey     *string       `json:"input_set_key,omitempty"`
	ChallengeKey    *string       `json:"challenge_key,omitempty"`
	PublishedCaseID *uuid.UUID    `json:"published_case_id,omitempty"`
	CaptureScope    *CaptureScope `json:"capture_scope,omitempty"`
}
type SDKImportMetadata struct {
	ArtifactID             uuid.UUID        `json:"artifact_id"`
	ChallengePackVersionID *uuid.UUID       `json:"challenge_pack_version_id,omitempty"`
	ApplicationRevision    *string          `json:"application_revision"`
	Fingerprints           Fingerprints     `json:"fingerprints"`
	CaptureScope           CaptureScope     `json:"capture_scope"`
	Cases                  []SDKCaseBinding `json:"cases"`
}
type SDKReportImport struct {
	EnvelopeVersion     int               `json:"envelope_version"`
	SourceKind          string            `json:"source_kind"`
	SourceSchemaVersion int               `json:"source_schema_version"`
	SourceJSON          string            `json:"source_json"`
	ExternalReportID    string            `json:"external_report_id,omitempty"`
	Metadata            SDKImportMetadata `json:"metadata"`
}
type SDKReport struct {
	ID                  uuid.UUID            `json:"id"`
	Binding             ProjectResultBinding `json:"binding"`
	EvidenceMetadata    EvidenceMetadata     `json:"evidence_metadata"`
	SourceKind          string               `json:"source_kind"`
	SourceSchemaVersion int                  `json:"source_schema_version"`
	ExternalReportID    string               `json:"external_report_id"`
	ContentHash         string               `json:"content_sha256"`
	Size                int                  `json:"size_bytes"`
	CreatedAt           time.Time            `json:"created_at"`
	Cases               []SDKCaseBinding     `json:"cases"`
	Summary             json.RawMessage      `json:"summary,omitempty"`
	Diagnostics         []string             `json:"diagnostics,omitempty"`
	SourceJSON          string               `json:"-"`
}

func invalidSDK() error {
	return fault("invalid_import", "Use a bounded v1 SDK report or agent result with exact case bindings and capture metadata.")
}
func DecodeSDKReport(b []byte) (SDKReportImport, error) {
	var in SDKReportImport
	l := LimitsFor(false)
	l.FileBytes = SDKReportRequestBytes
	l.StringBytes = SDKReportSourceBytes
	if validateJSON(b, l, true) != nil {
		return in, invalidSDK()
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || in.EnvelopeVersion != 1 || in.SourceSchemaVersion != 1 || (in.SourceKind != "eval_report" && in.SourceKind != "agent_result") || in.Metadata.ArtifactID == uuid.Nil {
		return in, invalidSDK()
	}
	// Typed decoding alone cannot distinguish omitted required nullable fields.
	var shape map[string]json.RawMessage
	_ = json.Unmarshal(b, &shape)
	var metadata map[string]json.RawMessage
	_ = json.Unmarshal(shape["metadata"], &metadata)
	for _, key := range []string{"artifact_id", "application_revision", "fingerprints", "capture_scope", "cases"} {
		if _, ok := metadata[key]; !ok {
			return in, invalidSDK()
		}
	}
	var fps map[string]json.RawMessage
	_ = json.Unmarshal(metadata["fingerprints"], &fps)
	for _, key := range []string{"model", "build", "test", "judge", "config"} {
		if _, ok := fps[key]; !ok {
			return in, invalidSDK()
		}
	}
	if len(raw(in.Metadata)) > SDKReportMetadataBytes || len(in.Metadata.Cases) > LimitsFor(false).ImportCases || in.Metadata.Cases == nil || !in.Metadata.CaptureScope.Valid() {
		return in, invalidSDK()
	}
	for _, v := range []*string{in.Metadata.ApplicationRevision, in.Metadata.Fingerprints.Model, in.Metadata.Fingerprints.Build, in.Metadata.Fingerprints.Test, in.Metadata.Fingerprints.Judge, in.Metadata.Fingerprints.Config} {
		if v != nil && (len(*v) > 1024 || strings.TrimSpace(*v) == "") {
			return in, invalidSDK()
		}
	}
	if len(in.ExternalReportID) > 256 {
		return in, invalidSDK()
	}
	l = LimitsFor(false)
	l.FileBytes = SDKReportSourceBytes
	if validateJSON([]byte(in.SourceJSON), l, true) != nil {
		return in, invalidSDK()
	}
	var source map[string]any
	if json.Unmarshal([]byte(in.SourceJSON), &source) != nil {
		return in, invalidSDK()
	}
	schemas, err := sdkSchemas()
	if err != nil {
		return in, fault("unavailable", "SDK schemas are unavailable.")
	}
	if schemas[in.SourceKind].Validate(source) != nil {
		return in, invalidSDK()
	}
	byID := map[string]map[string]any{}
	if in.SourceKind == "eval_report" {
		id := source["report_id"].(string)
		if len(id) > 256 || (in.ExternalReportID != "" && in.ExternalReportID != id) {
			return in, invalidSDK()
		}
		in.ExternalReportID = id
		if _, err := time.Parse(time.RFC3339Nano, source["generated_at"].(string)); err != nil {
			return in, invalidSDK()
		}
		for _, v := range source["cases"].([]any) {
			c := v.(map[string]any)
			id := c["case"].(map[string]any)["case_id"].(string)
			if _, ok := byID[id]; ok {
				return in, invalidSDK()
			}
			agent, _ := c["agent_result"].(map[string]any)
			byID[id] = agent
		}
		if len(byID) != len(in.Metadata.Cases) {
			return in, invalidSDK()
		}
	} else {
		if len(in.Metadata.Cases) != 1 {
			return in, invalidSDK()
		}
		byID[in.Metadata.Cases[0].SourceCaseID] = source
	}
	seen := map[string]bool{}
	for _, c := range in.Metadata.Cases {
		if strings.TrimSpace(c.SourceCaseID) == "" || len(c.SourceCaseID) > 256 || c.CaseKey == "" || len(c.CaseKey) > MaxKeyBytes || seen[c.SourceCaseID] {
			return in, invalidSDK()
		}
		seen[c.SourceCaseID] = true
		for _, v := range []*string{c.InputSetKey, c.ChallengeKey} {
			if v != nil && len(*v) > MaxKeyBytes {
				return in, invalidSDK()
			}
		}
		agent, ok := byID[c.SourceCaseID]
		if !ok {
			return in, invalidSDK()
		}
		capture := in.Metadata.CaptureScope
		if c.CaptureScope != nil {
			capture = *c.CaptureScope
		}
		if !capture.Valid() {
			return in, invalidSDK()
		}
		for key, v := range map[string]DimensionCapture{"messages": capture.Messages, "tool_calls": capture.Tools, "retrieval_context": capture.Retrieval} {
			if v.Scope == "complete" {
				if a, ok := agent[key].([]any); !ok || a == nil {
					return in, invalidSDK()
				}
			}
		}
	}
	return in, nil
}
