package voicee2e

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/evalpack"
	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/releasegate"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/voiceartifacts"
	"github.com/agentclash/agentclash/backend/internal/voicedeployment"
	"github.com/agentclash/agentclash/backend/internal/voiceeval"
	"github.com/agentclash/agentclash/backend/internal/voicefixtures"
	"github.com/agentclash/agentclash/backend/internal/voicereplay"
	"github.com/agentclash/agentclash/backend/internal/voicescorecard"
	"github.com/agentclash/agentclash/backend/internal/voicesim"
	"github.com/agentclash/agentclash/backend/internal/voicetextsim"
)

// Smoke command: go test ./internal/voicee2e -run TestSupportAgentVoiceEvalLoopSmoke -count=1
func TestSupportAgentVoiceEvalLoopSmoke(t *testing.T) {
	fixture := loadSupportFixture(t)
	assertSupportFixtureGoldens(t, fixture)
	bundle := parseSupportPack(t, fixture)
	manifest := loadSupportManifest(t)
	if err := manifest.VerifyLocalChecksums("../voiceartifacts/testdata/support_billing"); err != nil {
		t.Fatalf("manifest VerifyLocalChecksums returned error: %v", err)
	}

	baseline := runSupportTextSim(t, bundle, fixture, voicedeployment.OutcomePass)
	candidate := runSupportTextSim(t, bundle, fixture, voicedeployment.OutcomePass)
	if !bytes.Equal(baseline.TraceJSON, candidate.TraceJSON) {
		t.Fatalf("happy path trace JSON is not deterministic")
	}
	if !bytes.Equal(baseline.EventsJSON, candidate.EventsJSON) {
		t.Fatalf("happy path events JSON is not deterministic")
	}
	assertAgentTextOutput(t, baseline.Trace.Segments, strings.TrimSpace(string(fixture.ExpectedAgentTextOutput)))
	assertCanonicalVoiceEvents(t, baseline.Events)

	projection, err := voicereplay.Build(baseline.Events, manifest)
	if err != nil {
		t.Fatalf("voicereplay.Build returned error: %v", err)
	}
	projectionJSON, err := voicereplay.StableJSON(projection)
	if err != nil {
		t.Fatalf("voicereplay.StableJSON returned error: %v", err)
	}
	if len(projection.Turns) == 0 || !bytes.Contains(projectionJSON, []byte(`"turn_id": "turn-001"`)) {
		t.Fatalf("projection missing support turn:\n%s", projectionJSON)
	}
	if len(projection.DegradedEvidence) != 0 {
		t.Fatalf("projection degraded evidence = %v, want none", projection.DegradedEvidence)
	}

	expectations := scorecardExpectations(t, fixture)
	baselineScorecard := generateScorecard(t, baseline, expectations)
	candidateScorecard := generateScorecard(t, candidate, expectations)
	if !baselineScorecard.Passed || baselineScorecard.HardGateFailed {
		t.Fatalf("baseline scorecard = passed:%v hard_gate:%v, want passed/no hard gate", baselineScorecard.Passed, baselineScorecard.HardGateFailed)
	}
	if !candidateScorecard.Passed || candidateScorecard.HardGateFailed {
		t.Fatalf("candidate scorecard = passed:%v hard_gate:%v, want passed/no hard gate", candidateScorecard.Passed, candidateScorecard.HardGateFailed)
	}

	passEvaluation := evaluateVoiceGate(t, baselineScorecard, candidateScorecard)
	if passEvaluation.Verdict != releasegate.VerdictPass {
		t.Fatalf("happy compare verdict = %q/%s, want pass", passEvaluation.Verdict, passEvaluation.ReasonCode)
	}

	badCandidate := runSupportTextSim(t, bundle, fixture, voicedeployment.OutcomeFail)
	badScorecard := generateScorecard(t, badCandidate, expectations)
	if badScorecard.Passed || !badScorecard.HardGateFailed {
		t.Fatalf("bad candidate scorecard = passed:%v hard_gate:%v, want failed hard gate", badScorecard.Passed, badScorecard.HardGateFailed)
	}
	failEvaluation := evaluateVoiceGate(t, baselineScorecard, badScorecard)
	if failEvaluation.Verdict != releasegate.VerdictFail || failEvaluation.ReasonCode != "scorecard_not_passed" {
		t.Fatalf("bad compare verdict = %q/%s, want fail/scorecard_not_passed", failEvaluation.Verdict, failEvaluation.ReasonCode)
	}
}

func loadSupportFixture(t *testing.T) voicefixtures.SupportBillingFixture {
	t.Helper()
	fixture, err := voicefixtures.LoadSupportBillingFixture()
	if err != nil {
		t.Fatalf("LoadSupportBillingFixture returned error: %v", err)
	}
	return fixture
}

func assertSupportFixtureGoldens(t *testing.T, fixture voicefixtures.SupportBillingFixture) {
	t.Helper()
	run, err := voicefixtures.RunSupportBillingScenario()
	if err != nil {
		t.Fatalf("RunSupportBillingScenario returned error: %v", err)
	}
	assertBytesEqual(t, "fixture tool result golden", fixture.ExpectedToolResultJSON, run.ToolResultJSON)
	assertBytesEqual(t, "fixture agent text output golden", fixture.ExpectedAgentTextOutput, run.AgentTextOutput)
	assertBytesEqual(t, "fixture trace golden", fixture.ExpectedTraceJSON, run.TraceJSON)
	assertBytesEqual(t, "fixture scorecard golden", fixture.ExpectedScorecardJSON, run.ScorecardJSON)

	var turns []voicefixtures.ScriptedUserTurn
	if err := json.Unmarshal(fixture.ScriptedUserTurnsJSON, &turns); err != nil {
		t.Fatalf("decode scripted user turns: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("scripted user turns = %d, want 1", len(turns))
	}
	if strings.TrimSpace(turns[0].Text) == "" || strings.TrimSpace(turns[0].AudioArtifactRef) == "" {
		t.Fatalf("scripted user turn missing text/audio artifact: %+v", turns[0])
	}
}

func parseSupportPack(t *testing.T, fixture voicefixtures.SupportBillingFixture) evalpack.Bundle {
	t.Helper()
	bundle, err := evalpack.ParseYAML(fixture.EvalPackYAML)
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if bundle.Modality != evalpack.ModalityVoice {
		t.Fatalf("bundle modality = %q, want voice", bundle.Modality)
	}
	if bundle.InterfaceSpec == nil || !contains(bundle.InterfaceSpec.Transports, "text_sim") {
		t.Fatalf("bundle interface spec = %+v, want text_sim transport", bundle.InterfaceSpec)
	}
	return bundle
}

func loadSupportManifest(t *testing.T) voiceartifacts.Manifest {
	t.Helper()
	manifest, err := voiceartifacts.Load("../voiceartifacts/testdata/support_billing/voice_artifact_manifest.json")
	if err != nil {
		t.Fatalf("voiceartifacts.Load returned error: %v", err)
	}
	return manifest
}

func runSupportTextSim(t *testing.T, bundle evalpack.Bundle, fixture voicefixtures.SupportBillingFixture, outcome voicedeployment.Outcome) voicetextsim.Result {
	t.Helper()
	script, err := voicesim.LoadScript("../voicesim/testdata/support_billing_script.json")
	if err != nil {
		t.Fatalf("LoadScript returned error: %v", err)
	}
	deployment, err := voicedeployment.NewFake(fakeDeploymentScript(t, script, fixture, outcome))
	if err != nil {
		t.Fatalf("NewFake returned error: %v", err)
	}
	result, err := voicetextsim.Run(context.Background(), voicetextsim.Input{
		Bundle:     bundle,
		Script:     script,
		Deployment: deployment,
	})
	if err != nil {
		t.Fatalf("voicetextsim.Run returned error: %v", err)
	}
	return result
}

func fakeDeploymentScript(t *testing.T, script voicesim.Script, fixture voicefixtures.SupportBillingFixture, outcome voicedeployment.Outcome) voicedeployment.Script {
	t.Helper()
	var toolCall struct {
		CallID    string          `json:"call_id"`
		ToolName  string          `json:"tool_name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(fixture.ExpectedToolCallJSON, &toolCall); err != nil {
		t.Fatalf("decode tool call: %v", err)
	}
	var structured struct {
		SchemaRef string          `json:"schema_ref"`
		Output    json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(fixture.ExpectedStructuredJSON, &structured); err != nil {
		t.Fatalf("decode structured output: %v", err)
	}

	text := strings.TrimSpace(string(fixture.ExpectedAgentTextOutput))
	if text == "" {
		t.Fatalf("expected agent text output fixture is empty")
	}
	structuredOutput := structured.Output
	if outcome == voicedeployment.OutcomeFail {
		text = "I cannot find a duplicate charge."
		structuredOutput = json.RawMessage(`{"resolution":"not_found"}`)
	}
	confidence := 1.0
	return voicedeployment.Script{
		ScenarioKey: script.ScenarioKey,
		Turns: []voicedeployment.Turn{
			{
				TurnID:            script.Steps[0].TurnID,
				ExpectedInputText: script.Steps[0].UserText,
				Outcome:           outcome,
				Response: voicedeployment.ResponseScript{
					ExpectedToolCall: &voicedeployment.ExpectedToolCall{
						CallID:    toolCall.CallID,
						ToolName:  toolCall.ToolName,
						Arguments: toolCall.Arguments,
						OffsetMS:  700,
					},
					TextResponse: &voicedeployment.TextResponse{
						Text:     text,
						Language: script.Steps[0].Language,
						OffsetMS: 1200,
					},
					SpokenAudioArtifact: &voicedeployment.SpokenAudioArtifact{
						ArtifactRef: "voice://artifacts/agent-turn-001.wav",
						Format:      "wav",
						Channel:     "agent",
						DurationMS:  2400,
						OffsetMS:    1600,
					},
					TranscriptResponse: &voicedeployment.TranscriptResponse{
						Text:       text,
						Language:   script.Steps[0].Language,
						Confidence: &confidence,
						OffsetMS:   1700,
					},
					StructuredOutput: &voicedeployment.StructuredOutput{
						SchemaRef: structured.SchemaRef,
						Output:    structuredOutput,
						OffsetMS:  1800,
					},
					TimingMarkers: []voicedeployment.TimingMarker{
						{
							Key:      voiceeval.KeyEndOfUserTextToFirstAgentTextLegacy,
							ValueMS:  1200,
							OffsetMS: 1800,
						},
					},
				},
			},
		},
	}
}

func scorecardExpectations(t *testing.T, fixture voicefixtures.SupportBillingFixture) voicescorecard.Expectations {
	t.Helper()
	var toolCall struct {
		ToolName  string          `json:"tool_name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(fixture.ExpectedToolCallJSON, &toolCall); err != nil {
		t.Fatalf("decode expected tool call: %v", err)
	}
	return voicescorecard.Expectations{
		TaskSuccessField: "resolution",
		TaskSuccessValue: "refund_created",
		ExpectedToolName: toolCall.ToolName,
		ExpectedToolArgs: toolCall.Arguments,
		ForbiddenPhrases: []string{"cannot find a duplicate charge"},
		MaxTurns:         1,
		LatencyTargetMS:  1200,
		LatencyMaxMS:     1500,
		CostUSD:          0,
	}
}

func generateScorecard(t *testing.T, result voicetextsim.Result, expectations voicescorecard.Expectations) voicescorecard.Scorecard {
	t.Helper()
	scorecard, err := voicescorecard.Generate(voiceeval.Input{
		Trace:  result.Trace,
		Events: result.Events,
	}, expectations)
	if err != nil {
		t.Fatalf("voicescorecard.Generate returned error: %v", err)
	}
	return scorecard
}

func evaluateVoiceGate(t *testing.T, baseline voicescorecard.Scorecard, candidate voicescorecard.Scorecard) releasegate.Evaluation {
	t.Helper()
	evaluation, err := releasegate.Evaluate(
		releasegate.BuildVoiceComparisonSummary(baseline, candidate, releasegate.DefaultVoiceComparisonConfig()),
		releasegate.VoicePolicy(releasegate.DefaultVoiceComparisonConfig()),
	)
	if err != nil {
		t.Fatalf("releasegate.Evaluate returned error: %v", err)
	}
	return evaluation
}

func assertCanonicalVoiceEvents(t *testing.T, events []runevents.Envelope) {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("events are required")
	}
	foundCompleted := false
	for idx, event := range events {
		if event.SequenceNumber != int64(idx+1) {
			t.Fatalf("events[%d].SequenceNumber = %d, want %d", idx, event.SequenceNumber, idx+1)
		}
		if err := event.ValidatePersisted(); err != nil {
			t.Fatalf("events[%d] ValidatePersisted returned error: %v", idx, err)
		}
		if event.EventType == runevents.EventTypeSystemRunCompleted {
			foundCompleted = true
		}
	}
	if !foundCompleted {
		t.Fatal("system.run.completed event not found")
	}
}

func assertAgentTextOutput(t *testing.T, segments []multimodaltrace.Segment, want string) {
	t.Helper()
	for _, segment := range segments {
		if segment.Kind != multimodaltrace.SegmentKindTextOutput {
			continue
		}
		if segment.Text == nil {
			t.Fatalf("text output segment %q has nil payload", segment.SegmentID)
		}
		if segment.Text.Text != want {
			t.Fatalf("agent text output = %q, want %q", segment.Text.Text, want)
		}
		return
	}
	t.Fatalf("text output segment not found")
}

func assertBytesEqual(t *testing.T, label string, want []byte, got []byte) {
	t.Helper()
	if !bytes.Equal(want, got) {
		t.Fatalf("%s mismatch\nwant:\n%s\n got:\n%s", label, string(want), string(got))
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
