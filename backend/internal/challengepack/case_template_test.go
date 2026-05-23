package challengepack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/scoring"
)

func TestBuildCaseTemplateContext_MergesPayloadAndInputs(t *testing.T) {
	ctx := BuildCaseTemplateContext(map[string]any{
		"order_id": "from-payload",
		"nested": map[string]any{
			"id": "payload-nested",
		},
	}, []CaseInput{
		{Key: "order_id", Kind: "text", Value: "from-input"},
		{Key: "language", Kind: "text", Value: "French"},
	})

	if ctx["order_id"] != "from-input" {
		t.Fatalf("order_id = %v, want from-input", ctx["order_id"])
	}
	if ctx["language"] != "French" {
		t.Fatalf("language = %v, want French", ctx["language"])
	}
	nested, ok := ctx["nested"].(map[string]any)
	if !ok || nested["id"] != "payload-nested" {
		t.Fatalf("nested.id = %#v, want payload-nested", ctx["nested"])
	}
}

func TestRenderCaseTemplate_ResolvesTopLevelAndNestedPaths(t *testing.T) {
	ctx := BuildCaseTemplateContext(map[string]any{
		"order_id": "ord-1",
		"customer": map[string]any{
			"id": "cust-9",
		},
	}, nil)

	got, err := RenderCaseTemplate("pytest tests/test_{{order_id}}.py --customer {{customer.id}}", ctx)
	if err != nil {
		t.Fatalf("RenderCaseTemplate returned error: %v", err)
	}
	want := "pytest tests/test_ord-1.py --customer cust-9"
	if got != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
}

func TestRenderCaseTemplate_StrictMissingKey(t *testing.T) {
	_, err := RenderCaseTemplate("echo {{missing}}", CaseTemplateContext{})
	if err == nil {
		t.Fatal("expected error for missing placeholder")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v, want missing placeholder mention", err)
	}
}

func TestRenderCaseTemplateLenient_LeavesUnresolved(t *testing.T) {
	got := RenderCaseTemplateLenient("echo {{missing}}", CaseTemplateContext{})
	if got != "echo {{missing}}" {
		t.Fatalf("rendered = %q, want literal placeholder preserved", got)
	}
}

func TestExtractCaseTemplatePlaceholders(t *testing.T) {
	keys := ExtractCaseTemplatePlaceholders("{{order_id}} and {{customer.id}} and {{order_id}}")
	if len(keys) != 2 || keys[0] != "order_id" || keys[1] != "customer.id" {
		t.Fatalf("keys = %#v, want [order_id customer.id]", keys)
	}
}

func TestValidateBundleCaseTemplates_RejectsUnresolvedPlaceholder(t *testing.T) {
	errs := validateBundleCaseTemplates(caseTemplateTestBundle(`pytest {{missing}}`))
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	if !strings.Contains(errs.Error(), "missing") {
		t.Fatalf("error = %v, want unresolved placeholder", errs)
	}
}

func TestValidateBundleCaseTemplates_AcceptsResolvedPlaceholder(t *testing.T) {
	errs := validateBundleCaseTemplates(caseTemplateTestBundle(`pytest {{order_id}}`))
	if len(errs) > 0 {
		t.Fatalf("validateBundleCaseTemplates returned errors: %v", errs)
	}
}

func TestValidateBundle_AcceptsCaseTemplateTestCommand(t *testing.T) {
	bundle := caseTemplateTestBundle(`pytest tests/test_{{order_id}}.py`)
	bundle.Version.EvaluationSpec.PostExecutionChecks = []scoring.PostExecutionCheck{
		{Key: "generated_code", Type: scoring.PostExecutionCheckTypeFileCapture, Path: "/workspace/app.py"},
	}
	if err := ValidateBundle(bundle); err != nil {
		t.Fatalf("ValidateBundle returned error: %v", err)
	}
}

func TestBuildCaseTemplateContextFromPayload_StoredCaseDocument(t *testing.T) {
	payload, err := json.Marshal(StoredCaseDocument{
		SchemaVersion: 1,
		CaseKey:       "case-1",
		Payload:       map[string]any{"order_id": "stored"},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	ctx, err := BuildCaseTemplateContextFromPayload(payload, nil)
	if err != nil {
		t.Fatalf("BuildCaseTemplateContextFromPayload returned error: %v", err)
	}
	rendered, err := RenderCaseTemplate("echo {{order_id}}", ctx)
	if err != nil {
		t.Fatalf("RenderCaseTemplate returned error: %v", err)
	}
	if rendered != "echo stored" {
		t.Fatalf("rendered = %q, want echo stored", rendered)
	}
}

func caseTemplateTestBundle(testCommand string) Bundle {
	bundle := Bundle{
		Pack: PackMetadata{Slug: "demo", Name: "Demo", Family: "demo"},
		Version: VersionMetadata{
			Number: 1,
			EvaluationSpec: scoring.EvaluationSpec{
				Name:          "demo",
				VersionNumber: 1,
				JudgeMode:     scoring.JudgeModeDeterministic,
				Validators: []scoring.ValidatorDeclaration{
					{
						Key:    "tests",
						Type:   scoring.ValidatorTypeCodeExecution,
						Target: "file:generated_code",
						Config: json.RawMessage(`{"test_command":` + jsonString(testCommand) + `,"scoring":"all_or_nothing"}`),
					},
				},
				Scorecard: scoring.ScorecardDeclaration{
					Dimensions: []scoring.DimensionDeclaration{{Key: scoring.ScorecardDimensionCorrectness}},
				},
			},
		},
		Challenges: []ChallengeDefinition{
			{Key: "c1", Title: "C1", Category: "demo", Difficulty: "easy"},
		},
		InputSets: []InputSetDefinition{
			{
				Key:  "default",
				Name: "Default",
				Cases: []CaseDefinition{
					{
						ChallengeKey: "c1",
						CaseKey:      "case-1",
						Payload:      map[string]any{"order_id": "123"},
					},
				},
			},
		},
	}
	return bundle
}

func jsonString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
