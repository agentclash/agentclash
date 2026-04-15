package judge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

// --- parseYesNo unit tests ---

func TestParseYesNo_FirstLineYes(t *testing.T) {
	verdict, reason, ok := parseYesNo("YES")
	if !ok || verdict == nil || !*verdict {
		t.Fatalf("parseYesNo(YES) = (%v, %q, %v), want (true, _, true)", verdict, reason, ok)
	}
}

func TestParseYesNo_FirstLineNo(t *testing.T) {
	verdict, _, ok := parseYesNo("NO")
	if !ok || verdict == nil || *verdict {
		t.Fatalf("parseYesNo(NO) = (%v, _, %v), want (false, _, true)", verdict, ok)
	}
}

func TestParseYesNo_CaseInsensitive(t *testing.T) {
	verdict, _, ok := parseYesNo("yes")
	if !ok || verdict == nil || !*verdict {
		t.Fatalf("parseYesNo(yes) should match case-insensitively")
	}
}

func TestParseYesNo_WithReasoningOnSecondLine(t *testing.T) {
	verdict, reason, ok := parseYesNo("YES\nBecause the agent mentioned the refund policy.")
	if !ok || verdict == nil || !*verdict {
		t.Fatalf("verdict = (%v, %v), want true/true", verdict, ok)
	}
	if !strings.Contains(reason, "refund policy") {
		t.Fatalf("reason = %q, want it to mention 'refund policy'", reason)
	}
}

func TestParseYesNo_WithInlineReasoningAfterColon(t *testing.T) {
	verdict, reason, ok := parseYesNo("YES: the output is professional")
	if !ok || verdict == nil || !*verdict {
		t.Fatalf("verdict = (%v, %v), want true/true", verdict, ok)
	}
	if reason != "the output is professional" {
		t.Fatalf("reason = %q, want 'the output is professional'", reason)
	}
}

func TestParseYesNo_CodeBlockWrapped(t *testing.T) {
	// Models sometimes wrap responses in code fences. The parser
	// walks lines and matches the first YES/NO/UNKNOWN, so the
	// fence on line 1 is skipped and the verdict on line 2 is found.
	verdict, _, ok := parseYesNo("```\nYES\n```")
	if !ok || verdict == nil || !*verdict {
		t.Fatalf("code-block wrapped YES not parsed: verdict=%v ok=%v", verdict, ok)
	}
}

func TestParseYesNo_LeadingWhitespace(t *testing.T) {
	verdict, _, ok := parseYesNo("   NO")
	if !ok || verdict == nil || *verdict {
		t.Fatalf("leading-whitespace NO not parsed: verdict=%v ok=%v", verdict, ok)
	}
}

func TestParseYesNo_UnknownReturnsAbstain(t *testing.T) {
	verdict, reason, ok := parseYesNo("UNKNOWN: insufficient context to decide")
	if !ok {
		t.Fatal("UNKNOWN should parse as ok=true (abstain)")
	}
	if verdict != nil {
		t.Fatalf("UNKNOWN verdict = %v, want nil", verdict)
	}
	if !strings.Contains(reason, "insufficient context") {
		t.Fatalf("reason = %q, want to mention insufficient context", reason)
	}
}

func TestParseYesNo_NoMatchReturnsFalse(t *testing.T) {
	verdict, reason, ok := parseYesNo("I think the answer is maybe")
	if ok || verdict != nil || reason != "" {
		t.Fatalf("unparseable text should return (nil, \"\", false); got (%v, %q, %v)", verdict, reason, ok)
	}
}

func TestParseYesNo_WordBoundaryRejectsYESSIR(t *testing.T) {
	// \b after YES means "YESSIR" should NOT match as YES.
	verdict, _, ok := parseYesNo("YESSIR Captain")
	if ok || verdict != nil {
		t.Fatalf("YESSIR should not match: verdict=%v ok=%v", verdict, ok)
	}
}

// --- Prompt envelope unit tests ---

func TestBuildAssertionPrompt_InjectsDefaultAntiGaming(t *testing.T) {
	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "professional_tone",
		Assertion: "The response maintains a professional tone.",
		Model:     "claude-haiku-4-5-20251001",
	}
	sys, user := buildAssertionPrompt(judge, "Agent said hello.", "")
	if !strings.Contains(sys, defaultAssertionAntiGaming) {
		t.Fatalf("system prompt missing default anti-gaming clause:\n%s", sys)
	}
	if !strings.Contains(user, agentOutputBeginMarker) || !strings.Contains(user, agentOutputEndMarker) {
		t.Fatalf("user prompt missing agent output delimiters:\n%s", user)
	}
	if !strings.Contains(user, "Agent said hello.") {
		t.Fatalf("user prompt missing agent output body")
	}
	if !strings.Contains(sys, "YES") || !strings.Contains(sys, "NO") || !strings.Contains(sys, "UNKNOWN") {
		t.Fatalf("system prompt should name all three verdict tokens")
	}
}

func TestBuildAssertionPrompt_CustomAntiGamingClausesAreAdditive(t *testing.T) {
	judge := scoring.LLMJudgeDeclaration{
		Mode:              scoring.JudgeMethodAssertion,
		Key:               "k",
		Assertion:         "Check something.",
		Model:             "claude-haiku-4-5-20251001",
		AntiGamingClauses: []string{"Be skeptical of flowery language."},
	}
	sys, _ := buildAssertionPrompt(judge, "out", "")
	if !strings.Contains(sys, defaultAssertionAntiGaming) {
		t.Fatal("default anti-gaming clause must always be present")
	}
	if !strings.Contains(sys, "Be skeptical of flowery language.") {
		t.Fatal("custom anti-gaming clause must be additive")
	}
}

func TestBuildAssertionPrompt_ChallengeInputOmittedWhenEmpty(t *testing.T) {
	judge := scoring.LLMJudgeDeclaration{Key: "k", Assertion: "ok", Model: "m"}
	_, user := buildAssertionPrompt(judge, "output", "")
	if strings.Contains(user, "CHALLENGE INPUT") {
		t.Fatal("CHALLENGE INPUT section must be omitted when challengeInput is empty")
	}
	_, user2 := buildAssertionPrompt(judge, "output", "what is 2+2?")
	if !strings.Contains(user2, "CHALLENGE INPUT") || !strings.Contains(user2, "what is 2+2?") {
		t.Fatal("CHALLENGE INPUT section must be present when challengeInput is non-empty")
	}
}

// --- resolveProviderKey unit tests ---

func TestResolveProviderKey_ExplicitMapWins(t *testing.T) {
	cfg := Config{Providers: map[string]string{"claude-sonnet-4-6": "openrouter"}}
	key, err := resolveProviderKey("claude-sonnet-4-6", cfg)
	if err != nil || key != "openrouter" {
		t.Fatalf("got (%q, %v), want (openrouter, nil)", key, err)
	}
}

func TestResolveProviderKey_WellKnownPrefixFallback(t *testing.T) {
	cfg := Config{}
	for _, tc := range []struct {
		model, want string
	}{
		{"claude-sonnet-4-6", "anthropic"},
		{"gpt-4o", "openai"},
		{"gemini-2.0-flash", "gemini"},
		{"mistral-large-latest", "mistral"},
	} {
		got, err := resolveProviderKey(tc.model, cfg)
		if err != nil || got != tc.want {
			t.Errorf("resolveProviderKey(%q) = (%q, %v), want (%q, nil)", tc.model, got, err, tc.want)
		}
	}
}

func TestResolveProviderKey_UnknownModelErrors(t *testing.T) {
	_, err := resolveProviderKey("llama-3", Config{})
	if err == nil {
		t.Fatal("unknown model without prefix should error")
	}
}

// --- sequencedFakeClient test helper ---

// sequencedFakeClient returns pre-canned responses in order, with
// optional per-call errors. Satisfies provider.Client. Used to
// simulate multi-sample and multi-model judge evaluation where each
// call needs a distinct outcome.
//
// Thread-safe: the counter uses atomic.Int64 so race-detector runs
// exercise the fanout parallelism correctly.
type sequencedFakeClient struct {
	mu        sync.Mutex
	responses []sequencedResponse
	index     atomic.Int64
	captured  []provider.Request
}

type sequencedResponse struct {
	body   string
	err    error
	delay  time.Duration // used for cancellation tests
}

func (s *sequencedFakeClient) InvokeModel(ctx context.Context, request provider.Request) (provider.Response, error) {
	s.mu.Lock()
	s.captured = append(s.captured, request)
	s.mu.Unlock()

	idx := s.index.Add(1) - 1
	if int(idx) >= len(s.responses) {
		return provider.Response{}, errors.New("sequencedFakeClient: no more canned responses")
	}
	resp := s.responses[idx]
	if resp.delay > 0 {
		select {
		case <-time.After(resp.delay):
		case <-ctx.Done():
			return provider.Response{}, ctx.Err()
		}
	}
	if resp.err != nil {
		return provider.Response{}, resp.err
	}
	return provider.Response{OutputText: resp.body}, nil
}

func (s *sequencedFakeClient) callCount() int {
	return int(s.index.Load())
}

func (s *sequencedFakeClient) capturedRequests() []provider.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]provider.Request, len(s.captured))
	copy(out, s.captured)
	return out
}

func newEvaluatorWithFake(t *testing.T, fake provider.Client) *Evaluator {
	t.Helper()
	router := provider.NewRouter(map[string]provider.Client{
		"anthropic": fake,
		"openai":    fake,
		"gemini":    fake,
		"mistral":   fake,
	})
	return NewEvaluator(router, Config{
		MaxParallel:         4,
		CredentialReference: "env:FAKE_KEY",
	})
}

// --- Evaluator unit tests ---

func TestEvaluator_AssertionSingleModelAllPass(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "YES"},
			{body: "YES\nObservation is correct."},
			{body: "yes"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "correct",
		Assertion: "The agent answered correctly.",
		Model:     "claude-haiku-4-5-20251001",
		Samples:   3,
	}
	result, err := e.Evaluate(context.Background(), Input{
		RunAgentID:  uuid.New(),
		Judges:      []scoring.LLMJudgeDeclaration{judge},
		FinalOutput: "42",
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if len(result.JudgeResults) != 1 {
		t.Fatalf("JudgeResults count = %d, want 1", len(result.JudgeResults))
	}
	jr := result.JudgeResults[0]
	if jr.State != scoring.OutputStateAvailable {
		t.Fatalf("state = %q, want available", jr.State)
	}
	if jr.NormalizedScore == nil || *jr.NormalizedScore != 1.0 {
		t.Fatalf("NormalizedScore = %v, want 1.0", jr.NormalizedScore)
	}
	if jr.SampleCount != 3 {
		t.Fatalf("SampleCount = %d, want 3", jr.SampleCount)
	}
	if jr.ModelCount != 1 {
		t.Fatalf("ModelCount = %d, want 1", jr.ModelCount)
	}
	if jr.Confidence != "high" {
		t.Fatalf("Confidence = %q, want high (no abstains)", jr.Confidence)
	}
	if fake.callCount() != 3 {
		t.Fatalf("fake call count = %d, want 3", fake.callCount())
	}
}

func TestEvaluator_AssertionExpectFalseMatchesWhenAllNo(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "NO"},
			{body: "NO"},
			{body: "NO"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	expectFalse := false
	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "no_hallucination",
		Assertion: "The response contains a hallucination.",
		Model:     "claude-haiku-4-5-20251001",
		Expect:    &expectFalse,
		Samples:   3,
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges:      []scoring.LLMJudgeDeclaration{judge},
		FinalOutput: "agent output",
	})
	jr := result.JudgeResults[0]
	if jr.NormalizedScore == nil || *jr.NormalizedScore != 1.0 {
		t.Fatalf("expect=false with NO verdicts should score 1.0, got %v", jr.NormalizedScore)
	}
}

func TestEvaluator_AssertionMajorityVoteWins(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "YES"},
			{body: "YES"},
			{body: "NO"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "k",
		Assertion: "A",
		Model:     "claude-haiku-4-5-20251001",
		Samples:   3,
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges:      []scoring.LLMJudgeDeclaration{judge},
		FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if jr.NormalizedScore == nil || *jr.NormalizedScore != 1.0 {
		t.Fatalf("2 YES + 1 NO should yield score 1.0, got %v", jr.NormalizedScore)
	}
	if jr.Confidence != "high" {
		t.Fatalf("majority vote with no abstains → confidence=high, got %q", jr.Confidence)
	}
}

func TestEvaluator_AssertionTieBreaksToNo(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "YES"},
			{body: "NO"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "k",
		Assertion: "A",
		Model:     "claude-haiku-4-5-20251001",
		Samples:   2,
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges:      []scoring.LLMJudgeDeclaration{judge},
		FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	// Expect defaults to true; tie breaks to NO → mismatch → score 0.
	if jr.NormalizedScore == nil || *jr.NormalizedScore != 0.0 {
		t.Fatalf("1 YES + 1 NO tie should break to NO and score 0.0, got %v", jr.NormalizedScore)
	}
}

func TestEvaluator_AssertionAllUnknownAbstains(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "UNKNOWN: missing context"},
			{body: "UNKNOWN"},
			{body: "UNKNOWN"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "k",
		Assertion: "A",
		Model:     "claude-haiku-4-5-20251001",
		Samples:   3,
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges:      []scoring.LLMJudgeDeclaration{judge},
		FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if jr.State != scoring.OutputStateUnavailable {
		t.Fatalf("all-UNKNOWN should be unavailable, got %q", jr.State)
	}
	if jr.NormalizedScore != nil {
		t.Fatalf("all-UNKNOWN should have nil NormalizedScore, got %v", jr.NormalizedScore)
	}
	if jr.SampleCount != 3 {
		t.Fatalf("SampleCount = %d, want 3 (abstains still count)", jr.SampleCount)
	}
}

func TestEvaluator_AssertionPartialAbstainLowersConfidence(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "YES"},
			{body: "YES"},
			{body: "UNKNOWN"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodAssertion, Key: "k",
		Assertion: "A", Model: "claude-haiku-4-5-20251001", Samples: 3,
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{judge}, FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if jr.NormalizedScore == nil || *jr.NormalizedScore != 1.0 {
		t.Fatalf("2 YES + 1 abstain should still score 1.0, got %v", jr.NormalizedScore)
	}
	if jr.Confidence != "medium" {
		t.Fatalf("1/3 abstain rate → confidence=medium, got %q", jr.Confidence)
	}
}

func TestEvaluator_AssertionMultiModelConsensusUnanimous(t *testing.T) {
	// Two models, 1 sample each, both YES. Unanimous consensus → pass.
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "YES"}, // model A
			{body: "YES"}, // model B
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "k",
		Assertion: "A",
		Models:    []string{"claude-haiku-4-5-20251001", "gpt-4o-mini"},
		Samples:   1,
		Consensus: &scoring.ConsensusConfig{Aggregation: scoring.ConsensusAggUnanimous},
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{judge}, FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if jr.State != scoring.OutputStateAvailable {
		t.Fatalf("unanimous agreement → available, got %q (reason=%q)", jr.State, jr.Reason)
	}
	if jr.NormalizedScore == nil || *jr.NormalizedScore != 1.0 {
		t.Fatalf("NormalizedScore = %v, want 1.0", jr.NormalizedScore)
	}
	if jr.ModelCount != 2 {
		t.Fatalf("ModelCount = %d, want 2", jr.ModelCount)
	}
}

func TestEvaluator_AssertionMultiModelConsensusDisagreement(t *testing.T) {
	// Two models disagree under unanimous → unavailable.
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "YES"}, // model A
			{body: "NO"},  // model B
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode:      scoring.JudgeMethodAssertion,
		Key:       "k",
		Assertion: "A",
		Models:    []string{"claude-haiku-4-5-20251001", "gpt-4o-mini"},
		Samples:   1,
		Consensus: &scoring.ConsensusConfig{Aggregation: scoring.ConsensusAggUnanimous},
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{judge}, FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if jr.State != scoring.OutputStateUnavailable {
		t.Fatalf("disagreement under unanimous → unavailable, got %q", jr.State)
	}
	if !strings.Contains(jr.Reason, "unanimous") {
		t.Fatalf("reason should mention unanimous disagreement, got %q", jr.Reason)
	}
}

func TestEvaluator_AssertionMajorityVoteAcrossModels(t *testing.T) {
	// Three models with majority_vote: 2 YES, 1 NO → YES.
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "YES"},
			{body: "YES"},
			{body: "NO"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodAssertion, Key: "k",
		Assertion: "A",
		Models:    []string{"claude-haiku-4-5-20251001", "gpt-4o-mini", "gemini-2.0-flash"},
		Samples:   1,
		Consensus: &scoring.ConsensusConfig{Aggregation: scoring.ConsensusAggMajorityVote},
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{judge}, FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if jr.State != scoring.OutputStateAvailable || jr.NormalizedScore == nil || *jr.NormalizedScore != 1.0 {
		t.Fatalf("2-of-3 majority → score 1.0, got state=%q score=%v", jr.State, jr.NormalizedScore)
	}
}

func TestEvaluator_ProviderErrorSurfacesAsErrorState(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{err: errors.New("rate limited")},
			{err: errors.New("rate limited")},
			{err: errors.New("rate limited")},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodAssertion, Key: "k",
		Assertion: "A", Model: "claude-haiku-4-5-20251001", Samples: 3,
	}
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{judge}, FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if jr.State != scoring.OutputStateUnavailable {
		t.Fatalf("all provider calls failed → unavailable, got %q", jr.State)
	}
	if jr.NormalizedScore != nil {
		t.Fatalf("NormalizedScore should be nil, got %v", jr.NormalizedScore)
	}
	if jr.SampleCount != 3 {
		t.Fatalf("SampleCount = %d, want 3", jr.SampleCount)
	}
}

func TestEvaluator_PerJudgeErrorDoesntAbortOthers(t *testing.T) {
	// First judge errors every sample, second judge succeeds. Both
	// get results; the run is not aborted.
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			// Judge A — errors on all 3 samples
			{err: errors.New("overloaded")},
			{err: errors.New("overloaded")},
			{err: errors.New("overloaded")},
			// Judge B — all YES
			{body: "YES"},
			{body: "YES"},
			{body: "YES"},
		},
	}
	e := newEvaluatorWithFake(t, fake)

	result, err := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{
			{Mode: scoring.JudgeMethodAssertion, Key: "a", Assertion: "A", Model: "claude-haiku-4-5-20251001", Samples: 3},
			{Mode: scoring.JudgeMethodAssertion, Key: "b", Assertion: "B", Model: "claude-haiku-4-5-20251001", Samples: 3},
		},
		FinalOutput: "out",
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if len(result.JudgeResults) != 2 {
		t.Fatalf("result count = %d, want 2", len(result.JudgeResults))
	}
	if result.JudgeResults[0].State != scoring.OutputStateUnavailable {
		t.Errorf("judge A should be unavailable, got %q", result.JudgeResults[0].State)
	}
	if result.JudgeResults[1].State != scoring.OutputStateAvailable {
		t.Errorf("judge B should be available, got %q (reason=%q)", result.JudgeResults[1].State, result.JudgeResults[1].Reason)
	}
}

// TestEvaluator_RubricModeDispatchedNotPlaceholder pins the Phase 5
// transition: rubric mode is no longer a phase-gated placeholder. It
// now routes to evaluateRubric and runs the full pipeline with the
// fake client. A zero-response fake produces "no samples executed"
// unavailable, NOT a phase-5 placeholder — that's the Phase 5 proof.
func TestEvaluator_RubricModeDispatchedNotPlaceholder(t *testing.T) {
	e := newEvaluatorWithFake(t, &sequencedFakeClient{})
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{
			{Mode: scoring.JudgeMethodRubric, Key: "k", Rubric: "Rate the agent output from 1 to 5 on overall quality, paying attention to clarity, correctness, and completeness.", Model: "claude-sonnet-4-6"},
		},
		FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if strings.Contains(jr.Reason, "phase 5") {
		t.Fatalf("rubric mode should no longer be phase-gated, got reason: %q", jr.Reason)
	}
	// With a fake client returning no canned responses, every sample
	// errors out and the judge surfaces as unavailable. This confirms
	// the dispatch reached evaluateRubric rather than stubbing out.
	if jr.State != scoring.OutputStateUnavailable {
		t.Fatalf("state = %q, want unavailable (all fake samples erred)", jr.State)
	}
}

// TestEvaluator_NWiseModeSkippedFromPerAgentEvaluate pins the Phase 6
// transition: n_wise judges are no longer a placeholder — they're
// actively filtered out of the per-agent Evaluate path and routed to
// EvaluateNWise at the run level. A single n_wise judge passed
// through Evaluate produces a zero-length result slice (not an
// unavailable stub), because the workflow splits judges by mode
// before calling either entry point.
func TestEvaluator_NWiseModeSkippedFromPerAgentEvaluate(t *testing.T) {
	e := newEvaluatorWithFake(t, &sequencedFakeClient{})
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{
			{Mode: scoring.JudgeMethodNWise, Key: "k", Prompt: "rank", Model: "claude-sonnet-4-6"},
		},
	})
	if len(result.JudgeResults) != 0 {
		t.Fatalf("per-agent Evaluate should skip n_wise judges, got %d results: %+v", len(result.JudgeResults), result.JudgeResults)
	}
}

func TestEvaluator_PromptEnvelopeCapturesAssertion(t *testing.T) {
	// Verify the captured provider request includes the judge's
	// assertion text, the agent output delimiters, and the
	// default anti-gaming clause.
	fake := &sequencedFakeClient{responses: []sequencedResponse{{body: "YES"}}}
	e := newEvaluatorWithFake(t, fake)

	_, _ = e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{
			{Mode: scoring.JudgeMethodAssertion, Key: "k", Assertion: "The output mentions refund policy.", Model: "claude-haiku-4-5-20251001", Samples: 1},
		},
		FinalOutput: "Here is your refund.",
	})

	reqs := fake.capturedRequests()
	if len(reqs) != 1 {
		t.Fatalf("captured requests = %d, want 1", len(reqs))
	}
	sys := reqs[0].Messages[0].Content
	user := reqs[0].Messages[1].Content
	if !strings.Contains(sys, defaultAssertionAntiGaming) {
		t.Error("system message missing default anti-gaming clause")
	}
	if !strings.Contains(user, "The output mentions refund policy.") {
		t.Error("user message missing assertion text")
	}
	if !strings.Contains(user, "Here is your refund.") {
		t.Error("user message missing final output")
	}
	if !strings.Contains(user, agentOutputBeginMarker) || !strings.Contains(user, agentOutputEndMarker) {
		t.Error("user message missing agent output delimiters")
	}
}

func TestEvaluator_ContextCancellationMarksCalls(t *testing.T) {
	// Start with a cancelled context. Every call should surface as
	// an error outcome without ever invoking the provider. The
	// evaluator surfaces state=unavailable because no valid samples
	// were collected.
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{{body: "YES"}, {body: "YES"}, {body: "YES"}},
	}
	e := newEvaluatorWithFake(t, fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := e.Evaluate(ctx, Input{
		Judges: []scoring.LLMJudgeDeclaration{
			{Mode: scoring.JudgeMethodAssertion, Key: "k", Assertion: "A", Model: "claude-haiku-4-5-20251001", Samples: 3},
		},
		FinalOutput: "out",
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	jr := result.JudgeResults[0]
	if jr.State != scoring.OutputStateError {
		t.Fatalf("pre-cancelled judge should be state=error, got %q", jr.State)
	}
	if fake.callCount() != 0 {
		t.Errorf("cancelled judge dispatched %d calls, want 0", fake.callCount())
	}
}

func TestEvaluator_ProviderKeyResolvedFromModelPrefix(t *testing.T) {
	// Ensure the per-model prefix fallback correctly routes Claude
	// to anthropic. The captured request should carry ProviderKey=
	// "anthropic".
	fake := &sequencedFakeClient{responses: []sequencedResponse{{body: "YES"}}}
	e := newEvaluatorWithFake(t, fake)

	_, _ = e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{
			{Mode: scoring.JudgeMethodAssertion, Key: "k", Assertion: "A", Model: "claude-sonnet-4-6", Samples: 1},
		},
		FinalOutput: "out",
	})
	reqs := fake.capturedRequests()
	if len(reqs) != 1 {
		t.Fatalf("captured requests = %d, want 1", len(reqs))
	}
	if reqs[0].ProviderKey != "anthropic" {
		t.Errorf("ProviderKey = %q, want anthropic", reqs[0].ProviderKey)
	}
	if reqs[0].CredentialReference != "env:FAKE_KEY" {
		t.Errorf("CredentialReference = %q, want env:FAKE_KEY", reqs[0].CredentialReference)
	}
}

func TestEvaluator_PayloadIsValidJSON(t *testing.T) {
	// The aggregated result carries a jsonb payload that the Phase 4
	// activity will persist via repository.UpsertLLMJudgeResult. It
	// must round-trip cleanly through json.Unmarshal.
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{{body: "YES"}, {body: "YES"}, {body: "NO"}},
	}
	e := newEvaluatorWithFake(t, fake)
	result, _ := e.Evaluate(context.Background(), Input{
		Judges: []scoring.LLMJudgeDeclaration{
			{Mode: scoring.JudgeMethodAssertion, Key: "k", Assertion: "A", Model: "claude-haiku-4-5-20251001", Samples: 3},
		},
		FinalOutput: "out",
	})
	jr := result.JudgeResults[0]
	if len(jr.Payload) == 0 {
		t.Fatal("payload is empty, want serialised assertion payload")
	}
	var decoded map[string]any
	if err := json.Unmarshal(jr.Payload, &decoded); err != nil {
		t.Fatalf("payload not valid JSON: %v\npayload=%s", err, jr.Payload)
	}
	if decoded["mode"] != "assertion" {
		t.Errorf("payload.mode = %v, want assertion", decoded["mode"])
	}
	if decoded["available"] != true {
		t.Errorf("payload.available = %v, want true", decoded["available"])
	}
}
