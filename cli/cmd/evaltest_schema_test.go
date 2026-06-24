package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestEvalReportSchemaAcceptsFixtures(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	schemaPath := filepath.Join(repoRoot, "schemas", "evaltest", "eval-report.schema.json")
	schema := loadEvalJSONSchema(t, schemaPath)

	fixtures := []struct {
		name       string
		fixture    string
		wantExit   int
	}{
		{name: "all pass", fixture: "all-pass.json", wantExit: 0},
		{name: "metric failure", fixture: "metric-failure.json", wantExit: 1},
		{name: "provider error", fixture: "provider-error.json", wantExit: 3},
		{name: "config error", fixture: "config-error.json", wantExit: 2},
		{name: "multi turn", fixture: "multi-turn.json", wantExit: 0},
	}

	for _, tt := range fixtures {
		t.Run(tt.name, func(t *testing.T) {
			fixturePath := filepath.Join(repoRoot, "schemas", "evaltest", "fixtures", tt.fixture)
			doc := loadEvalJSONDocument(t, fixturePath)
			if err := schema.Validate(doc); err != nil {
				t.Fatalf("schema rejected fixture %s: %v", tt.fixture, err)
			}

			report, ok := doc.(map[string]any)
			if !ok {
				t.Fatalf("fixture %s is not an object", tt.fixture)
			}
			exitRaw := report["exit_code"]
			var got int
			switch v := exitRaw.(type) {
			case float64:
				got = int(v)
			case int:
				got = v
			case int64:
				got = int(v)
			default:
				t.Fatalf("exit_code missing or wrong type in %s: %T", tt.fixture, exitRaw)
			}
			if got != tt.wantExit {
				t.Fatalf("exit_code = %d, want %d", got, tt.wantExit)
			}
		})
	}
}

func TestEvalReportSchemaRejectsUnknownVersion(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	schemaPath := filepath.Join(repoRoot, "schemas", "evaltest", "eval-report.schema.json")
	schema := loadEvalJSONSchema(t, schemaPath)

	fixturePath := filepath.Join(repoRoot, "schemas", "evaltest", "fixtures", "all-pass.json")
	doc := loadEvalJSONDocument(t, fixturePath)
	report, ok := doc.(map[string]any)
	if !ok {
		t.Fatal("fixture is not an object")
	}
	report["schema_version"] = 999

	if err := schema.Validate(report); err == nil {
		t.Fatal("expected schema to reject unknown schema_version")
	}
}

func TestAgentResultSchemaAcceptsMultiTurnFixture(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	schemaPath := filepath.Join(repoRoot, "schemas", "evaltest", "agent-result.schema.json")
	schema := loadEvalJSONSchema(t, schemaPath)

	fixturePath := filepath.Join(repoRoot, "schemas", "evaltest", "fixtures", "multi-turn.json")
	report := loadEvalJSONDocument(t, fixturePath)
	reportMap, ok := report.(map[string]any)
	if !ok {
		t.Fatal("report is not an object")
	}
	cases, ok := reportMap["cases"].([]any)
	if !ok || len(cases) == 0 {
		t.Fatal("expected cases array")
	}
	caseResult, ok := cases[0].(map[string]any)
	if !ok {
		t.Fatal("case result is not an object")
	}
	agentResult := caseResult["agent_result"]
	if err := schema.Validate(agentResult); err != nil {
		t.Fatalf("agent-result schema rejected multi-turn fixture: %v", err)
	}
}

func TestEvaltestExitCodesDocumented(t *testing.T) {
	want := map[int]string{
		0: "success",
		1: "assertion_failed",
		2: "config_error",
		3: "provider_error",
		4: "internal_error",
	}
	for code, name := range want {
		found := false
		for _, entry := range documentedExitCodes {
			if entry.Code == code {
				for _, cmd := range entry.Commands {
					if cmd == "evaltest run" {
						found = true
						break
					}
				}
			}
		}
		if !found {
			t.Fatalf("exit code %d (%s) not documented for evaltest run", code, name)
		}
	}
}

func loadEvalJSONSchema(t *testing.T, path string) *jsonschema.Resolved {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema %s: %v", path, err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal schema %s: %v", path, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve schema %s: %v", path, err)
	}
	return resolved
}

func loadEvalJSONDocument(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read document %s: %v", path, err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal document %s: %v", path, err)
	}
	return doc
}
