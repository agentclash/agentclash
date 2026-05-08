package scoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRetrievalHitValidator_PassesWhenExpectedDocumentAppearsInTopK(t *testing.T) {
	outcome := validateRetrievalHit(ragFinalOutput(), `["policy-travel"]`, json.RawMessage(`{"k":2}`))
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass; reason=%s", outcome.verdict, outcome.reason)
	}
	if outcome.normalizedScore == nil || *outcome.normalizedScore != 1 {
		t.Fatalf("score = %v, want 1", outcome.normalizedScore)
	}
	if got := outcome.evidence["matched_ids"]; !containsStringValue(got, "policy-travel") {
		t.Fatalf("matched_ids = %#v, want policy-travel", got)
	}
}

func TestRetrievalHitValidator_FailsWhenExpectedDocumentOutsideTopK(t *testing.T) {
	outcome := validateRetrievalHit(ragFinalOutput(), `["policy-security"]`, json.RawMessage(`{"k":2}`))
	if outcome.verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", outcome.verdict)
	}
	if outcome.normalizedScore == nil || *outcome.normalizedScore != 0 {
		t.Fatalf("score = %v, want 0", outcome.normalizedScore)
	}
}

func TestRetrievalPrecisionValidator_ComputesPartialCredit(t *testing.T) {
	outcome := validateRetrievalPrecision(ragFinalOutput(), `["policy-travel"]`, json.RawMessage(`{"k":2,"pass_at":0.5}`))
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass; reason=%s", outcome.verdict, outcome.reason)
	}
	if outcome.normalizedScore == nil || *outcome.normalizedScore != 0.5 {
		t.Fatalf("score = %v, want 0.5", outcome.normalizedScore)
	}
	if got := outcome.evidence["considered_chunks"]; got != 2 {
		t.Fatalf("considered_chunks = %#v, want 2", got)
	}
}

func TestRetrievalPrecisionValidator_PassFailFromThreshold(t *testing.T) {
	outcome := validateRetrievalPrecision(ragFinalOutput(), `["policy-travel"]`, json.RawMessage(`{"k":2,"pass_at":0.75}`))
	if outcome.verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", outcome.verdict)
	}
	if !strings.Contains(outcome.reason, "below pass_at") {
		t.Fatalf("reason = %q, want threshold failure", outcome.reason)
	}
}

func TestRetrievalHitValidator_UsesExpectedIdsPathForCitationStyleChecks(t *testing.T) {
	outcome := validateRetrievalHit(ragFinalOutput(), ragFinalOutput(), json.RawMessage(`{"expected_ids_path":"$.citations","k":2}`))
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass; reason=%s", outcome.verdict, outcome.reason)
	}
}

func TestRetrievalHitValidator_IgnoresRankFieldAndUsesArrayOrder(t *testing.T) {
	actual := `{
		"retrieved_chunks": [
			{"chunk_id":"wrong-top","document_id":"wrong","rank":99,"score":0.1},
			{"chunk_id":"right-second","document_id":"right","rank":1,"score":0.99}
		]
	}`
	outcome := validateRetrievalHit(actual, `["right"]`, json.RawMessage(`{"k":1}`))
	if outcome.verdict != "fail" {
		t.Fatalf("verdict = %q, want fail because array order, not rank field, controls top-k", outcome.verdict)
	}
}

func TestRetrievalHitValidator_MatchesConfiguredIDFields(t *testing.T) {
	actual := `{"retrieved_chunks":[{"source_id":"source-7","chunk_id":"ignored"}]}`
	outcome := validateRetrievalHit(actual, `["source-7"]`, json.RawMessage(`{"id_fields":["source_id"]}`))
	if outcome.verdict != "pass" {
		t.Fatalf("verdict = %q, want pass; reason=%s", outcome.verdict, outcome.reason)
	}
}

func TestRetrievalHitValidator_FailsWhenExpectedIDsAreAbsent(t *testing.T) {
	outcome := validateRetrievalHit(ragFinalOutput(), `["policy-missing"]`, nil)
	if outcome.verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", outcome.verdict)
	}
	if got := outcome.evidence["missing_ids"]; !containsStringValue(got, "policy-missing") {
		t.Fatalf("missing_ids = %#v, want policy-missing", got)
	}
}

func TestRetrievalValidators_ReturnErrorForMalformedJSON(t *testing.T) {
	outcome := validateRetrievalHit(`{"retrieved_chunks":`, `["policy-travel"]`, nil)
	if outcome.verdict != "error" {
		t.Fatalf("verdict = %q, want error", outcome.verdict)
	}
	if !strings.Contains(outcome.reason, "parse actual retrieval JSON") {
		t.Fatalf("reason = %q, want parse error", outcome.reason)
	}
}

func TestRetrievalValidators_ReturnUnavailableForEmptyRetrievedChunks(t *testing.T) {
	outcome := validateRetrievalHit(`{"retrieved_chunks":[]}`, `["policy-travel"]`, nil)
	if outcome.state != OutputStateUnavailable {
		t.Fatalf("state = %s, want unavailable", outcome.state)
	}
	if !strings.Contains(outcome.reason, "retrieved chunks are unavailable") {
		t.Fatalf("reason = %q, want unavailable reason", outcome.reason)
	}
}

func TestValidateEvaluationSpec_RAGValidatorConfig(t *testing.T) {
	tests := []struct {
		name    string
		vtype   ValidatorType
		config  json.RawMessage
		wantSub string
	}{
		{name: "negative k", vtype: ValidatorTypeRetrievalHit, config: json.RawMessage(`{"k":-1}`), wantSub: ".k"},
		{name: "zero pass_at", vtype: ValidatorTypeRetrievalPrecision, config: json.RawMessage(`{"pass_at":0}`), wantSub: ".pass_at"},
		{name: "invalid pass_at", vtype: ValidatorTypeRetrievalPrecision, config: json.RawMessage(`{"pass_at":1.1}`), wantSub: ".pass_at"},
		{name: "empty id fields", vtype: ValidatorTypeRetrievalHit, config: json.RawMessage(`{"id_fields":[]}`), wantSub: ".id_fields"},
		{name: "invalid path", vtype: ValidatorTypeRetrievalHit, config: json.RawMessage(`{"retrieved_chunks_path":"retrieved_chunks"}`), wantSub: ".retrieved_chunks_path"},
		{name: "unknown field", vtype: ValidatorTypeRetrievalHit, config: json.RawMessage(`{"unknown":true}`), wantSub: "unknown field"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := retrievalSpec(tt.vtype, tt.config)
			err := ValidateEvaluationSpec(spec)
			if err == nil {
				t.Fatalf("ValidateEvaluationSpec returned nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestEvaluateRunAgent_RAGRetrievalValidatorsScoreFinalOutputEnvelope(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "rag-retrieval",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{
				Key:          "hit",
				Type:         ValidatorTypeRetrievalHit,
				Target:       "final_output",
				ExpectedFrom: "literal:[\"policy-travel\"]",
				Config:       json.RawMessage(`{"k":2}`),
			},
			{
				Key:          "precision",
				Type:         ValidatorTypeRetrievalPrecision,
				Target:       "final_output",
				ExpectedFrom: "literal:[\"policy-travel\"]",
				Config:       json.RawMessage(`{"k":2,"pass_at":0.5}`),
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "retrieval", Source: DimensionSourceValidators, Validators: []string{"hit", "precision"}},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events: []Event{
			{Type: "system.output.finalized", SequenceNumber: 9, OccurredAt: time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC), Payload: []byte(`{"final_output":` + quoteJSON(ragFinalOutput()) + `}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	if evaluation.ValidatorResults[0].State != OutputStateAvailable || evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("hit result = state %s verdict %q, want available/pass", evaluation.ValidatorResults[0].State, evaluation.ValidatorResults[0].Verdict)
	}
	if evaluation.ValidatorResults[0].Source == nil || evaluation.ValidatorResults[0].Source.EventType != "system.output.finalized" {
		t.Fatalf("source = %#v, want finalized event source", evaluation.ValidatorResults[0].Source)
	}
	if evaluation.DimensionScores["retrieval"] == nil || *evaluation.DimensionScores["retrieval"] != 0.75 {
		t.Fatalf("retrieval dimension score = %v, want 0.75", evaluation.DimensionScores["retrieval"])
	}
}

func retrievalSpec(vtype ValidatorType, config json.RawMessage) EvaluationSpec {
	return EvaluationSpec{
		Name:          "rag-config",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "rag", Type: vtype, Target: "final_output", ExpectedFrom: "literal:[\"policy-travel\"]", Config: config},
		},
		Scorecard: ScorecardDeclaration{Dimensions: []DimensionDeclaration{{Key: "correctness"}}},
	}
}

func ragFinalOutput() string {
	return `{
		"answer": "Employees may expense Uber after 10pm if policy conditions are met.",
		"retrieved_chunks": [
			{"chunk_id":"travel-policy#chunk-03","document_id":"policy-travel","rank":3,"score":0.91},
			{"chunk_id":"benefits-policy#chunk-01","document_id":"policy-benefits","rank":1,"score":0.99},
			{"chunk_id":"security-policy#chunk-09","document_id":"policy-security","rank":2,"score":0.95}
		],
		"citations": ["travel-policy#chunk-03"]
	}`
}

func quoteJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func containsStringValue(value any, want string) bool {
	items, ok := value.([]string)
	if ok {
		for _, item := range items {
			if item == want {
				return true
			}
		}
		return false
	}
	anyItems, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range anyItems {
		if item == want {
			return true
		}
	}
	return false
}
