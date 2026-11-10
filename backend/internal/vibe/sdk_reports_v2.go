package vibe

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

// Refund v2 has observed fixture checks, not AgentResult-v1 reply semantics.
// These types are a read-only projection. SourceJSON remains the original bytes.
type refundReport struct {
	ReportID       string  `json:"report_id"`
	RunID          string  `json:"run_id"`
	SuiteID        *string `json:"suite_id"`
	SuiteDigest    *string `json:"suite_digest"`
	TargetIdentity *string `json:"target_identity"`
	TargetRevision *string `json:"target_revision"`
	Runner         struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		Language string `json:"language"`
	} `json:"runner"`
	Execution struct {
		Mode       string   `json:"mode"`
		Status     string   `json:"status"`
		PytestExit *int     `json:"pytest_exit_code"`
		Duration   float64  `json:"duration_ms"`
		Required   []string `json:"required_case_ids"`
		Missing    []string `json:"missing_required_case_ids"`
		Complete   bool     `json:"required_cases_complete"`
	} `json:"execution"`
	Summary  map[string]int `json:"summary"`
	Cases    []refundCase   `json:"cases"`
	Failures []struct {
		CaseID  string `json:"case_id"`
		CheckID string `json:"check_id"`
		Reason  string `json:"reason"`
	} `json:"failures"`
	Exit int `json:"exit_code"`
}
type refundCase struct {
	Case struct {
		ID     string  `json:"case_id"`
		Name   string  `json:"name"`
		NodeID string  `json:"nodeid"`
		Split  *string `json:"split"`
	} `json:"case"`
	Status         string        `json:"status"`
	Checks         []refundCheck `json:"checks"`
	Invoked        bool          `json:"target_invoked"`
	TargetLatency  *float64      `json:"target_latency_ms"`
	ScoringLatency *float64      `json:"scoring_latency_ms"`
	Cost           *float64      `json:"cost_usd"`
	Observations   struct {
		Tools   bool `json:"tool_calls"`
		Fixture bool `json:"fixture_state"`
		Cost    bool `json:"cost"`
	} `json:"observations"`
	Evidence map[string]json.RawMessage `json:"evidence"`
	Error    *string                    `json:"error"`
}
type refundCheck struct {
	ID       string                     `json:"check_id"`
	Passed   bool                       `json:"passed"`
	Expected any                        `json:"expected"`
	Actual   any                        `json:"actual"`
	Evidence map[string]json.RawMessage `json:"evidence"`
}

var refundFinalChecks = []string{"duplicate_refunds", "original_refunds", "subscription_active", "outcome", "refund_amounts_correct", "no_unexpected_refunds"}
var refundTurnCheck = regexp.MustCompile(`^turn_([1-9]|10)_(outcome_truthful|subscription_preserved)$`)

func (c refundCase) complete() bool {
	return (c.Status == "passed" || c.Status == "failed") && c.Invoked && c.Observations.Tools && c.Observations.Fixture
}
func refundInteger(v any) bool { n, ok := v.(float64); return ok && n >= 0 && n == math.Trunc(n) }

// Semantics follow the pinned MCP comparison contract and runner finalization.
// Failure reasons remain opaque text; only their exact case/check references and
// execution-error correspondence are checked, without regrading raw evidence.
func validateRefundReport(source string) (refundReport, error) {
	var r refundReport
	if json.Unmarshal([]byte(source), &r) != nil || r.ReportID != r.RunID {
		return r, invalidSDK()
	}
	counts := map[string]int{"total": len(r.Cases), "passed": 0, "failed": 0, "skipped": 0, "errored": 0, "incomplete": 0, "metric_failures": 0}
	ids := map[string]refundCase{}
	expectedFailures := map[string]string{}
	for _, c := range r.Cases {
		if _, ok := ids[c.Case.ID]; ok {
			return r, invalidSDK()
		}
		ids[c.Case.ID] = c
		status := c.Status
		if status == "error" {
			status = "errored"
		}
		counts[status]++
		checks := map[string]refundCheck{}
		failed := 0
		turns := 0
		for _, ch := range c.Checks {
			if _, ok := checks[ch.ID]; ok {
				return r, invalidSDK()
			}
			checks[ch.ID] = ch
			execution := ch.ID == "pytest_setup" || ch.ID == "pytest_call" || ch.ID == "pytest_teardown"
			if execution {
				if _, ok := ch.Actual.(string); !ok || c.Status == "passed" || ch.Passed || ch.Expected != "no exception" {
					return r, invalidSDK()
				}
			} else {
				if ch.Passed != reflect.DeepEqual(ch.Expected, ch.Actual) || (!slices.Contains(refundFinalChecks, ch.ID) && !refundTurnCheck.MatchString(ch.ID)) {
					return r, invalidSDK()
				}
				switch ch.ID {
				case "duplicate_refunds":
					if (ch.Expected != float64(0) && ch.Expected != float64(1)) || !refundInteger(ch.Actual) {
						return r, invalidSDK()
					}
				case "original_refunds":
					if ch.Expected != float64(0) || !refundInteger(ch.Actual) {
						return r, invalidSDK()
					}
				case "outcome":
					if !slices.Contains([]any{"refunded", "clarify", "error"}, ch.Expected) {
						return r, invalidSDK()
					}
					if _, ok := ch.Actual.(string); !ok {
						return r, invalidSDK()
					}
				default:
					if ch.Expected != true {
						return r, invalidSDK()
					}
					if _, ok := ch.Actual.(bool); !ok {
						return r, invalidSDK()
					}
				}
			}
			if strings.HasPrefix(ch.ID, "turn_") && strings.HasSuffix(ch.ID, "_outcome_truthful") {
				turns++
			}
			if !ch.Passed {
				failed++
				expectedFailures[c.Case.ID+"/"+ch.ID] = ""
			}
		}
		counts["metric_failures"] += failed
		if c.Status == "passed" || c.Status == "failed" {
			if !c.complete() {
				return r, invalidSDK()
			}
			for _, id := range refundFinalChecks {
				if _, ok := checks[id]; !ok {
					return r, invalidSDK()
				}
			}
			if turns == 0 {
				return r, invalidSDK()
			}
			for turn := 1; turn <= turns; turn++ {
				for _, suffix := range []string{"outcome_truthful", "subscription_preserved"} {
					if _, ok := checks[fmt.Sprintf("turn_%d_%s", turn, suffix)]; !ok {
						return r, invalidSDK()
					}
				}
			}
			for id := range checks {
				if strings.HasPrefix(id, "turn_") && strings.HasSuffix(id, "_subscription_preserved") {
					pair := strings.TrimSuffix(id, "subscription_preserved") + "outcome_truthful"
					if _, ok := checks[pair]; !ok {
						return r, invalidSDK()
					}
				}
			}
			if (checks["outcome"].Expected == "refunded") != (checks["duplicate_refunds"].Expected == float64(1)) {
				return r, invalidSDK()
			}
		}
		if c.Status == "passed" && (failed != 0 || c.Error != nil) || c.Status == "failed" && failed == 0 {
			return r, invalidSDK()
		}
		if c.Error != nil && *c.Error != "" {
			expectedFailures[c.Case.ID+"/execution"] = *c.Error
		}
	}
	if !reflect.DeepEqual(counts, r.Summary) {
		return r, invalidSDK()
	}
	seen := map[string]bool{}
	missing := []string{}
	for _, id := range r.Execution.Required {
		if seen[id] {
			return r, invalidSDK()
		}
		seen[id] = true
		if !ids[id].complete() {
			missing = append(missing, id)
		}
	}
	sortedMissing := slices.Clone(r.Execution.Missing)
	slices.Sort(missing)
	slices.Sort(sortedMissing)
	if !slices.Equal(missing, sortedMissing) {
		return r, invalidSDK()
	}
	complete := len(r.Cases) > 0 && len(missing) == 0
	for _, c := range r.Cases {
		complete = complete && c.complete()
	}
	if r.Execution.Complete != complete {
		return r, invalidSDK()
	}
	if len(r.Failures) != len(expectedFailures) {
		return r, invalidSDK()
	}
	for _, f := range r.Failures {
		key := f.CaseID + "/" + f.CheckID
		reason, ok := expectedFailures[key]
		if !ok || (reason != "" && reason != f.Reason) {
			return r, invalidSDK()
		}
		delete(expectedFailures, key)
	}
	code := 0
	switch r.Execution.Status {
	case "config_error":
		code = 2
	case "timeout":
		code = 3
	case "internal_error":
		code = 4
	case "completed":
		if r.Execution.PytestExit == nil || *r.Execution.PytestExit < 0 || *r.Execution.PytestExit > 5 {
			return r, invalidSDK()
		}
		switch *r.Execution.PytestExit {
		case 2, 4, 5:
			code = 2
		case 3:
			code = 4
		default:
			if counts["errored"] > 0 {
				code = 3
			} else if !complete || counts["incomplete"] > 0 {
				code = 2
			} else if counts["failed"] > 0 {
				code = 1
			}
		}
	}
	if r.Exit != code || r.Exit == 0 && (*r.Execution.PytestExit != 0 || counts["passed"] != counts["total"] || len(r.Failures) > 0) {
		return r, invalidSDK()
	}
	return r, nil
}

func refundDiagnostics(r refundReport, m SDKImportMetadata) []string {
	out := []string{"Refund v2 grades, fixture observations and target identity are uploader-supplied claims, not verified hosted execution. Exact case mapping does not establish hosted comparison equivalence.", "Cost and production model identity are unknown. Hosted regrade and expectation promotion are unsupported for this source format."}
	if !r.Execution.Complete || r.Execution.Status != "completed" {
		out = append(out, "UNKNOWN/incomplete execution: "+r.Execution.Status+". Missing required cases: "+strings.Join(r.Execution.Missing, ", ")+".")
	}
	if r.TargetIdentity == nil || r.TargetRevision == nil || r.SuiteID == nil || r.SuiteDigest == nil {
		out = append(out, "Source target or suite identity/revision is unknown.")
	}
	if m.ApplicationRevision != nil && r.TargetRevision != nil && *m.ApplicationRevision != *r.TargetRevision {
		out = append(out, "Declared application revision differs from the source target_revision; neither claim was overwritten.")
	}
	if m.Fingerprints.Test != nil && r.SuiteDigest != nil && *m.Fingerprints.Test != *r.SuiteDigest {
		out = append(out, "Declared test fingerprint differs from the source suite_digest; neither claim was overwritten.")
	}
	return out
}

func refundCaseView(r SDKReport, selected SDKCaseBinding) (CaseResult, error) {
	var doc refundReport
	if err := json.Unmarshal([]byte(r.SourceJSON), &doc); err != nil {
		return CaseResult{}, err
	}
	for _, source := range doc.Cases {
		if source.Case.ID != selected.SourceCaseID {
			continue
		}
		c := CaseResult{CaseKey: selected.CaseKey, Version: r.Binding.ArtifactID.String(), Title: source.Case.Name, Input: raw(nil), Verdict: Unknown, Checks: []CheckResult{}, ExpectedChecks: len(source.Checks)}
		if source.Status == "passed" {
			c.Verdict = Pass
		} else if source.Status == "failed" {
			c.Verdict = Fail
		}
		if source.Error != nil {
			c.Error = &Fault{Code: "imported_" + source.Status, Message: *source.Error}
		}
		for _, ch := range source.Checks {
			v := Fail
			if ch.Passed {
				v = Pass
			}
			c.Checks = append(c.Checks, CheckResult{Key: ch.ID, Verdict: v, Evidence: string(raw(map[string]any{"claim": "uploader supplied", "expected": ch.Expected, "actual": ch.Actual, "source": refundCheckSource(ch), "turn": refundCheckTurn(ch)}))})
		}
		return c, nil
	}
	return CaseResult{}, fault("not_found", "Original source case is unavailable.")
}

// Only known summary fields cross into the ordinary receipt/preview. Arbitrary
// evidence and execution tracebacks remain in the explicitly requested original.
func refundCheckSource(c refundCheck) *string {
	var s string
	if json.Unmarshal(c.Evidence["source"], &s) == nil && (s == "fixture_state" || s == "target_result") {
		return &s
	}
	return nil
}
func refundCheckTurn(c refundCheck) *int {
	var n int
	if json.Unmarshal(c.Evidence["turn"], &n) == nil && n >= 1 && n <= 10 {
		return &n
	}
	return nil
}
func refundProvenance(r refundReport) json.RawMessage {
	cases := []any{}
	for _, c := range r.Cases {
		checks := []any{}
		for _, ch := range c.Checks {
			checks = append(checks, map[string]any{"check_id": ch.ID, "passed": ch.Passed, "source": refundCheckSource(ch), "turn": refundCheckTurn(ch)})
		}
		cases = append(cases, map[string]any{"case_id": c.Case.ID, "name": c.Case.Name, "split": c.Case.Split, "status": c.Status, "target_invoked": c.Invoked, "observations": c.Observations, "cost_usd": c.Cost, "target_latency_ms": c.TargetLatency, "scoring_latency_ms": c.ScoringLatency, "checks": checks, "has_execution_error": c.Error != nil})
	}
	return raw(map[string]any{"runner": r.Runner, "suite_id": r.SuiteID, "suite_digest": r.SuiteDigest, "target_identity": r.TargetIdentity, "target_revision": r.TargetRevision, "execution": r.Execution, "cases": cases})
}

// Preserve the MCP parseContract reserved-key rule for v2 only. Reference-like
// keys remain inert JSON data; this does not change v1 or definition parsing.
func refundJSONKeys(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "__proto__" || key == "prototype" || key == "constructor" || !refundJSONKeys(child) {
				return false
			}
		}
	case []any:
		for _, child := range v {
			if !refundJSONKeys(child) {
				return false
			}
		}
	}
	return true
}
