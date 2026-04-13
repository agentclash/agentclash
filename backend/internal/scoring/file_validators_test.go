package scoring

import (
	"encoding/json"
	"testing"
)

func TestValidateFileExists_MustExistTrue_FileExists(t *testing.T) {
	outcome := validateFileExists("some content", json.RawMessage(`{"must_exist": true}`))
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass", outcome.verdict)
	}
}

func TestValidateFileExists_MustExistTrue_FileDoesNotExist(t *testing.T) {
	// When file doesn't exist, evaluateValidators calls validateFileExistsUnavailable.
	outcome := validateFileExistsUnavailable(json.RawMessage(`{"must_exist": true}`))
	if outcome.verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateFileExists_MustExistFalse_FileDoesNotExist(t *testing.T) {
	outcome := validateFileExistsUnavailable(json.RawMessage(`{"must_exist": false}`))
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass", outcome.verdict)
	}
}

func TestValidateFileExists_MustExistFalse_FileExists(t *testing.T) {
	outcome := validateFileExists("some content", json.RawMessage(`{"must_exist": false}`))
	if outcome.verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateFileExists_DefaultConfigMeansExists(t *testing.T) {
	outcome := validateFileExists("content", nil)
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass (default must_exist=true, file exists)", outcome.verdict)
	}
}

func TestValidateFileContentMatch_Exact(t *testing.T) {
	config := json.RawMessage(`{"match_mode": "exact"}`)
	outcome := validateFileContentMatch("hello world", "hello world", config)
	if outcome.verdict != "pass" {
		t.Fatalf("exact match: verdict = %q, want pass", outcome.verdict)
	}

	outcome = validateFileContentMatch("hello world!", "hello world", config)
	if outcome.verdict != "fail" {
		t.Fatalf("exact mismatch: verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateFileContentMatch_Contains(t *testing.T) {
	config := json.RawMessage(`{"match_mode": "contains"}`)
	outcome := validateFileContentMatch("the quick brown fox", "brown fox", config)
	if outcome.verdict != "pass" {
		t.Fatalf("contains match: verdict = %q, want pass", outcome.verdict)
	}

	outcome = validateFileContentMatch("the quick brown fox", "lazy dog", config)
	if outcome.verdict != "fail" {
		t.Fatalf("contains mismatch: verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateFileContentMatch_Regex(t *testing.T) {
	config := json.RawMessage(`{"match_mode": "regex"}`)
	outcome := validateFileContentMatch("status: 200 OK", `status:\s+\d+ OK`, config)
	if outcome.verdict != "pass" {
		t.Fatalf("regex match: verdict = %q, want pass", outcome.verdict)
	}

	outcome = validateFileContentMatch("status: error", `status:\s+\d+ OK`, config)
	if outcome.verdict != "fail" {
		t.Fatalf("regex mismatch: verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateFileContentMatch_NotContains(t *testing.T) {
	config := json.RawMessage(`{"match_mode": "not_contains"}`)
	outcome := validateFileContentMatch("clean code here", "dead_code_function", config)
	if outcome.verdict != "pass" {
		t.Fatalf("not_contains (absent): verdict = %q, want pass", outcome.verdict)
	}

	outcome = validateFileContentMatch("keep dead_code_function around", "dead_code_function", config)
	if outcome.verdict != "fail" {
		t.Fatalf("not_contains (present): verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateFileContentMatch_JSONEqual(t *testing.T) {
	actual := `{"b": 2, "a": 1}`
	expected := `{"a":1,"b":2}`
	config := json.RawMessage(`{"match_mode": "json_equal"}`)
	outcome := validateFileContentMatch(actual, expected, config)
	if outcome.verdict != "pass" {
		t.Fatalf("json_equal match: verdict = %q, want pass", outcome.verdict)
	}

	outcome = validateFileContentMatch(`{"a": 1}`, `{"a": 2}`, config)
	if outcome.verdict != "fail" {
		t.Fatalf("json_equal mismatch: verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateFileContentMatch_IgnoreWhitespace(t *testing.T) {
	config := json.RawMessage(`{"match_mode": "exact", "ignore_whitespace": true}`)
	outcome := validateFileContentMatch("hello   world", "hello world", config)
	if outcome.verdict != "pass" {
		t.Fatalf("ignore_whitespace: verdict = %q, want pass", outcome.verdict)
	}
}

func TestValidateFileContentMatch_DefaultModeIsContains(t *testing.T) {
	outcome := validateFileContentMatch("the answer is 42", "42", nil)
	if outcome.verdict != "pass" {
		t.Fatalf("default contains: verdict = %q, want pass", outcome.verdict)
	}
}

func TestValidateFileJSONSchema_Valid(t *testing.T) {
	actual := `{"status": "success", "results": [1, 2]}`
	config := json.RawMessage(`{
		"schema": {
			"type": "object",
			"required": ["status", "results"],
			"properties": {
				"status": {"type": "string", "enum": ["success", "partial", "failure"]},
				"results": {"type": "array"}
			}
		}
	}`)
	outcome := validateFileJSONSchema(actual, config)
	if outcome.verdict != "pass" {
		t.Fatalf("json_schema valid: verdict = %q, reason = %q", outcome.verdict, outcome.reason)
	}
}

func TestValidateFileJSONSchema_Invalid(t *testing.T) {
	actual := `{"name": "test"}`
	config := json.RawMessage(`{
		"schema": {
			"type": "object",
			"required": ["status", "name"],
			"properties": {
				"status": {"type": "string"},
				"name": {"type": "string"}
			}
		}
	}`)
	outcome := validateFileJSONSchema(actual, config)
	if outcome.verdict != "fail" {
		t.Fatalf("json_schema invalid: verdict = %q, want fail (missing required field)", outcome.verdict)
	}
}

func TestValidateFileJSONSchema_MissingConfig(t *testing.T) {
	outcome := validateFileJSONSchema(`{}`, nil)
	if outcome.verdict != "error" {
		t.Fatalf("missing config: verdict = %q, want error", outcome.verdict)
	}
}

func TestValidateDirectoryStructure_AllPresent(t *testing.T) {
	listing := DirectoryListingResult{
		Key:  "project_structure",
		Path: "/workspace/project/",
		Entries: []DirectoryEntry{
			{Path: "/workspace/project/src/main.py", Size: 100},
			{Path: "/workspace/project/tests/test_main.py", Size: 200},
			{Path: "/workspace/project/requirements.txt", Size: 50},
			{Path: "/workspace/project/src/", Size: 0, IsDir: true},
			{Path: "/workspace/project/tests/", Size: 0, IsDir: true},
		},
	}
	actual, _ := json.Marshal(listing)
	config := json.RawMessage(`{
		"required_files": ["src/main.py", "tests/test_main.py", "requirements.txt"],
		"forbidden_files": [".env", "src/main.py.bak"],
		"required_directories": ["src/", "tests/"]
	}`)
	outcome := validateDirectoryStructure(string(actual), config)
	if outcome.verdict != "pass" {
		t.Fatalf("all present: verdict = %q, reason = %q", outcome.verdict, outcome.reason)
	}
}

func TestValidateDirectoryStructure_MissingRequired(t *testing.T) {
	listing := DirectoryListingResult{
		Key:  "project_structure",
		Path: "/workspace/project/",
		Entries: []DirectoryEntry{
			{Path: "/workspace/project/src/main.py", Size: 100},
		},
	}
	actual, _ := json.Marshal(listing)
	config := json.RawMessage(`{
		"required_files": ["src/main.py", "tests/test_main.py"]
	}`)
	outcome := validateDirectoryStructure(string(actual), config)
	if outcome.verdict != "fail" {
		t.Fatalf("missing required: verdict = %q, want fail", outcome.verdict)
	}
	if outcome.reason == "" {
		t.Fatal("expected a reason describing what's missing")
	}
}

func TestValidateDirectoryStructure_ForbiddenPresent(t *testing.T) {
	listing := DirectoryListingResult{
		Key:  "project_structure",
		Path: "/workspace/project/",
		Entries: []DirectoryEntry{
			{Path: "/workspace/project/.env", Size: 100},
			{Path: "/workspace/project/src/main.py", Size: 200},
		},
	}
	actual, _ := json.Marshal(listing)
	config := json.RawMessage(`{
		"forbidden_files": [".env"]
	}`)
	outcome := validateDirectoryStructure(string(actual), config)
	if outcome.verdict != "fail" {
		t.Fatalf("forbidden present: verdict = %q, want fail", outcome.verdict)
	}
}

func TestValidateDirectoryStructure_MissingConfig(t *testing.T) {
	outcome := validateDirectoryStructure(`{}`, nil)
	if outcome.verdict != "error" {
		t.Fatalf("missing config: verdict = %q, want error", outcome.verdict)
	}
}
