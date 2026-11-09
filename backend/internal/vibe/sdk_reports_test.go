package vibe

import (
	"encoding/json"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sdkEnvelope(t *testing.T, kind, source string) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(source), &doc); err != nil {
		t.Fatal(err)
	}
	cases := []any{}
	if kind == "eval_report" {
		for _, v := range doc["cases"].([]any) {
			id := v.(map[string]any)["case"].(map[string]any)["case_id"].(string)
			cases = append(cases, map[string]any{"source_case_id": id, "case_key": id})
		}
	} else {
		cases = append(cases, map[string]any{"source_case_id": "local", "case_key": "one"})
	}
	return raw(map[string]any{"envelope_version": 1, "source_kind": kind, "source_schema_version": 1, "source_json": source, "metadata": map[string]any{"artifact_id": uuid.New(), "application_revision": nil, "fingerprints": Fingerprints{}, "capture_scope": UnknownCaptureScope(), "cases": cases}})
}
func TestSDKReportStrictV1AndOriginalBytes(t *testing.T) {
	for _, name := range []string{"all-pass", "metric-failure", "provider-error", "config-error", "multi-turn"} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("../../../schemas/evaltest/fixtures", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			source := " \r\n" + string(b) + "\t\n"
			got, err := DecodeSDKReport(sdkEnvelope(t, "eval_report", source))
			if err != nil {
				t.Fatal(err)
			}
			if got.SourceJSON != source {
				t.Fatal("source bytes changed")
			}
		})
	}
	for _, source := range []string{`{"input":"hello"}`, `{"input":"hello","metadata":{"extra":{"$ref":"inert"}},"tool_calls":[{"name":"x","arguments":{"$dynamicRef":"inert"}}]}`} {
		if _, err := DecodeSDKReport(sdkEnvelope(t, "agent_result", source)); err != nil {
			t.Fatal("valid standalone", err)
		}
	}
	for _, source := range []string{`{"input":"hello","schema_version":1}`, `{"input":"hello","unknown":1}`, `{"input":"hello","input":"again"}`, `{"input":"hello"} {}`, `{"input":"\ud800"}`, `{"input":"hi","metadata":{"cost_usd":1e999}}`} {
		envelope := sdkEnvelope(t, "agent_result", `{"input":"ok"}`)
		var v map[string]any
		json.Unmarshal(envelope, &v)
		v["source_json"] = source
		if _, err := DecodeSDKReport(raw(v)); err == nil {
			t.Fatalf("accepted invalid source %s", source)
		}
	}
	if err := ValidateJSON([]byte(`{"$ref":"blocked"}`), LimitsFor(false)); err == nil {
		t.Fatal("definition parser relaxed")
	}
}
func TestSDKReportEnvelopeAndCaptureLimits(t *testing.T) {
	original := sdkEnvelope(t, "agent_result", `{"input":"hi"}`)
	for _, change := range []func(map[string]any){func(v map[string]any) { v["actor"] = "forged" }, func(v map[string]any) { v["source_schema_version"] = 2 }, func(v map[string]any) { v["source_json"] = strings.Repeat(" ", SDKReportSourceBytes+1) }, func(v map[string]any) { v["metadata"].(map[string]any)["cases"] = []any{} }, func(v map[string]any) {
		v["metadata"].(map[string]any)["capture_scope"].(map[string]any)["tools"] = map[string]any{"scope": "complete", "instrumentation": "instrumented"}
	}} {
		var v map[string]any
		json.Unmarshal(original, &v)
		change(v)
		if _, err := DecodeSDKReport(raw(v)); err == nil {
			t.Fatal("invalid envelope accepted")
		}
	}
	var v map[string]any
	json.Unmarshal(original, &v)
	v["source_json"] = `{"input":"hi","tool_calls":[]}`
	v["metadata"].(map[string]any)["capture_scope"].(map[string]any)["tools"] = map[string]any{"scope": "complete", "instrumentation": "instrumented"}
	if _, err := DecodeSDKReport(raw(v)); err != nil {
		t.Fatal("complete empty capture rejected", err)
	}
}
func TestSDKSchemaPackagedBytes(t *testing.T) {
	for name, got := range map[string][]byte{"eval-report": sdkReportSchema, "agent-result": sdkAgentSchema} {
		want, err := os.ReadFile("../../../schemas/evaltest/" + name + ".schema.json")
		if err != nil || string(want) != string(got) {
			t.Fatal("packaged canonical schema drift", name, err)
		}
	}
}

func TestSDKRegradeKeepsExistingQuoteFingerprint(t *testing.T) {
	p := Plan{Cases: []string{"one"}, Submission: Submission{Kind: "check", Models: Models{Target: "target", Evaluator: "judge"}}, Calls: 2, MaxCost: 100}
	// Existing issued quotes use this precise pre-SDK serialization. A nil SDK
	// source must add no bytes; SDK plans must bind their source hash separately.
	old := Hash(raw(struct {
		Cases    []string
		Recovery *ResultRecovery
		Kind     string
		Purpose  string
		Models   Models
		Artifact *Artifact
		Evidence *EvidenceSet
		Baseline *uuid.UUID
		Cost     int64
		Calls    int
		Grading  *GradingContract
		Target   *TargetConfiguration
		Limits   *Limits
	}{p.Cases, p.Recovery, p.Submission.Kind, p.Submission.Purpose, p.Submission.Models, p.Artifact, p.Evidence, p.Submission.BaselineID, p.MaxCost, p.Calls, p.Grading, p.TargetConfig, p.ExecutionLimits}))
	if runFingerprint(p) != old {
		t.Fatal("ordinary quote fingerprint changed")
	}
	p.RegradeSource = &ResultRecovery{Source: ResultRef{Kind: ResultSDKReport, ID: uuid.New()}, ArtifactID: uuid.New(), EvidenceHash: "one"}
	first := runFingerprint(p)
	p.RegradeSource.EvidenceHash = "two"
	if runFingerprint(p) == first {
		t.Fatal("SDK quote omitted source evidence hash")
	}
}
