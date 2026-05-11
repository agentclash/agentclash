package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRunReplayCallsReplayEndpointAndRendersSteps(t *testing.T) {
	var gotCursor string
	var gotLimit string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/replays/agent-1": func(w http.ResponseWriter, r *http.Request) {
			gotCursor = r.URL.Query().Get("cursor")
			gotLimit = r.URL.Query().Get("limit")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"state":            "ready",
				"run_agent_status": "completed",
				"steps": []map[string]any{
					{"step_type": "model_call", "summary": "Explained the fix"},
					{"step_type": "tool_call", "summary": "Ran regression tests"},
				},
			})
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "replay", "agent-1", "--cursor", "3", "--limit", "2",
	}, srv.URL); err != nil {
		t.Fatalf("run replay error: %v", err)
	}

	if gotCursor != "3" || gotLimit != "2" {
		t.Fatalf("query cursor/limit = %q/%q, want 3/2", gotCursor, gotLimit)
	}
	out := stdout.finish()
	for _, snippet := range []string{
		"Run Agent ID:",
		"agent-1",
		"Replay Steps",
		"model_call",
		"Ran regression tests",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("run replay output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestReplayGetLegacySurfaceUsesSharedReplayPath(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/replays/agent-1": jsonHandler(http.StatusOK, map[string]any{
			"state":            "ready",
			"run_agent_status": "completed",
			"steps": []map[string]any{
				{"step_type": "model_call", "summary": "Legacy replay still works"},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"replay", "get", "agent-1", "--limit", "1"}, srv.URL); err != nil {
		t.Fatalf("replay get error: %v", err)
	}

	out := stdout.finish()
	if !strings.Contains(out, "Legacy replay still works") {
		t.Fatalf("legacy replay output missing shared renderer content\n---\n%s", out)
	}
}

func TestRunCompareCallsCompareEndpointAndRendersRegressionSummary(t *testing.T) {
	var gotBaseline string
	var gotCandidate string
	var gotBaselineAgent string
	var gotCandidateAgent string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/compare": func(w http.ResponseWriter, r *http.Request) {
			gotBaseline = r.URL.Query().Get("baseline_run_id")
			gotCandidate = r.URL.Query().Get("candidate_run_id")
			gotBaselineAgent = r.URL.Query().Get("baseline_run_agent_id")
			gotCandidateAgent = r.URL.Query().Get("candidate_run_agent_id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"state":       "comparable",
				"status":      "regression",
				"reason_code": "correctness_regressed",
				"key_deltas": []map[string]any{
					{
						"metric":          "correctness",
						"baseline_value":  0.96,
						"candidate_value": 0.81,
						"delta":           -0.15,
						"outcome":         "regressed",
					},
				},
				"regression_reasons": []string{
					"candidate failed a regression case that baseline passed",
				},
				"evidence_quality": map[string]any{
					"warnings": []string{"candidate replay omitted one terminal event"},
				},
			})
		},
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "compare",
		"--baseline", "run-b",
		"--candidate", "run-c",
		"--baseline-agent", "agent-b",
		"--candidate-agent", "agent-c",
	}, srv.URL); err != nil {
		t.Fatalf("run compare error: %v", err)
	}

	if gotBaseline != "run-b" || gotCandidate != "run-c" || gotBaselineAgent != "agent-b" || gotCandidateAgent != "agent-c" {
		t.Fatalf("compare query = baseline %q candidate %q baseline-agent %q candidate-agent %q", gotBaseline, gotCandidate, gotBaselineAgent, gotCandidateAgent)
	}
	out := stdout.finish()
	for _, snippet := range []string{
		"Run Comparison",
		"correctness_regressed",
		"correctness",
		"regressed",
		"candidate failed a regression case",
		"candidate replay omitted one terminal event",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("run compare output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestCompareRunsLegacySurfaceUsesSharedComparePath(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/compare": jsonHandler(http.StatusOK, map[string]any{
			"state":       "comparable",
			"status":      "improved",
			"reason_code": "candidate_improved",
			"key_deltas": []map[string]any{
				{
					"metric":          "correctness",
					"baseline_value":  0.81,
					"candidate_value": 0.96,
					"delta":           0.15,
					"outcome":         "improved",
				},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"compare", "runs", "--baseline", "run-b", "--candidate", "run-c",
	}, srv.URL); err != nil {
		t.Fatalf("compare runs error: %v", err)
	}

	out := stdout.finish()
	for _, snippet := range []string{"Run Comparison", "candidate_improved", "improved"} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("legacy compare output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestRunCompareSanitizesRegressionReasonLines(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/compare": jsonHandler(http.StatusOK, map[string]any{
			"state":        "comparable\nstate",
			"status":       "regression\x1b[2J",
			"reason_code":  "correctness_regressed\nreason",
			"generated_at": "2026-05-11T10:00:00Z\ngenerated",
			"key_deltas": []any{
				nil,
				"not an object",
				map[string]any{
					"metric":          "correctness\nmetric",
					"baseline_value":  0.96,
					"candidate_value": 0.81,
					"delta":           -0.15,
					"outcome":         "regressed\noutcome",
				},
			},
			"regression_reasons": []string{
				"candidate regressed\x1b[2J\nerror: forged reason",
			},
			"evidence_quality": map[string]any{
				"warnings":       []string{"missing replay\x07\nwarning: forged warning"},
				"missing_fields": []string{"summary\nfield"},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "compare", "--baseline", "run-b", "--candidate", "run-c",
	}, srv.URL); err != nil {
		t.Fatalf("run compare error: %v", err)
	}

	out := stdout.finish()
	for _, forbidden := range []string{"\x1b", "\x07"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("compare output leaked control byte %q: %q", forbidden, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "error: forged") || strings.HasPrefix(trimmed, "warning: forged") || trimmed == "field" {
			t.Fatalf("compare output allowed forged newline line: %q", out)
		}
	}
	for _, snippet := range []string{
		"comparable state",
		"correctness_regressed reason",
		"2026-05-11T10:00:00Z generated",
		"correctness metric",
		"regressed outcome",
		"candidate regressed",
		"error: forged reason",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("sanitized compare output missing %q\n---\n%s", snippet, out)
		}
	}
}

func TestRunCompareSanitizesDimensionTableCells(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/compare": jsonHandler(http.StatusOK, map[string]any{
			"state":  "comparable",
			"status": "stable",
			"dimensions": []any{
				nil,
				map[string]any{
					"name":            "instruction following\nDimension",
					"baseline_score":  0.9,
					"candidate_score": 0.8,
					"delta":           -0.1,
				},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "compare", "--baseline", "run-b", "--candidate", "run-c",
	}, srv.URL); err != nil {
		t.Fatalf("run compare error: %v", err)
	}

	out := stdout.finish()
	if !strings.Contains(out, "instruction following Dimension") {
		t.Fatalf("sanitized dimension name missing\n---\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "Dimension" {
			t.Fatalf("dimension table allowed forged newline line: %q", out)
		}
	}
}

func TestRunReplaySanitizesDetailLines(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/replays/agent-1": jsonHandler(http.StatusOK, map[string]any{
			"state":            "ready\nstate",
			"message":          "finished\x1b[2J\nerror: forged message",
			"run_agent_status": "completed\nstatus",
			"steps": []any{
				nil,
				map[string]any{"step_type": "model_call", "summary": "ok"},
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "replay", "agent-1"}, srv.URL); err != nil {
		t.Fatalf("run replay error: %v", err)
	}

	out := stdout.finish()
	if strings.Contains(out, "\x1b") {
		t.Fatalf("replay output leaked control byte: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "error: forged") {
			t.Fatalf("replay output allowed forged newline line: %q", out)
		}
	}
	for _, snippet := range []string{"ready state", "finished", "error: forged message", "completed status"} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("sanitized replay output missing %q\n---\n%s", snippet, out)
		}
	}
}
