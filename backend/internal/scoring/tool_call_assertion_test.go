package scoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestToolCallAssertionPresenceAndArgumentFragment(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{{
			Key:    "submitted_answer",
			Type:   ValidatorTypeToolCallAssertion,
			Target: "tool_calls",
			Config: json.RawMessage(`{
				"tool_name": "submit",
				"arguments_contain": {"answer": {"value": "42"}}
			}`),
		}},
		[]Event{
			toolCallEvent(1, "tool.call.completed", "search", `{"query":"life"}`),
			toolCallEvent(2, "tool.call.completed", "submit", `{"answer":{"value":"42","secret":"SHOULD_NOT_LEAK"}}`),
		},
	)

	result := evaluation.ValidatorResults[0]
	if result.State != OutputStateAvailable || result.Verdict != "pass" {
		t.Fatalf("validator = state %s verdict %q reason %q", result.State, result.Verdict, result.Reason)
	}
	if result.Source == nil || result.Source.Kind != SourceKindToolCall || result.Source.Sequence == nil || *result.Source.Sequence != 2 {
		t.Fatalf("source = %+v, want tool-call source at sequence 2", result.Source)
	}
	if strings.Contains(string(result.RawOutput), "SHOULD_NOT_LEAK") {
		t.Fatalf("raw output leaked tool arguments: %s", result.RawOutput)
	}
	assertRawOutputField(t, result.RawOutput, "matched_count", float64(1))
}

func TestToolCallAssertionArgumentFragmentArraySubset(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{{
			Key:    "tagged_submit",
			Type:   ValidatorTypeToolCallAssertion,
			Target: "tool_calls",
			Config: json.RawMessage(`{
				"tool_name": "submit",
				"arguments_contain": {"tags": ["urgent"], "steps": [{"status": "done"}]}
			}`),
		}},
		[]Event{
			toolCallEvent(1, "tool.call.completed", "submit", `{
				"tags":["info","urgent"],
				"steps":[{"status":"queued"},{"status":"done","id":"final"}]
			}`),
		},
	)

	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("array subset fragment verdict = %q, want pass; reason=%q", got, evaluation.ValidatorResults[0].Reason)
	}
}

func TestToolCallAssertionArgumentFragmentArraySubsetConsumesMatches(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{{
			Key:    "duplicate_steps",
			Type:   ValidatorTypeToolCallAssertion,
			Target: "tool_calls",
			Config: json.RawMessage(`{
				"tool_name": "submit",
				"arguments_contain": {"steps": ["approve", "approve"]}
			}`),
		}},
		[]Event{
			toolCallEvent(1, "tool.call.completed", "submit", `{"steps":["approve"]}`),
		},
	)

	if got := evaluation.ValidatorResults[0].Verdict; got != "fail" {
		t.Fatalf("duplicate array subset verdict = %q, want fail; reason=%q", got, evaluation.ValidatorResults[0].Reason)
	}
}

func TestToolCallAssertionAbsence(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{
			{
				Key:    "no_exec",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"exec","must_call":false}`),
			},
			{
				Key:    "no_search",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"search","must_call":false}`),
			},
		},
		[]Event{
			toolCallEvent(1, "tool.call.completed", "search", `{"query":"x"}`),
		},
	)

	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("absence pass verdict = %q, want pass", got)
	}
	if got := evaluation.ValidatorResults[1].Verdict; got != "fail" {
		t.Fatalf("absence fail verdict = %q, want fail", got)
	}
	if !strings.Contains(evaluation.ValidatorResults[1].Reason, "forbidden matching tool call") {
		t.Fatalf("absence failure reason = %q", evaluation.ValidatorResults[1].Reason)
	}
}

func TestToolCallAssertionCountsAndOrder(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{
			{
				Key:    "read_count",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"read_file","min_count":2,"max_count":2}`),
			},
			{
				Key:    "expected_order",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"ordered_tools":["search","read_file","submit"],"order_mode":"subsequence"}`),
			},
			{
				Key:    "exact_order_fails",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"ordered_tools":["search","read_file","submit"],"order_mode":"exact"}`),
			},
			{
				Key:    "exact_submit_count_fails",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"submit","count":2}`),
			},
			{
				Key:    "zero_exec_count",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"exec","count":0}`),
			},
			{
				Key:    "zero_exec_min",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"exec","min_count":0}`),
			},
		},
		[]Event{
			toolCallEvent(1, "tool.call.completed", "search", `{"query":"x"}`),
			toolCallEvent(2, "tool.call.completed", "read_file", `{"path":"a.txt"}`),
			toolCallEvent(3, "tool.call.completed", "read_file", `{"path":"b.txt"}`),
			toolCallEvent(4, "tool.call.completed", "submit", `{"answer":"done"}`),
		},
	)

	wantVerdicts := map[string]string{
		"read_count":               "pass",
		"expected_order":           "pass",
		"exact_order_fails":        "fail",
		"exact_submit_count_fails": "fail",
		"zero_exec_count":          "pass",
		"zero_exec_min":            "pass",
	}
	for _, result := range evaluation.ValidatorResults {
		if result.Verdict != wantVerdicts[result.Key] {
			t.Fatalf("%s verdict = %q, want %q; reason=%q", result.Key, result.Verdict, wantVerdicts[result.Key], result.Reason)
		}
	}
}

func TestToolCallAssertionZeroCountFailsWhenToolObserved(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{{
			Key:    "zero_search_count",
			Type:   ValidatorTypeToolCallAssertion,
			Target: "tool_calls",
			Config: json.RawMessage(`{"tool_name":"search","count":0}`),
		}},
		[]Event{
			toolCallEvent(1, "tool.call.completed", "search", `{"query":"x"}`),
		},
	)

	result := evaluation.ValidatorResults[0]
	if result.Verdict != "fail" {
		t.Fatalf("zero count observed verdict = %q, want fail", result.Verdict)
	}
	if strings.Contains(result.Reason, "expected matching tool call was not observed") {
		t.Fatalf("zero count failure used implicit presence reason: %q", result.Reason)
	}
}

func TestToolCallAssertionPresenceAndCountsIgnoreFailedCalls(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{
			{
				Key:    "failed_submit_presence",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"submit","must_call":true}`),
			},
			{
				Key:    "failed_submit_count",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"submit","count":1}`),
			},
			{
				Key:    "failed_submit_zero_count",
				Type:   ValidatorTypeToolCallAssertion,
				Target: "tool_calls",
				Config: json.RawMessage(`{"tool_name":"submit","count":0}`),
			},
		},
		[]Event{
			toolCallEvent(1, "tool.call.failed", "submit", `{"answer":"done"}`),
		},
	)

	wantVerdicts := map[string]string{
		"failed_submit_presence":   "fail",
		"failed_submit_count":      "fail",
		"failed_submit_zero_count": "pass",
	}
	for _, result := range evaluation.ValidatorResults {
		if result.Verdict != wantVerdicts[result.Key] {
			t.Fatalf("%s verdict = %q, want %q; reason=%q", result.Key, result.Verdict, wantVerdicts[result.Key], result.Reason)
		}
	}
	assertRawOutputField(t, evaluation.ValidatorResults[0].RawOutput, "observed_count", float64(0))
	assertRawOutputField(t, evaluation.ValidatorResults[0].RawOutput, "failed_count", float64(1))
}

func TestToolCallAssertionExactOrderIgnoresFailedRetries(t *testing.T) {
	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{{
			Key:    "retry_order",
			Type:   ValidatorTypeToolCallAssertion,
			Target: "tool_calls",
			Config: json.RawMessage(`{"ordered_tools":["search","submit"],"order_mode":"exact"}`),
		}},
		[]Event{
			toolCallEvent(1, "tool.call.failed", "search", `{"query":"x"}`),
			toolCallEvent(2, "tool.call.completed", "search", `{"query":"x"}`),
			toolCallEvent(3, "tool.call.completed", "submit", `{"answer":"done"}`),
		},
	)

	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("retry exact order verdict = %q, want pass; reason=%q", got, evaluation.ValidatorResults[0].Reason)
	}
}

func TestToolCallAssertionOrdersBySequenceNumber(t *testing.T) {
	search := toolCallEvent(1, "tool.call.completed", "search", `{"query":"x"}`)
	submit := toolCallEvent(2, "tool.call.completed", "submit", `{"answer":"done"}`)
	search.OccurredAt = time.Date(2026, 5, 10, 12, 0, 2, 0, time.UTC)
	submit.OccurredAt = time.Date(2026, 5, 10, 12, 0, 1, 0, time.UTC)

	evaluation := evaluateToolCallAssertionSpec(t,
		[]ValidatorDeclaration{{
			Key:    "sequence_order",
			Type:   ValidatorTypeToolCallAssertion,
			Target: "tool_calls",
			Config: json.RawMessage(`{"ordered_tools":["search","submit"],"order_mode":"exact"}`),
		}},
		[]Event{submit, search},
	)

	if got := evaluation.ValidatorResults[0].Verdict; got != "pass" {
		t.Fatalf("sequence order verdict = %q, want pass; reason=%q", got, evaluation.ValidatorResults[0].Reason)
	}
}

func TestOrderedEvaluationEventsHandlesMixedSequences(t *testing.T) {
	unsequenced := Event{Type: "unsequenced", OccurredAt: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)}
	seqTwo := Event{Type: "seq-two", SequenceNumber: 2, OccurredAt: time.Date(2026, 5, 10, 12, 0, 1, 0, time.UTC)}
	seqOne := Event{Type: "seq-one", SequenceNumber: 1, OccurredAt: time.Date(2026, 5, 10, 12, 0, 2, 0, time.UTC)}

	ordered := orderedEvaluationEvents([]Event{unsequenced, seqTwo, seqOne})

	got := []string{ordered[0].Type, ordered[1].Type, ordered[2].Type}
	want := []string{"seq-one", "seq-two", "unsequenced"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered event types = %v, want %v", got, want)
		}
	}
}

func TestToolCallAssertionValidation(t *testing.T) {
	cases := []struct {
		name   string
		config string
		target string
		needle string
	}{
		{
			name:   "missing config",
			config: "",
			target: "tool_calls",
			needle: "tool_call_assertion config is required",
		},
		{
			name:   "wrong target",
			config: `{"tool_name":"submit"}`,
			target: "final_output",
			needle: `target must be "tool_calls"`,
		},
		{
			name:   "unknown field",
			config: `{"tool_name":"submit","typo":true}`,
			target: "tool_calls",
			needle: "unknown field",
		},
		{
			name:   "invalid count range",
			config: `{"tool_name":"submit","min_count":2,"max_count":1}`,
			target: "tool_calls",
			needle: "min_count must be less than or equal to max_count",
		},
		{
			name:   "invalid arguments fragment",
			config: `{"tool_name":"submit","arguments_contain":["answer"]}`,
			target: "tool_calls",
			needle: "arguments_contain must be a JSON object",
		},
		{
			name:   "invalid order mode",
			config: `{"ordered_tools":["search"],"order_mode":"before"}`,
			target: "tool_calls",
			needle: `order_mode must be "subsequence" or "exact"`,
		},
		{
			name:   "must call true with zero count",
			config: `{"tool_name":"submit","must_call":true,"count":0}`,
			target: "tool_calls",
			needle: "must_call cannot be true when count is 0",
		},
		{
			name:   "must call false with positive count",
			config: `{"tool_name":"submit","must_call":false,"count":1}`,
			target: "tool_calls",
			needle: "must_call cannot be false when count is greater than 0",
		},
		{
			name:   "must call true with zero max count",
			config: `{"tool_name":"submit","must_call":true,"max_count":0}`,
			target: "tool_calls",
			needle: "must_call cannot be true when max_count is 0",
		},
		{
			name:   "must call false with positive min count",
			config: `{"tool_name":"submit","must_call":false,"min_count":1}`,
			target: "tool_calls",
			needle: "must_call cannot be false when min_count is greater than 0",
		},
		{
			name:   "expected_from rejected",
			config: `{"tool_name":"submit"}`,
			target: "tool_calls",
			needle: "expected_from must be omitted for tool_call_assertion validators",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := json.RawMessage(tc.config)
			if tc.config == "" {
				config = nil
			}
			spec := toolCallAssertionSpec([]ValidatorDeclaration{{
				Key:    "tool_assert",
				Type:   ValidatorTypeToolCallAssertion,
				Target: tc.target,
				Config: config,
			}})
			if tc.name == "expected_from rejected" {
				spec.Validators[0].ExpectedFrom = "case.expectations.expected_output"
			}
			err := ValidateEvaluationSpec(spec)
			if err == nil {
				t.Fatal("ValidateEvaluationSpec error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tc.needle)
			}
		})
	}
}

func TestLoadEvaluationSpecToolCallAssertion(t *testing.T) {
	spec, err := DecodeDefinition(json.RawMessage(`{
		"name": "tool-assertions",
		"version_number": 1,
		"judge_mode": "deterministic",
		"validators": [{
			"key": "submitted",
			"type": "tool_call_assertion",
			"target": "tool_calls",
			"config": {
				"tool_name": "submit",
				"must_call": true,
				"arguments_contain": {"answer": "42"}
			}
		}],
		"scorecard": {"dimensions": ["correctness"]}
	}`))
	if err != nil {
		t.Fatalf("DecodeDefinition returned error: %v", err)
	}
	if spec.Validators[0].Type != ValidatorTypeToolCallAssertion {
		t.Fatalf("validator type = %q, want %q", spec.Validators[0].Type, ValidatorTypeToolCallAssertion)
	}
	if spec.Validators[0].ExpectedFrom != "" {
		t.Fatalf("expected_from = %q, want empty", spec.Validators[0].ExpectedFrom)
	}
}

func evaluateToolCallAssertionSpec(t *testing.T, validators []ValidatorDeclaration, events []Event) RunAgentEvaluation {
	t.Helper()
	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Events:           events,
	}, toolCallAssertionSpec(validators))
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}
	return evaluation
}

func toolCallAssertionSpec(validators []ValidatorDeclaration) EvaluationSpec {
	return EvaluationSpec{
		Name:          "tool-call-assertions",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators:    validators,
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: ScorecardDimensionCorrectness}},
		},
	}
}

func toolCallEvent(sequence int64, eventType, toolName, arguments string) Event {
	return Event{
		Type:           eventType,
		SequenceNumber: sequence,
		OccurredAt:     time.Date(2026, 5, 10, 12, 0, int(sequence), 0, time.UTC),
		Payload:        json.RawMessage(`{"tool_name":` + quoteJSON(toolName) + `,"arguments":` + arguments + `,"result":{"is_error":false,"content":"ok"}}`),
	}
}

func quoteJSON(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func assertRawOutputField(t *testing.T, raw json.RawMessage, key string, want any) {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode raw output: %v", err)
	}
	if got := decoded[key]; got != want {
		t.Fatalf("raw_output[%q] = %#v, want %#v; raw=%s", key, got, want, raw)
	}
}
