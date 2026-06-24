package voicefixtures

import (
	"bytes"
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
)

var updateGoldens = flag.Bool("update", false, "update generated golden fixture files")

func TestSupportBillingScenarioGoldensAreDeterministic(t *testing.T) {
	fixture, err := LoadSupportBillingFixture()
	if err != nil {
		t.Fatalf("LoadSupportBillingFixture returned error: %v", err)
	}
	if _, err := challengepack.ParseYAML(fixture.ChallengePackYAML); err != nil {
		t.Fatalf("challenge pack fixture failed validation: %v", err)
	}

	first, err := RunSupportBillingScenario()
	if err != nil {
		t.Fatalf("first RunSupportBillingScenario returned error: %v", err)
	}
	second, err := RunSupportBillingScenario()
	if err != nil {
		t.Fatalf("second RunSupportBillingScenario returned error: %v", err)
	}

	assertBytesEqual(t, "trace output repeated run", first.TraceJSON, second.TraceJSON)
	assertBytesEqual(t, "scorecard output repeated run", first.ScorecardJSON, second.ScorecardJSON)
	if !reflect.DeepEqual(first.EventTimestamps, second.EventTimestamps) {
		t.Fatalf("event timestamps mismatch\nwant: %#v\n got: %#v", first.EventTimestamps, second.EventTimestamps)
	}
	assertBytesEqual(t, "tool-call arguments repeated run", first.ToolCallArgumentsJSON, second.ToolCallArgumentsJSON)
	if *updateGoldens {
		updateGeneratedGoldens(t, first)
		fixture, err = LoadSupportBillingFixture()
		if err != nil {
			t.Fatalf("reload updated fixture: %v", err)
		}
	}
	assertBytesEqual(t, "tool call golden", fixture.ExpectedToolCallJSON, first.ToolCallJSON)
	assertBytesEqual(t, "tool result golden", fixture.ExpectedToolResultJSON, first.ToolResultJSON)
	assertBytesEqual(t, "agent text output golden", fixture.ExpectedAgentTextOutput, first.AgentTextOutput)
	assertBytesEqual(t, "structured output golden", fixture.ExpectedStructuredJSON, first.StructuredOutputJSON)
	assertBytesEqual(t, "trace golden", fixture.ExpectedTraceJSON, first.TraceJSON)
	assertBytesEqual(t, "scorecard golden", fixture.ExpectedScorecardJSON, first.ScorecardJSON)
}

func assertBytesEqual(t *testing.T, label string, want []byte, got []byte) {
	t.Helper()
	if !bytes.Equal(want, got) {
		t.Fatalf("%s mismatch\nwant:\n%s\n got:\n%s", label, string(want), string(got))
	}
}

func updateGeneratedGoldens(t *testing.T, run ScenarioRun) {
	t.Helper()
	writeGolden(t, "testdata/support_billing/expected_tool_call.json", run.ToolCallJSON)
	writeGolden(t, "testdata/support_billing/expected_tool_result.json", run.ToolResultJSON)
	writeGolden(t, "testdata/support_billing/expected_agent_text_output.txt", run.AgentTextOutput)
	writeGolden(t, "testdata/support_billing/expected_structured_output.json", run.StructuredOutputJSON)
	writeGolden(t, "testdata/support_billing/expected_trace.json", run.TraceJSON)
	writeGolden(t, "testdata/support_billing/expected_scorecard.json", run.ScorecardJSON)
}

func writeGolden(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("update golden %s: %v", path, err)
	}
}
