package voicefixtures

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/google/uuid"
)

const (
	SupportBillingScenarioKey = "support_billing_duplicate_charge"
	SupportBillingSeed        = int64(42)

	supportBillingBaseTime = "2026-05-13T10:00:00Z"
	supportBillingRunID    = "33333333-3333-3333-3333-333333333333"
	supportBillingAgentID  = "44444444-4444-4444-4444-444444444444"
)

//go:embed testdata/support_billing/*
var supportBillingFS embed.FS

type SupportBillingFixture struct {
	ChallengePackYAML       []byte
	ScriptedUserTurnsJSON   []byte
	ExpectedToolCallJSON    []byte
	ExpectedToolResultJSON  []byte
	ExpectedAgentTextOutput []byte
	ExpectedStructuredJSON  []byte
	ExpectedTraceJSON       []byte
	ExpectedScorecardJSON   []byte
}

type ScriptedUserTurn struct {
	TurnID             string `json:"turn_id"`
	Speaker            string `json:"speaker"`
	Text               string `json:"text"`
	Language           string `json:"language"`
	AudioArtifactRef   string `json:"audio_artifact_ref"`
	AudioFormat        string `json:"audio_format"`
	AudioChannel       string `json:"audio_channel"`
	DurationMS         int64  `json:"duration_ms"`
	OccurredAtOffsetMS int64  `json:"occurred_at_offset_ms"`
}

type ToolCallFixture struct {
	CallID    string          `json:"call_id"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResultFixture struct {
	CallID   string          `json:"call_id"`
	ToolName string          `json:"tool_name"`
	Result   json.RawMessage `json:"result"`
}

type StructuredOutputFixture struct {
	SchemaRef string          `json:"schema_ref"`
	Output    json.RawMessage `json:"output"`
}

type AgentResponse struct {
	TextOutput       string
	Language         string
	AudioArtifactRef string
	AudioFormat      string
	AudioChannel     string
	AudioDurationMS  int64
	StructuredOutput StructuredOutputFixture
	ToolCall         ToolCallFixture
	ToolResult       ToolResultFixture
}

type ScenarioRun struct {
	Trace                 multimodaltrace.Trace
	TraceJSON             []byte
	ScorecardJSON         []byte
	ToolCallJSON          []byte
	ToolCallArgumentsJSON []byte
	ToolResultJSON        []byte
	AgentTextOutput       []byte
	StructuredOutputJSON  []byte
	EventTimestamps       []time.Time
}

type FakeClock struct {
	base time.Time
}

func NewFakeClock(base time.Time) *FakeClock {
	return &FakeClock{base: base}
}

func (c *FakeClock) AtOffset(offset time.Duration) time.Time {
	return c.base.Add(offset).UTC()
}

type FakeUserSimulator struct {
	turns []ScriptedUserTurn
}

func NewFakeUserSimulator(turns []ScriptedUserTurn) *FakeUserSimulator {
	return &FakeUserSimulator{turns: append([]ScriptedUserTurn(nil), turns...)}
}

func (s *FakeUserSimulator) Turns() []ScriptedUserTurn {
	return append([]ScriptedUserTurn(nil), s.turns...)
}

type FakeToolEndpoint struct {
	expectedCall ToolCallFixture
	result       ToolResultFixture
	calls        []ToolCallFixture
}

func NewFakeToolEndpoint(expectedCall ToolCallFixture, result ToolResultFixture) *FakeToolEndpoint {
	return &FakeToolEndpoint{expectedCall: expectedCall, result: result}
}

func (e *FakeToolEndpoint) Call(call ToolCallFixture) (ToolResultFixture, error) {
	if call.CallID != e.expectedCall.CallID || call.ToolName != e.expectedCall.ToolName || !bytes.Equal(call.Arguments, e.expectedCall.Arguments) {
		return ToolResultFixture{}, fmt.Errorf("unexpected tool call: got %s/%s %s", call.CallID, call.ToolName, string(call.Arguments))
	}
	e.calls = append(e.calls, call)
	return e.result, nil
}

func (e *FakeToolEndpoint) Calls() []ToolCallFixture {
	return append([]ToolCallFixture(nil), e.calls...)
}

type FakeVoiceAgentDeployment struct {
	tool             *FakeToolEndpoint
	expectedCall     ToolCallFixture
	structuredOutput StructuredOutputFixture
	textOutput       string
}

func NewFakeVoiceAgentDeployment(tool *FakeToolEndpoint, expectedCall ToolCallFixture, structuredOutput StructuredOutputFixture, textOutput string) *FakeVoiceAgentDeployment {
	return &FakeVoiceAgentDeployment{
		tool:             tool,
		expectedCall:     expectedCall,
		structuredOutput: structuredOutput,
		textOutput:       textOutput,
	}
}

func (d *FakeVoiceAgentDeployment) Respond(turn ScriptedUserTurn) (AgentResponse, error) {
	if strings.TrimSpace(turn.Text) == "" {
		return AgentResponse{}, fmt.Errorf("scripted user turn %q has empty text", turn.TurnID)
	}
	result, err := d.tool.Call(d.expectedCall)
	if err != nil {
		return AgentResponse{}, err
	}
	return AgentResponse{
		TextOutput:       d.textOutput,
		Language:         turn.Language,
		AudioArtifactRef: "fixtures/support_billing/audio/agent-turn-001.wav",
		AudioFormat:      "wav",
		AudioChannel:     "agent",
		AudioDurationMS:  2400,
		StructuredOutput: d.structuredOutput,
		ToolCall:         d.expectedCall,
		ToolResult:       result,
	}, nil
}

type FakeMediaTransport struct {
	frames []MediaFrame
}

type MediaFrame struct {
	Actor       multimodaltrace.Actor
	ArtifactRef string
	Format      string
	Channel     string
	DurationMS  int64
	OccurredAt  time.Time
}

func (t *FakeMediaTransport) ReceiveUserAudio(turn ScriptedUserTurn, occurredAt time.Time) MediaFrame {
	frame := MediaFrame{
		Actor:       multimodaltrace.ActorUser,
		ArtifactRef: turn.AudioArtifactRef,
		Format:      turn.AudioFormat,
		Channel:     turn.AudioChannel,
		DurationMS:  turn.DurationMS,
		OccurredAt:  occurredAt,
	}
	t.frames = append(t.frames, frame)
	return frame
}

func (t *FakeMediaTransport) SendAgentAudio(response AgentResponse, occurredAt time.Time) MediaFrame {
	frame := MediaFrame{
		Actor:       multimodaltrace.ActorAgent,
		ArtifactRef: response.AudioArtifactRef,
		Format:      response.AudioFormat,
		Channel:     response.AudioChannel,
		DurationMS:  response.AudioDurationMS,
		OccurredAt:  occurredAt,
	}
	t.frames = append(t.frames, frame)
	return frame
}

func (t *FakeMediaTransport) Frames() []MediaFrame {
	return append([]MediaFrame(nil), t.frames...)
}

func LoadSupportBillingFixture() (SupportBillingFixture, error) {
	read := func(name string) ([]byte, error) {
		data, err := supportBillingFS.ReadFile("testdata/support_billing/" + name)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	var fixture SupportBillingFixture
	var err error
	if fixture.ChallengePackYAML, err = read("challenge_pack.yaml"); err != nil {
		return SupportBillingFixture{}, err
	}
	if fixture.ScriptedUserTurnsJSON, err = read("scripted_user_turns.json"); err != nil {
		return SupportBillingFixture{}, err
	}
	if fixture.ExpectedToolCallJSON, err = read("expected_tool_call.json"); err != nil {
		return SupportBillingFixture{}, err
	}
	if fixture.ExpectedToolResultJSON, err = read("expected_tool_result.json"); err != nil {
		return SupportBillingFixture{}, err
	}
	if fixture.ExpectedAgentTextOutput, err = read("expected_agent_text_output.txt"); err != nil {
		return SupportBillingFixture{}, err
	}
	if fixture.ExpectedStructuredJSON, err = read("expected_structured_output.json"); err != nil {
		return SupportBillingFixture{}, err
	}
	if fixture.ExpectedTraceJSON, err = read("expected_trace.json"); err != nil {
		return SupportBillingFixture{}, err
	}
	if fixture.ExpectedScorecardJSON, err = read("expected_scorecard.json"); err != nil {
		return SupportBillingFixture{}, err
	}
	return fixture, nil
}

func RunSupportBillingScenario(seed int64) (ScenarioRun, error) {
	fixture, err := LoadSupportBillingFixture()
	if err != nil {
		return ScenarioRun{}, err
	}
	if seed != SupportBillingSeed {
		return ScenarioRun{}, fmt.Errorf("unsupported support billing fixture seed %d", seed)
	}

	var turns []ScriptedUserTurn
	if err := json.Unmarshal(fixture.ScriptedUserTurnsJSON, &turns); err != nil {
		return ScenarioRun{}, fmt.Errorf("decode scripted user turns: %w", err)
	}
	if len(turns) != 1 {
		return ScenarioRun{}, fmt.Errorf("support billing fixture expects one scripted turn, got %d", len(turns))
	}

	var expectedCall ToolCallFixture
	if err := decodeCanonical(fixture.ExpectedToolCallJSON, &expectedCall); err != nil {
		return ScenarioRun{}, fmt.Errorf("decode expected tool call: %w", err)
	}
	var expectedResult ToolResultFixture
	if err := decodeCanonical(fixture.ExpectedToolResultJSON, &expectedResult); err != nil {
		return ScenarioRun{}, fmt.Errorf("decode expected tool result: %w", err)
	}
	var structured StructuredOutputFixture
	if err := decodeCanonical(fixture.ExpectedStructuredJSON, &structured); err != nil {
		return ScenarioRun{}, fmt.Errorf("decode expected structured output: %w", err)
	}

	baseTime, err := time.Parse(time.RFC3339, supportBillingBaseTime)
	if err != nil {
		return ScenarioRun{}, err
	}
	runID := uuid.MustParse(supportBillingRunID)
	runAgentID := uuid.MustParse(supportBillingAgentID)
	clock := NewFakeClock(baseTime)
	simulator := NewFakeUserSimulator(turns)
	tool := NewFakeToolEndpoint(expectedCall, expectedResult)
	agent := NewFakeVoiceAgentDeployment(tool, expectedCall, structured, strings.TrimSpace(string(fixture.ExpectedAgentTextOutput)))
	media := &FakeMediaTransport{}

	turn := simulator.Turns()[0]
	response, err := agent.Respond(turn)
	if err != nil {
		return ScenarioRun{}, err
	}

	userFrame := media.ReceiveUserAudio(turn, clock.AtOffset(time.Duration(turn.OccurredAtOffsetMS)*time.Millisecond))
	agentFrame := media.SendAgentAudio(response, clock.AtOffset(6*time.Second))
	confidence := 1.0
	trace := multimodaltrace.Trace{
		TraceID:       "trace-support-billing-seed-42",
		SchemaVersion: multimodaltrace.SchemaVersionV1,
		RunID:         runID,
		RunAgentID:    runAgentID,
		Segments: []multimodaltrace.Segment{
			{
				SegmentID:      "seg-001",
				SequenceNumber: 1,
				Kind:           multimodaltrace.SegmentKindAudioInput,
				Actor:          userFrame.Actor,
				OccurredAt:     userFrame.OccurredAt,
				Audio: &multimodaltrace.AudioPayload{
					ArtifactRef: userFrame.ArtifactRef,
					Format:      userFrame.Format,
					Channel:     userFrame.Channel,
					DurationMS:  userFrame.DurationMS,
				},
			},
			{
				SegmentID:      "seg-002",
				SequenceNumber: 2,
				Kind:           multimodaltrace.SegmentKindTranscriptFinal,
				Actor:          multimodaltrace.ActorSystem,
				OccurredAt:     clock.AtOffset(2 * time.Second),
				Transcript: &multimodaltrace.TranscriptPayload{
					Text:            turn.Text,
					Language:        turn.Language,
					Confidence:      &confidence,
					SourceSegmentID: "seg-001",
				},
			},
			{
				SegmentID:      "seg-003",
				SequenceNumber: 3,
				Kind:           multimodaltrace.SegmentKindToolCall,
				Actor:          multimodaltrace.ActorAgent,
				OccurredAt:     clock.AtOffset(3 * time.Second),
				ToolCall: &multimodaltrace.ToolCallPayload{
					CallID:    response.ToolCall.CallID,
					ToolName:  response.ToolCall.ToolName,
					Arguments: response.ToolCall.Arguments,
				},
			},
			{
				SegmentID:      "seg-004",
				SequenceNumber: 4,
				Kind:           multimodaltrace.SegmentKindToolResult,
				Actor:          multimodaltrace.ActorTool,
				OccurredAt:     clock.AtOffset(4 * time.Second),
				ToolResult: &multimodaltrace.ToolResultPayload{
					CallID:   response.ToolResult.CallID,
					ToolName: response.ToolResult.ToolName,
					Result:   response.ToolResult.Result,
				},
			},
			{
				SegmentID:      "seg-005",
				SequenceNumber: 5,
				Kind:           multimodaltrace.SegmentKindTextOutput,
				Actor:          multimodaltrace.ActorAgent,
				OccurredAt:     clock.AtOffset(5 * time.Second),
				Text: &multimodaltrace.TextPayload{
					Text:     response.TextOutput,
					Language: response.Language,
				},
			},
			{
				SegmentID:      "seg-006",
				SequenceNumber: 6,
				Kind:           multimodaltrace.SegmentKindAudioOutput,
				Actor:          agentFrame.Actor,
				OccurredAt:     agentFrame.OccurredAt,
				Audio: &multimodaltrace.AudioPayload{
					ArtifactRef: agentFrame.ArtifactRef,
					Format:      agentFrame.Format,
					Channel:     agentFrame.Channel,
					DurationMS:  agentFrame.DurationMS,
				},
			},
			{
				SegmentID:      "seg-007",
				SequenceNumber: 7,
				Kind:           multimodaltrace.SegmentKindStructuredOutput,
				Actor:          multimodaltrace.ActorAgent,
				OccurredAt:     clock.AtOffset(7 * time.Second),
				StructuredOutput: &multimodaltrace.StructuredOutputPayload{
					SchemaRef: response.StructuredOutput.SchemaRef,
					Output:    response.StructuredOutput.Output,
				},
			},
			{
				SegmentID:      "seg-008",
				SequenceNumber: 8,
				Kind:           multimodaltrace.SegmentKindTimingMarker,
				Actor:          multimodaltrace.ActorEvaluator,
				OccurredAt:     clock.AtOffset(7 * time.Second),
				TimingMarker: &multimodaltrace.TimingMarkerPayload{
					Key:     "end_of_speech_to_first_agent_text",
					ValueMS: 3200,
				},
			},
		},
	}
	if err := trace.Validate(); err != nil {
		return ScenarioRun{}, fmt.Errorf("validate support billing trace: %w", err)
	}

	traceJSON, err := marshalGolden(trace)
	if err != nil {
		return ScenarioRun{}, err
	}
	scorecardJSON, err := marshalGolden(supportBillingScorecard(seed))
	if err != nil {
		return ScenarioRun{}, err
	}
	toolCallJSON, err := marshalGolden(response.ToolCall)
	if err != nil {
		return ScenarioRun{}, err
	}
	toolResultJSON, err := marshalGolden(response.ToolResult)
	if err != nil {
		return ScenarioRun{}, err
	}
	structuredJSON, err := marshalGolden(response.StructuredOutput)
	if err != nil {
		return ScenarioRun{}, err
	}

	timestamps := make([]time.Time, 0, len(trace.Segments))
	for _, segment := range trace.Segments {
		timestamps = append(timestamps, segment.OccurredAt)
	}

	return ScenarioRun{
		Trace:                 trace,
		TraceJSON:             traceJSON,
		ScorecardJSON:         scorecardJSON,
		ToolCallJSON:          toolCallJSON,
		ToolCallArgumentsJSON: append([]byte(nil), response.ToolCall.Arguments...),
		ToolResultJSON:        toolResultJSON,
		AgentTextOutput:       append([]byte(response.TextOutput), '\n'),
		StructuredOutputJSON:  structuredJSON,
		EventTimestamps:       timestamps,
	}, nil
}

type supportBillingScorecardFixture struct {
	SchemaVersion int                         `json:"schema_version"`
	ScenarioKey   string                      `json:"scenario_key"`
	Seed          int64                       `json:"seed"`
	Passed        bool                        `json:"passed"`
	OverallScore  float64                     `json:"overall_score"`
	Checks        []supportBillingScoreCheck  `json:"checks"`
	Metrics       []supportBillingScoreMetric `json:"metrics"`
}

type supportBillingScoreCheck struct {
	Key      string `json:"key"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type supportBillingScoreMetric struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
	Unit  string `json:"unit"`
}

func supportBillingScorecard(seed int64) supportBillingScorecardFixture {
	return supportBillingScorecardFixture{
		SchemaVersion: 1,
		ScenarioKey:   SupportBillingScenarioKey,
		Seed:          seed,
		Passed:        true,
		OverallScore:  1,
		Checks: []supportBillingScoreCheck{
			{Key: "refund_created", Passed: true, Expected: "created", Actual: "created"},
			{Key: "tool_name", Passed: true, Expected: "refund_api", Actual: "refund_api"},
			{Key: "structured_resolution", Passed: true, Expected: "refund_created", Actual: "refund_created"},
		},
		Metrics: []supportBillingScoreMetric{
			{Key: "turn_count", Value: 1, Unit: "turn"},
			{Key: "end_of_speech_to_first_agent_text_ms", Value: 3200, Unit: "ms"},
		},
	}
}

func decodeCanonical(data []byte, out any) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	canonical, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(canonical, out)
}

func marshalGolden(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
