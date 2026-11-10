package vibe

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func refundFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../../schemas/refund-report-v2/fixtures/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func refundEnvelope(t *testing.T, source string) []byte {
	t.Helper()
	var e map[string]any
	json.Unmarshal(sdkEnvelope(t, "eval_report", source), &e)
	e["source_schema_version"] = 2
	return raw(e)
}
func TestSDKRefundV2Fixtures(t *testing.T) {
	for _, name := range []string{"claude-correct-v2", "claude-faulty-v1", "openai-correct-v2", "openai-faulty-v1"} {
		t.Run(name, func(t *testing.T) {
			source := refundFixture(t, name)
			got, err := DecodeSDKReport(refundEnvelope(t, source))
			if err != nil {
				t.Fatal(err)
			}
			if got.SourceJSON != source || got.SourceSchemaVersion != 2 {
				t.Fatal("original changed")
			}
			if _, err = DecodeSDKReport(sdkEnvelope(t, "eval_report", source)); err == nil {
				t.Fatal("v2 mislabeled v1 accepted")
			}
		})
	}
}
func TestSDKRefundV2Malformed(t *testing.T) {
	changes := map[string]func(map[string]any){
		"version":        func(d map[string]any) { d["schema_version"] = 1 },
		"unknown":        func(d map[string]any) { d["unknown"] = true },
		"run":            func(d map[string]any) { d["run_id"] = "other" },
		"summary":        func(d map[string]any) { d["summary"].(map[string]any)["passed"] = 0 },
		"case duplicate": func(d map[string]any) { d["cases"] = append(d["cases"].([]any), d["cases"].([]any)[0]) },
		"check duplicate": func(d map[string]any) {
			c := d["cases"].([]any)[0].(map[string]any)
			c["checks"] = append(c["checks"].([]any), c["checks"].([]any)[0])
		},
		"check mismatch": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["checks"].([]any)[0].(map[string]any)["actual"] = false
		},
		"invocation": func(d map[string]any) { d["cases"].([]any)[0].(map[string]any)["target_invoked"] = false },
		"missing": func(d map[string]any) {
			d["execution"].(map[string]any)["missing_required_case_ids"] = []any{"missing"}
		},
		"exit": func(d map[string]any) { d["exit_code"] = 1 },
		"failure": func(d map[string]any) {
			d["failures"] = []any{map[string]any{"case_id": "fake", "check_id": "fake", "reason": "bad"}}
		},
		"cost": func(d map[string]any) { d["cases"].([]any)[0].(map[string]any)["cost_usd"] = 0 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			var d map[string]any
			json.Unmarshal([]byte(refundFixture(t, "openai-correct-v2")), &d)
			change(d)
			if _, err := DecodeSDKReport(refundEnvelope(t, string(raw(d)))); err == nil {
				t.Fatal("accepted malformed v2")
			}
		})
	}
}

func TestSDKRefundV2IncompleteDiagnosticsAndProjection(t *testing.T) {
	for _, status := range []string{"incomplete", "skipped", "error", "zero"} {
		t.Run(status, func(t *testing.T) {
			var d map[string]any
			json.Unmarshal([]byte(refundFixture(t, "openai-correct-v2")), &d)
			c := d["cases"].([]any)[0].(map[string]any)
			c["status"] = status
			c["target_invoked"] = false
			c["checks"] = []any{}
			c["target_latency_ms"] = nil
			c["scoring_latency_ms"] = nil
			c["observations"] = map[string]any{"tool_calls": false, "fixture_state": false, "cost": false}
			c["evidence"] = map[string]any{"arbitrary": map[string]any{"$ref": "inert"}}
			counts := map[string]any{"total": 1, "passed": 0, "failed": 0, "errored": 0, "skipped": 0, "incomplete": 0, "metric_failures": 0}
			key := status
			if key == "error" {
				key = "errored"
			}
			counts[key] = 1
			d["summary"] = counts
			ex := d["execution"].(map[string]any)
			ex["required_cases_complete"] = false
			ex["missing_required_case_ids"] = ex["required_case_ids"]
			d["exit_code"] = 2
			if status == "error" {
				ex["status"] = "config_error"
				ex["pytest_exit_code"] = nil
			}
			if status == "zero" {
				delete(counts, "zero")
				counts["total"] = 0
				d["cases"] = []any{}
				ex["pytest_exit_code"] = 5
			}
			source := string(raw(d))
			in, err := DecodeSDKReport(refundEnvelope(t, source))
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := validateRefundReport(in.SourceJSON)
			if err != nil {
				t.Fatal(err)
			}
			if len(refundDiagnostics(parsed, in.Metadata)) < 3 {
				t.Fatal("missing incomplete diagnostic")
			}
			if status != "zero" {
				report := SDKReport{SourceKind: "eval_report", SourceSchemaVersion: 2, SourceJSON: source, Cases: in.Metadata.Cases}
				got, err := SDKReportCase(report, in.Metadata.Cases[0].SourceCaseID, "")
				if err != nil || got.Verdict != Unknown || got.Output != "" {
					t.Fatal("incomplete upgraded", got, err)
				}
			}
			if !strings.Contains(string(refundProvenance(parsed)), `"cost_usd":null`) && status != "zero" {
				t.Fatal("unknown cost lost")
			}
		})
	}
}
func TestSDKRefundV2ClaimsAndSafeView(t *testing.T) {
	source := refundFixture(t, "openai-correct-v2")
	in, err := DecodeSDKReport(refundEnvelope(t, source))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := validateRefundReport(source)
	different := "different"
	in.Metadata.ApplicationRevision = &different
	in.Metadata.Fingerprints.Test = &different
	ds := strings.Join(refundDiagnostics(parsed, in.Metadata), " ")
	if !strings.Contains(ds, "revision differs") || !strings.Contains(ds, "fingerprint differs") {
		t.Fatal(ds)
	}
	view := string(refundProvenance(parsed))
	for _, secret := range []string{"prompt", "tool_calls\":[", "Duplicate already refunded", "arguments"} {
		if strings.Contains(view, secret) {
			t.Fatal("trace in summary", secret)
		}
	}
	r := SDKReport{SourceKind: "eval_report", SourceSchemaVersion: 2, SourceJSON: source, Cases: in.Metadata.Cases}
	c, err := SDKReportCase(r, in.Metadata.Cases[0].SourceCaseID, "")
	if err != nil || c.Verdict != Pass || len(c.Checks) != 10 || c.Output != "" || c.Expected != "" {
		t.Fatal(c, err)
	}
}

func TestSDKRefundV2ObservedChecksKeepOriginalCoordinates(t *testing.T) {
	source := refundFixture(t, "openai-correct-v2")
	in, err := DecodeSDKReport(refundEnvelope(t, source))
	if err != nil {
		t.Fatal(err)
	}
	r := SDKReport{SourceKind: "eval_report", SourceSchemaVersion: 2, SourceJSON: source, Cases: in.Metadata.Cases, EvidenceMetadata: EvidenceMetadata{EvidenceProvenance: EvidenceProvenance{Origin: OriginImported, CaptureScope: UnknownCaptureScope()}}}
	got, err := observedSDK(r)
	if err != nil || len(got.Cases) != 1 || len(got.Cases[0].Entries) != 10 {
		t.Fatal(got, err)
	}
	if got.Cases[0].Entries[0].Path != "/cases/0/checks/0" || !strings.Contains(string(got.Cases[0].Entries[0].Value), `"check_id"`) || got.Cases[0].CaptureScope.Tools.Scope != "unknown" {
		t.Fatal("v2 changed provenance", got)
	}
	for _, e := range got.Cases[0].Entries {
		if e.Kind != "grade" {
			t.Fatal("invented normalized observation", e)
		}
	}
}

func TestSDKRefundV2RejectsStructuralAndCoverageDrift(t *testing.T) {
	changes := map[string]func(map[string]any){
		"unknown runner": func(d map[string]any) { d["runner"].(map[string]any)["hosted"] = true },
		"unknown case": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["agent_result"] = map[string]any{"output": "invented"}
		},
		"unknown check": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["checks"].([]any)[0].(map[string]any)["metric_key"] = "invented"
		},
		"unknown observation": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["observations"].(map[string]any)["retrieval"] = true
		},
		"missing final check": func(d map[string]any) {
			c := d["cases"].([]any)[0].(map[string]any)
			c["checks"] = c["checks"].([]any)[:9]
		},
		"unpaired turns": func(d map[string]any) {
			c := d["cases"].([]any)[0].(map[string]any)
			checks := c["checks"].([]any)
			c["checks"] = append(checks[:1], checks[2:]...)
		},
		"noncontiguous turns": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["checks"].([]any)[0].(map[string]any)["check_id"] = "turn_3_outcome_truthful"
		},
		"duplicate required": func(d map[string]any) {
			e := d["execution"].(map[string]any)
			e["required_case_ids"] = append(e["required_case_ids"].([]any), e["required_case_ids"].([]any)[0])
		},
		"coverage flag": func(d map[string]any) { d["execution"].(map[string]any)["required_cases_complete"] = false },
		"missing observations": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["observations"].(map[string]any)["fixture_state"] = false
		},
		"observed cost": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["observations"].(map[string]any)["cost"] = true
		},
		"suite digest": func(d map[string]any) { d["suite_digest"] = "invalid" },
		"timestamp":    func(d map[string]any) { d["generated_at"] = "tomorrow" },
		"failed without failures": func(d map[string]any) {
			d["cases"].([]any)[0].(map[string]any)["status"] = "failed"
			d["summary"].(map[string]any)["passed"] = 0
			d["summary"].(map[string]any)["failed"] = 1
			d["exit_code"] = 1
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			var d map[string]any
			json.Unmarshal([]byte(refundFixture(t, "openai-correct-v2")), &d)
			change(d)
			if _, err := DecodeSDKReport(refundEnvelope(t, string(raw(d)))); err == nil {
				t.Fatal("accepted contract drift")
			}
		})
	}
	b, err := os.ReadFile("../../../schemas/evaltest/fixtures/all-pass.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeSDKReport(refundEnvelope(t, string(b))); err == nil {
		t.Fatal("v1 mislabeled v2")
	}
	var e map[string]any
	json.Unmarshal(refundEnvelope(t, refundFixture(t, "openai-correct-v2")), &e)
	e["metadata"].(map[string]any)["capture_scope"].(map[string]any)["tools"] = map[string]any{"scope": "complete", "instrumentation": "instrumented"}
	if _, err = DecodeSDKReport(raw(e)); err == nil {
		t.Fatal("v2 fixture observations became v1 complete tool capture")
	}
}

func TestSDKRefundV2JSONBudgetPreservesAuthority(t *testing.T) {
	for _, key := range []string{"__proto__", "prototype", "constructor", "$ref"} {
		t.Run(key, func(t *testing.T) {
			var d map[string]any
			json.Unmarshal([]byte(refundFixture(t, "openai-correct-v2")), &d)
			d["cases"].([]any)[0].(map[string]any)["evidence"].(map[string]any)["arbitrary"] = map[string]any{key: map[string]any{"raw": []any{nil, true, "inert"}}}
			_, err := DecodeSDKReport(refundEnvelope(t, string(raw(d))))
			if (err == nil) != (key == "$ref") {
				t.Fatal("reserved key or inert evidence behavior changed", key, err)
			}
		})
	}
	var d map[string]any
	json.Unmarshal([]byte(refundFixture(t, "openai-correct-v2")), &d)
	var nested any = "inert"
	for i := 0; i < 18; i++ {
		nested = map[string]any{"nested": nested}
	}
	d["cases"].([]any)[0].(map[string]any)["evidence"].(map[string]any)["arbitrary"] = nested
	if _, err := DecodeSDKReport(refundEnvelope(t, string(raw(d)))); err == nil {
		t.Fatal("v2 MCP depth limit relaxed")
	}
	// Shared v1 data imports retain their prior inert-key behavior.
	if _, err := DecodeSDKReport(sdkEnvelope(t, "agent_result", `{"input":"original","metadata":{"extra":{"constructor":{"$ref":"inert"}}}}`)); err != nil {
		t.Fatal("v1 changed", err)
	}
}
