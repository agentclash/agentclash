package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestCompareGateReturnsExitCodeByVerdict(t *testing.T) {
	cases := []struct {
		verdict string
		wantErr bool
		wantCode int
	}{
		{"pass", false, 0},
		{"warn", true, gateExitWarn},
		{"fail", true, gateExitFail},
		{"insufficient_evidence", true, gateExitInsufficientEvidence},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.verdict, func(t *testing.T) {
			srv := fakeAPI(t, map[string]http.HandlerFunc{
				"POST /v1/release-gates/evaluate": jsonHandler(200, map[string]any{
					"baseline_run_id":  "run-b",
					"candidate_run_id": "run-c",
					"release_gate": map[string]any{
						"verdict":         tc.verdict,
						"reason_code":     "tc_reason",
						"summary":         "test summary",
						"evidence_status": tc.verdict,
					},
				}),
			})
			defer srv.Close()

			t.Setenv("AGENTCLASH_TOKEN", "test-tok")
			err := executeCommand(t, []string{
				"compare", "gate",
				"--baseline", "run-b",
				"--candidate", "run-c",
			}, srv.URL)

			if !tc.wantErr {
				if err != nil {
					t.Fatalf("verdict %q: expected no error, got %v", tc.verdict, err)
				}
				return
			}

			var exitErr *ExitCodeError
			if !errors.As(err, &exitErr) {
				t.Fatalf("verdict %q: expected *ExitCodeError, got %T (%v)", tc.verdict, err, err)
			}
			if exitErr.Code != tc.wantCode {
				t.Fatalf("verdict %q: exit code = %d, want %d", tc.verdict, exitErr.Code, tc.wantCode)
			}
		})
	}
}

func TestCompareGateStructuredOutputPreservesFullPayload(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": jsonHandler(200, map[string]any{
			"baseline_run_id":  "run-b",
			"candidate_run_id": "run-c",
			"release_gate": map[string]any{
				"id":                 "gate-1",
				"run_comparison_id":  "cmp-1",
				"policy_key":         "default",
				"policy_version":     3,
				"policy_fingerprint": "abc123",
				"policy_snapshot":    map[string]any{"threshold": 0.9},
				"verdict":            "pass",
				"reason_code":        "all_dims_pass",
				"summary":            "all dimensions pass",
				"evidence_status":    "complete",
				"evaluation_details": map[string]any{"dimensions": []string{"correctness"}},
				"generated_at":       "2026-04-19T12:00:00Z",
				"updated_at":         "2026-04-19T12:00:01Z",
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"compare", "gate",
		"--baseline", "run-b",
		"--candidate", "run-c",
		"--json",
	}, srv.URL)
	if err != nil {
		t.Fatalf("compare gate --json: %v", err)
	}

	out := stdout.finish()
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON output: %v\n---\n%s", err, out)
	}
	gate, ok := payload["release_gate"].(map[string]any)
	if !ok {
		t.Fatalf("missing release_gate in output: %v", payload)
	}
	for _, key := range []string{"id", "policy_key", "policy_version", "policy_fingerprint", "policy_snapshot", "evaluation_details", "generated_at", "updated_at"} {
		if _, present := gate[key]; !present {
			t.Fatalf("structured output dropped %q (got keys: %v)", key, keysOf(gate))
		}
	}
}

func TestCompareGateSanitizesHostileSummary(t *testing.T) {
	// A compromised backend can't be allowed to:
	// (1) move the cursor / clear the screen / smuggle OSC hyperlinks, or
	// (2) inject extra lines via \n in the single-line gate summary — which
	//     would let it forge something like "passed\nerror: already approved".
	hostile := "OWNED\x1b[2J\x1b[H\x07\nerror: forged-line"
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": jsonHandler(200, map[string]any{
			"release_gate": map[string]any{
				"verdict":         "fail",
				"summary":         hostile,
				"reason_code":     "evil\x1breason\ttwo",
				"evidence_status": "evil\x07status",
			},
		}),
	})
	defer srv.Close()

	stderr := captureStderr(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	_ = executeCommand(t, []string{
		"compare", "gate", "--baseline", "a", "--candidate", "b",
	}, srv.URL)

	got := stderr.finish()
	for _, forbidden := range []string{"\x1b", "\x07"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("stderr leaked control byte %q: %q", forbidden, got)
		}
	}
	// The gate summary must render as a single line per verdict message —
	// a verdict header plus at most one reason/evidence follow-up line.
	// If the forged-line snippet appears on its own line, the injection
	// succeeded.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "error: forged-line") {
			t.Fatalf("stderr has forged newline-injected line: %q", got)
		}
	}
	if !strings.Contains(got, "OWNED") {
		t.Fatalf("stderr lost the printable portion of summary: %q", got)
	}
}

type streamCapture struct {
	t       *testing.T
	target  **os.File
	original *os.File
	r, w    *os.File
	done    chan string
}

func captureStdout(t *testing.T) *streamCapture {
	t.Helper()
	return beginCapture(t, &os.Stdout)
}

func captureStderr(t *testing.T) *streamCapture {
	t.Helper()
	return beginCapture(t, &os.Stderr)
}

func beginCapture(t *testing.T, target **os.File) *streamCapture {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	sc := &streamCapture{t: t, target: target, original: *target, r: r, w: w, done: make(chan string, 1)}
	*target = w
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		sc.done <- buf.String()
	}()
	return sc
}

func (sc *streamCapture) finish() string {
	_ = sc.w.Close()
	out := <-sc.done
	*sc.target = sc.original
	_ = sc.r.Close()
	return out
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestCompareGateRejectsUnknownVerdict(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/release-gates/evaluate": jsonHandler(200, map[string]any{
			"release_gate": map[string]any{"verdict": "unheard_of"},
		}),
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"compare", "gate", "--baseline", "a", "--candidate", "b",
	}, srv.URL)

	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %T (%v)", err, err)
	}
	if exitErr.Code != gateExitFail {
		t.Fatalf("unknown verdict should map to fail exit code, got %d", exitErr.Code)
	}
}
