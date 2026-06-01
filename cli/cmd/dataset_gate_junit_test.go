package cmd

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintDatasetGateJUnitFailure(t *testing.T) {
	exampleID := "11111111-1111-1111-1111-111111111111"
	result := map[string]any{
		"gate": map[string]any{
			"pass":              false,
			"pass_rate":         0.5,
			"regression_count":  1,
			"regressions": []any{
				map[string]any{
					"dataset_example_id": exampleID,
					"reason":             "verdict_regressed",
					"baseline_verdict":   "pass",
					"candidate_verdict":  "fail",
				},
			},
		},
	}

	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	exitCode := -1
	exitFn := func(code int) { exitCode = code }
	oldExit := datasetGateExit
	datasetGateExit = exitFn
	defer func() {
		os.Stdout = origStdout
		datasetGateExit = oldExit
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- printDatasetGateJUnit(result, 1)
		w.Close()
	}()
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("copy stdout: %v", copyErr)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("printDatasetGateJUnit() error = %v", err)
	}
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}

	output := buf.String()
	if !strings.HasPrefix(output, xml.Header) {
		t.Fatalf("output missing xml header:\n%s", output)
	}
	if !strings.Contains(output, exampleID) {
		t.Fatalf("output missing example id:\n%s", output)
	}
	if !strings.Contains(output, `failures="1"`) {
		t.Fatalf("output missing failure count:\n%s", output)
	}
}

func TestPrintDatasetGateJUnitCountsRegressionsAndThresholds(t *testing.T) {
	result := map[string]any{
		"gate": map[string]any{
			"pass":              false,
			"pass_rate":         0.5,
			"regression_count":  1,
			"regressions": []any{
				map[string]any{
					"dataset_example_id": "11111111-1111-1111-1111-111111111111",
					"reason":             "verdict_regressed",
					"baseline_verdict":   "pass",
					"candidate_verdict":  "fail",
				},
			},
			"failed_thresholds": []any{"max_regressions"},
		},
	}

	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	oldExit := datasetGateExit
	datasetGateExit = func(int) {}
	defer func() {
		os.Stdout = origStdout
		datasetGateExit = oldExit
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- printDatasetGateJUnit(result, 1)
		w.Close()
	}()
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("copy stdout: %v", copyErr)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("printDatasetGateJUnit() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `tests="2"`) {
		t.Fatalf("output missing combined test count:\n%s", output)
	}
	if !strings.Contains(output, `failures="2"`) {
		t.Fatalf("output missing combined failure count:\n%s", output)
	}
}
