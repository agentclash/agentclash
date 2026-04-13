package scoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResolveFileCaptureEvidence_FileExists(t *testing.T) {
	evidence := extractedEvidence{
		capturedFiles: map[string]FileCaptureResult{
			"app_py": {
				Key:     "app_py",
				Path:    "/workspace/app.py",
				Exists:  true,
				Content: "print('hello')",
				Size:    14,
			},
		},
	}
	value, _, reason, err := resolveFileCaptureEvidence("app_py", evidence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason != "" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if value == nil || *value != "print('hello')" {
		t.Fatalf("value = %v, want %q", value, "print('hello')")
	}
}

func TestResolveFileCaptureEvidence_FileDoesNotExist(t *testing.T) {
	evidence := extractedEvidence{
		capturedFiles: map[string]FileCaptureResult{
			"missing": {
				Key:    "missing",
				Path:   "/workspace/nope.txt",
				Exists: false,
			},
		},
	}
	value, _, reason, err := resolveFileCaptureEvidence("missing", evidence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != nil {
		t.Fatalf("value should be nil for non-existent file, got %q", *value)
	}
	if reason == "" {
		t.Fatal("expected a reason for non-existent file")
	}
}

func TestResolveFileCaptureEvidence_KeyNotFound(t *testing.T) {
	evidence := extractedEvidence{}
	value, _, reason, err := resolveFileCaptureEvidence("unknown_key", evidence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != nil {
		t.Fatalf("value should be nil for unknown key, got %q", *value)
	}
	if reason == "" {
		t.Fatal("expected a reason for unknown key")
	}
}

func TestResolveFileCaptureEvidence_DirectoryListing(t *testing.T) {
	evidence := extractedEvidence{
		capturedDirListings: map[string]DirectoryListingResult{
			"project_dir": {
				Key:  "project_dir",
				Path: "/workspace/",
				Entries: []DirectoryEntry{
					{Path: "/workspace/main.py", Size: 100},
				},
			},
		},
	}
	value, _, reason, err := resolveFileCaptureEvidence("project_dir", evidence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason != "" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if value == nil {
		t.Fatal("expected non-nil value for directory listing")
	}
	// Should be valid JSON
	var parsed DirectoryListingResult
	if err := json.Unmarshal([]byte(*value), &parsed); err != nil {
		t.Fatalf("directory listing value is not valid JSON: %v", err)
	}
	if len(parsed.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(parsed.Entries))
	}
}

func TestBuildEvidence_ExtractsFileCaptureEvents(t *testing.T) {
	capturePayload, _ := json.Marshal(FileCaptureResult{
		Key:     "output_json",
		Path:    "/workspace/output.json",
		Exists:  true,
		Content: `{"result": "ok"}`,
		Size:    16,
	})

	events := []Event{
		{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
		{Type: "grader.verification.file_captured", OccurredAt: time.Date(2026, 4, 1, 0, 0, 1, 0, time.UTC), Payload: capturePayload},
		{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 1, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
	}

	evidence := buildEvidence(nil, events)
	if evidence.capturedFiles == nil {
		t.Fatal("capturedFiles should not be nil")
	}
	capture, ok := evidence.capturedFiles["output_json"]
	if !ok {
		t.Fatal("expected output_json in capturedFiles")
	}
	if !capture.Exists {
		t.Fatal("expected file to exist")
	}
	if capture.Content != `{"result": "ok"}` {
		t.Fatalf("content = %q, want %q", capture.Content, `{"result": "ok"}`)
	}
}

func TestBuildEvidence_ExtractsDirectoryListingEvents(t *testing.T) {
	listingPayload, _ := json.Marshal(DirectoryListingResult{
		Key:  "project_structure",
		Path: "/workspace/",
		Entries: []DirectoryEntry{
			{Path: "/workspace/main.py", Size: 100},
			{Path: "/workspace/tests/", Size: 0, IsDir: true},
		},
	})

	events := []Event{
		{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
		{Type: "grader.verification.directory_listed", OccurredAt: time.Date(2026, 4, 1, 0, 0, 1, 0, time.UTC), Payload: listingPayload},
		{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 1, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
	}

	evidence := buildEvidence(nil, events)
	if evidence.capturedDirListings == nil {
		t.Fatal("capturedDirListings should not be nil")
	}
	listing, ok := evidence.capturedDirListings["project_structure"]
	if !ok {
		t.Fatal("expected project_structure in capturedDirListings")
	}
	if len(listing.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(listing.Entries))
	}
}

func TestEvaluateRunAgent_WithFileValidators(t *testing.T) {
	capturePayload, _ := json.Marshal(FileCaptureResult{
		Key:     "app_py_content",
		Path:    "/workspace/app.py",
		Exists:  true,
		Content: "def main():\n    print('fixed')\n",
		Size:    30,
	})

	spec := EvaluationSpec{
		Name:          "file-check-fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "app_fixed",
				Type:         ValidatorTypeFileContentMatch,
				Target:       "file:app_py_content",
				ExpectedFrom: "literal:fixed",
				Config:       json.RawMessage(`{"match_mode": "contains"}`),
			},
			{
				Key:    "app_exists",
				Type:   ValidatorTypeFileExists,
				Target: "file:app_py_content",
				Config: json.RawMessage(`{"must_exist": true}`),
			},
		},
		PostExecutionChecks: []PostExecutionCheck{
			{Key: "app_py_content", Type: PostExecutionCheckTypeFileCapture, Path: "/workspace/app.py"},
		},
		Metrics: []MetricDeclaration{
			{Key: "completed", Type: MetricTypeBoolean, Collector: "run_completed_successfully"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}, {Key: ScorecardDimensionReliability}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "grader.verification.file_captured", OccurredAt: time.Date(2026, 4, 1, 0, 0, 1, 0, time.UTC), Payload: capturePayload},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 1, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done","total_tokens":5}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if len(evaluation.ValidatorResults) != 2 {
		t.Fatalf("validator results = %d, want 2", len(evaluation.ValidatorResults))
	}

	// file_content_match
	contentResult := evaluation.ValidatorResults[0]
	if contentResult.Verdict != "pass" {
		t.Fatalf("file_content_match verdict = %q, want pass (reason: %s)", contentResult.Verdict, contentResult.Reason)
	}

	// file_exists
	existsResult := evaluation.ValidatorResults[1]
	if existsResult.Verdict != "pass" {
		t.Fatalf("file_exists verdict = %q, want pass (reason: %s)", existsResult.Verdict, existsResult.Reason)
	}

	if evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] == nil || *evaluation.DimensionScores[string(ScorecardDimensionCorrectness)] != 1 {
		t.Fatalf("correctness = %v, want 1.0", evaluation.DimensionScores[string(ScorecardDimensionCorrectness)])
	}
}

func TestEvaluateRunAgent_FileExistsFail_WhenMissing(t *testing.T) {
	capturePayload, _ := json.Marshal(FileCaptureResult{
		Key:    "missing_file",
		Path:   "/workspace/missing.py",
		Exists: false,
	})

	spec := EvaluationSpec{
		Name:          "file-missing-fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:    "file_must_exist",
				Type:   ValidatorTypeFileExists,
				Target: "file:missing_file",
				Config: json.RawMessage(`{"must_exist": true}`),
			},
		},
		PostExecutionChecks: []PostExecutionCheck{
			{Key: "missing_file", Type: PostExecutionCheckTypeFileCapture, Path: "/workspace/missing.py"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "grader.verification.file_captured", OccurredAt: time.Date(2026, 4, 1, 0, 0, 1, 0, time.UTC), Payload: capturePayload},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 1, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if len(evaluation.ValidatorResults) != 1 {
		t.Fatalf("validator results = %d, want 1", len(evaluation.ValidatorResults))
	}
	result := evaluation.ValidatorResults[0]
	if result.Verdict != "fail" {
		t.Fatalf("file_exists verdict = %q, want fail (file doesn't exist)", result.Verdict)
	}
	if result.NormalizedScore == nil || *result.NormalizedScore != 0 {
		t.Fatalf("normalized_score = %v, want 0", result.NormalizedScore)
	}
}

func TestEvaluateRunAgent_NoCaptureEvents_ExistingPacksUnaffected(t *testing.T) {
	// Existing packs without post_execution_checks should behave identically.
	spec := EvaluationSpec{
		Name:          "legacy-fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "exact",
				Type:         ValidatorTypeExactMatch,
				Target:       "final_output",
				ExpectedFrom: "challenge_input",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{ChallengeIdentityID: uuid.New(), ItemKey: "test", Payload: []byte(`"42"`)},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 4, 1, 0, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"42"}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	if evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("legacy pack verdict = %q, want pass", evaluation.ValidatorResults[0].Verdict)
	}
}

func TestPostExecutionCheck_EffectiveMaxSizeBytes(t *testing.T) {
	check := PostExecutionCheck{Key: "test", Type: "file_capture", Path: "/test"}
	if got := check.EffectiveMaxSizeBytes(); got != DefaultMaxFileSizeBytes {
		t.Fatalf("default max size = %d, want %d", got, DefaultMaxFileSizeBytes)
	}

	check.MaxSizeBytes = 500
	if got := check.EffectiveMaxSizeBytes(); got != 500 {
		t.Fatalf("custom max size = %d, want 500", got)
	}
}
