package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

// Phase 6 of issue #148 — n_wise mode tests. Covers:
//   • parseNWiseResponse: strict, code-fence, prose, abstain, malformed
//   • extractJSONObject tier reuse
//   • generateSampleOrderings: cyclic, fixed, fallback warnings
//   • mapRankingToAgentRanks: valid mapping, label errors, duplicates,
//     missing agents, out-of-range rank
//   • labelIndex: A..Z, invalid cases
//   • computeBordaPoints math
//   • finalRanksFromBorda with ties
//   • normalizeBorda edge cases
//   • deriveNWiseConfidence: full/partial/scattered/single-sample
//   • truncateForNWise rune-aware truncation
//   • buildNWisePrompt golden test (inline constant)
//   • EvaluateNWise end-to-end via sequencedFakeClient for 3-agent,
//     2-agent, N=1 unavailable, N>26 error, all-samples-parse-fail,
//     one-agent-empty-output filtered
//   • Multi-model warning and use-first

// --- Small helpers ---

func makeAgents(labels ...string) []NWiseAgent {
	agents := make([]NWiseAgent, 0, len(labels))
	for _, label := range labels {
		agents = append(agents, NWiseAgent{
			RunAgentID:  uuid.New(),
			Label:       label,
			FinalOutput: fmt.Sprintf("output for %s", label),
		})
	}
	return agents
}

// newSerialEvaluatorWithFake returns an evaluator with MaxParallel=1
// so the sequencedFakeClient's atomic counter lines up with sample
// dispatch order. n_wise aggregation tests that need response N to
// match sample N must use this helper — otherwise parallel fan-out
// can assign responses in wall-clock order.
func newSerialEvaluatorWithFake(t *testing.T, fake provider.Client) *Evaluator {
	t.Helper()
	router := provider.NewRouter(map[string]provider.Client{
		"anthropic": fake,
		"openai":    fake,
		"gemini":    fake,
		"mistral":   fake,
	})
	return NewEvaluator(router, Config{
		MaxParallel:         1,
		CredentialReference: "env:FAKE_KEY",
	})
}

// sampleRankingResponse builds a JSON response body for n_wise
// parsing tests. Takes a list of (agent_label, rank) pairs.
func sampleRankingResponse(entries ...[2]any) string {
	type rank struct {
		AgentLabel string `json:"agent_label"`
		Rank       int    `json:"rank"`
	}
	shaped := struct {
		Ranking []rank `json:"ranking"`
	}{}
	for _, e := range entries {
		shaped.Ranking = append(shaped.Ranking, rank{
			AgentLabel: e[0].(string),
			Rank:       e[1].(int),
		})
	}
	buf, _ := json.Marshal(shaped)
	return string(buf)
}

// --- parseNWiseResponse: parser tiers ---

func TestParseNWiseResponse_StrictJSON(t *testing.T) {
	text := sampleRankingResponse([2]any{"A", 1}, [2]any{"B", 2}, [2]any{"C", 3})
	parsed, ok := parseNWiseResponse(text, defaultNWiseSchema)
	if !ok {
		t.Fatal("parseNWiseResponse ok=false, want true")
	}
	if len(parsed.Ranking) != 3 {
		t.Fatalf("ranking count = %d, want 3", len(parsed.Ranking))
	}
}

func TestParseNWiseResponse_CodeFenceWrapped(t *testing.T) {
	inner := sampleRankingResponse([2]any{"A", 2}, [2]any{"B", 1})
	text := "```json\n" + inner + "\n```"
	parsed, ok := parseNWiseResponse(text, defaultNWiseSchema)
	if !ok || len(parsed.Ranking) != 2 {
		t.Fatalf("code-fence wrapped ranking not parsed: %+v ok=%v", parsed, ok)
	}
}

func TestParseNWiseResponse_ProsePrologue(t *testing.T) {
	inner := sampleRankingResponse([2]any{"A", 1}, [2]any{"B", 2})
	text := "Here is my analysis: " + inner + " and that's my final answer."
	parsed, ok := parseNWiseResponse(text, defaultNWiseSchema)
	if !ok || len(parsed.Ranking) != 2 {
		t.Fatalf("prose prolog not parsed: %+v ok=%v", parsed, ok)
	}
}

func TestParseNWiseResponse_UnableToJudge(t *testing.T) {
	text := `{"unable_to_judge": true, "reason": "outputs are too similar to rank"}`
	parsed, ok := parseNWiseResponse(text, defaultNWiseSchema)
	if !ok {
		t.Fatal("abstain should return ok=true")
	}
	if !parsed.UnableToJudge {
		t.Error("UnableToJudge not set")
	}
	if parsed.AbstainReason != "outputs are too similar to rank" {
		t.Errorf("AbstainReason = %q", parsed.AbstainReason)
	}
}

func TestParseNWiseResponse_EmptyRanking(t *testing.T) {
	text := `{"ranking": []}`
	_, ok := parseNWiseResponse(text, defaultNWiseSchema)
	if ok {
		t.Error("empty ranking should fail parse")
	}
}

func TestParseNWiseResponse_MissingRequiredField(t *testing.T) {
	text := `{"ranking": [{"agent_label": "A"}]}`
	_, ok := parseNWiseResponse(text, defaultNWiseSchema)
	if ok {
		t.Error("ranking missing rank should fail schema validation")
	}
}

func TestParseNWiseResponse_MalformedJSON(t *testing.T) {
	_, ok := parseNWiseResponse("not json at all", defaultNWiseSchema)
	if ok {
		t.Error("malformed should fail")
	}
}

// --- generateSampleOrderings ---

func TestGenerateSampleOrderings_CyclicN3Samples3(t *testing.T) {
	orderings, warn := generateSampleOrderings(3, 3, true)
	if warn != "" {
		t.Errorf("unexpected warning: %q", warn)
	}
	want := [][]int{
		{0, 1, 2},
		{1, 2, 0},
		{2, 0, 1},
	}
	for i, got := range orderings {
		if len(got) != 3 {
			t.Fatalf("sample %d length = %d, want 3", i, len(got))
		}
		for j, v := range got {
			if v != want[i][j] {
				t.Errorf("sample %d slot %d = %d, want %d", i, j, v, want[i][j])
			}
		}
	}
}

func TestGenerateSampleOrderings_CyclicSamplesGreaterThanN(t *testing.T) {
	// samples=5, n=3 → cycle back: shifts 0,1,2,0,1
	orderings, warn := generateSampleOrderings(3, 5, true)
	if warn != "" {
		t.Errorf("unexpected warning: %q", warn)
	}
	if len(orderings) != 5 {
		t.Fatalf("sample count = %d, want 5", len(orderings))
	}
	// Sample 0 and sample 3 should be identical (both shift=0)
	for j := 0; j < 3; j++ {
		if orderings[0][j] != orderings[3][j] {
			t.Errorf("sample 0 and sample 3 should have same ordering, differ at slot %d", j)
		}
	}
}

func TestGenerateSampleOrderings_SamplesLessThanN_FallsBackWithWarn(t *testing.T) {
	// samples=2, n=3, debias=true → fallback with warning
	orderings, warn := generateSampleOrderings(3, 2, true)
	if warn == "" {
		t.Error("should emit warning when samples < N")
	}
	if !strings.Contains(warn, "samples >= N_agents") {
		t.Errorf("warning should explain the fallback: %q", warn)
	}
	// All samples should use identity ordering
	for i, got := range orderings {
		for j, v := range got {
			if v != j {
				t.Errorf("sample %d slot %d = %d, want %d (fallback should be identity)", i, j, v, j)
			}
		}
	}
}

func TestGenerateSampleOrderings_DebiasDisabledAllIdentity(t *testing.T) {
	orderings, warn := generateSampleOrderings(4, 4, false)
	if warn != "" {
		t.Errorf("debias disabled should emit no warning, got %q", warn)
	}
	for i, got := range orderings {
		for j, v := range got {
			if v != j {
				t.Errorf("sample %d slot %d = %d, want %d", i, j, v, j)
			}
		}
	}
}

func TestGenerateSampleOrderings_N2Samples2(t *testing.T) {
	orderings, _ := generateSampleOrderings(2, 2, true)
	want := [][]int{{0, 1}, {1, 0}}
	for i := range orderings {
		for j := range orderings[i] {
			if orderings[i][j] != want[i][j] {
				t.Errorf("sample %d slot %d = %d, want %d", i, j, orderings[i][j], want[i][j])
			}
		}
	}
}

func TestGenerateSampleOrderings_ZeroSamplesDefaults(t *testing.T) {
	// samples=0 → use JudgeDefaultSamples (3)
	orderings, _ := generateSampleOrderings(3, 0, true)
	if len(orderings) != scoring.JudgeDefaultSamples {
		t.Errorf("sample count = %d, want %d", len(orderings), scoring.JudgeDefaultSamples)
	}
}

// --- labelIndex + nwiseLabelAt ---

func TestLabelIndex(t *testing.T) {
	cases := []struct {
		label string
		want  int
	}{
		{"A", 0},
		{"B", 1},
		{"Z", 25},
		{"a", 0}, // case-insensitive
		{" B ", 1},
		{"AA", -1}, // too long
		{"", -1},
		{"1", -1},
		{"[", -1}, // non-alpha
	}
	for _, tc := range cases {
		got := labelIndex(tc.label)
		if got != tc.want {
			t.Errorf("labelIndex(%q) = %d, want %d", tc.label, got, tc.want)
		}
	}
}

func TestNwiseLabelAt(t *testing.T) {
	if nwiseLabelAt(0) != "A" {
		t.Error("slot 0 should be A")
	}
	if nwiseLabelAt(25) != "Z" {
		t.Error("slot 25 should be Z")
	}
	// Beyond-range defensive fallback
	if got := nwiseLabelAt(30); got == "" {
		t.Error("beyond-range should return non-empty defensive label")
	}
}

// --- mapRankingToAgentRanks ---

func TestMapRankingToAgentRanks_Valid(t *testing.T) {
	// 3 agents, ordering [0,1,2] → A=agent[0], B=agent[1], C=agent[2]
	// LLM ranks: A=1, B=2, C=3
	ranking := []nwiseRankEntry{
		{AgentLabel: "A", Rank: 1},
		{AgentLabel: "B", Rank: 2},
		{AgentLabel: "C", Rank: 3},
	}
	ranks, _ := mapRankingToAgentRanks(ranking, 3, []int{0, 1, 2})
	if ranks == nil {
		t.Fatal("valid ranking should map successfully")
	}
	if ranks[0] != 1 || ranks[1] != 2 || ranks[2] != 3 {
		t.Errorf("got %v, want [1, 2, 3]", ranks)
	}
}

func TestMapRankingToAgentRanks_ShuffledOrdering(t *testing.T) {
	// ordering [1, 2, 0] → A=agent[1], B=agent[2], C=agent[0]
	// LLM ranks A=1, B=2, C=3 → agent[1]=rank 1, agent[2]=rank 2, agent[0]=rank 3
	ranking := []nwiseRankEntry{
		{AgentLabel: "A", Rank: 1},
		{AgentLabel: "B", Rank: 2},
		{AgentLabel: "C", Rank: 3},
	}
	ranks, _ := mapRankingToAgentRanks(ranking, 3, []int{1, 2, 0})
	if ranks == nil {
		t.Fatal("should map successfully")
	}
	if ranks[0] != 3 || ranks[1] != 1 || ranks[2] != 2 {
		t.Errorf("got %v, want [3, 1, 2]", ranks)
	}
}

func TestMapRankingToAgentRanks_MissingAgent(t *testing.T) {
	ranking := []nwiseRankEntry{
		{AgentLabel: "A", Rank: 1},
		{AgentLabel: "B", Rank: 2},
	}
	ranks, _ := mapRankingToAgentRanks(ranking, 3, []int{0, 1, 2})
	if ranks != nil {
		t.Error("missing agent should return nil")
	}
}

func TestMapRankingToAgentRanks_DuplicateAgent(t *testing.T) {
	// Two entries for label A (both map to agent[0]) — invalid
	ranking := []nwiseRankEntry{
		{AgentLabel: "A", Rank: 1},
		{AgentLabel: "A", Rank: 2},
		{AgentLabel: "C", Rank: 3},
	}
	ranks, _ := mapRankingToAgentRanks(ranking, 3, []int{0, 1, 2})
	if ranks != nil {
		t.Error("duplicate agent should return nil")
	}
}

func TestMapRankingToAgentRanks_UnknownLabel(t *testing.T) {
	ranking := []nwiseRankEntry{
		{AgentLabel: "X", Rank: 1}, // X is out of range for n=3
		{AgentLabel: "B", Rank: 2},
		{AgentLabel: "C", Rank: 3},
	}
	ranks, _ := mapRankingToAgentRanks(ranking, 3, []int{0, 1, 2})
	if ranks != nil {
		t.Error("unknown label should return nil")
	}
}

func TestMapRankingToAgentRanks_OutOfRangeRank(t *testing.T) {
	ranking := []nwiseRankEntry{
		{AgentLabel: "A", Rank: 5}, // rank 5 for n=3 is invalid
		{AgentLabel: "B", Rank: 1},
		{AgentLabel: "C", Rank: 2},
	}
	ranks, _ := mapRankingToAgentRanks(ranking, 3, []int{0, 1, 2})
	if ranks != nil {
		t.Error("out-of-range rank should return nil")
	}
}

// --- computeBordaPoints + finalRanksFromBorda + normalizeBorda ---

func TestComputeBordaPoints_KnownDistribution(t *testing.T) {
	// 3 agents, 2 samples
	// Sample 0: agent[0]=1, agent[1]=2, agent[2]=3
	// Sample 1: agent[0]=1, agent[1]=3, agent[2]=2
	// Borda points per agent (n=3):
	//   agent[0]: (3-1)+(3-1) = 2+2 = 4
	//   agent[1]: (3-2)+(3-3) = 1+0 = 1
	//   agent[2]: (3-3)+(3-2) = 0+1 = 1
	rankings := [][]int{
		{1, 2, 3},
		{1, 3, 2},
	}
	points := computeBordaPoints(rankings, 3)
	if points[0] != 4 || points[1] != 1 || points[2] != 1 {
		t.Errorf("got %v, want [4, 1, 1]", points)
	}
}

func TestComputeBordaPoints_SkipsNilSamples(t *testing.T) {
	rankings := [][]int{
		{1, 2, 3},
		nil, // failed sample
		{1, 2, 3},
	}
	points := computeBordaPoints(rankings, 3)
	// Only 2 valid samples, same rankings → agent[0]=4, agent[1]=2, agent[2]=0
	if points[0] != 4 || points[1] != 2 || points[2] != 0 {
		t.Errorf("got %v, want [4, 2, 0]", points)
	}
}

func TestFinalRanksFromBorda_TiesBreakToSameRank(t *testing.T) {
	// Points [10, 5, 5, 2] → ranks [1, 2, 2, 4]
	points := []int{10, 5, 5, 2}
	ranks := finalRanksFromBorda(points)
	if ranks[0] != 1 {
		t.Errorf("highest points → rank 1, got %d", ranks[0])
	}
	if ranks[1] != 2 || ranks[2] != 2 {
		t.Errorf("tied second → both rank 2, got %d and %d", ranks[1], ranks[2])
	}
	if ranks[3] != 4 {
		t.Errorf("after tie → rank 4 (skip), got %d", ranks[3])
	}
}

func TestNormalizeBorda_AlwaysFirstIsOne(t *testing.T) {
	// 3 samples, 3 agents, agent always rank 1
	// Points per sample = n - 1 = 2
	// Total = 2 * 3 = 6
	// Denom = 3 * (3 - 1) = 6
	// Normalized = 1.0
	got := normalizeBorda(6, 3, 3)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("always-first → 1.0, got %v", got)
	}
}

func TestNormalizeBorda_AlwaysLastIsZero(t *testing.T) {
	got := normalizeBorda(0, 3, 3)
	if got != 0 {
		t.Errorf("always-last → 0, got %v", got)
	}
}

func TestNormalizeBorda_EdgeCases(t *testing.T) {
	// N=1 → denom 0, guard returns 0
	if got := normalizeBorda(0, 3, 1); got != 0 {
		t.Errorf("N=1 → 0, got %v", got)
	}
	// samples=0 → guard returns 0
	if got := normalizeBorda(5, 0, 3); got != 0 {
		t.Errorf("samples=0 → 0, got %v", got)
	}
}

// --- deriveNWiseConfidence ---

func TestDeriveNWiseConfidence_FullConsistency(t *testing.T) {
	rankings := [][]int{
		{1, 2, 3},
		{1, 2, 3},
		{1, 2, 3},
	}
	// Agent 0 always rank 1 → high
	got := deriveNWiseConfidence(rankings, 0, 3)
	if got != "high" {
		t.Errorf("full consistency → high, got %q", got)
	}
}

func TestDeriveNWiseConfidence_PartialConsistency(t *testing.T) {
	rankings := [][]int{
		{1, 2, 3},
		{1, 2, 3},
		{2, 1, 3}, // agent 0 at rank 2 this time
	}
	// Agent 0: rank 1 twice, rank 2 once → 2/3 ≈ 0.67 → medium (boundary)
	got := deriveNWiseConfidence(rankings, 0, 3)
	if got != "medium" {
		t.Errorf("2/3 consistency → medium, got %q", got)
	}
}

func TestDeriveNWiseConfidence_Scattered(t *testing.T) {
	rankings := [][]int{
		{1, 2, 3},
		{2, 1, 3},
		{3, 1, 2},
	}
	// Agent 0: 1, 2, 3 → 1/3 → low
	got := deriveNWiseConfidence(rankings, 0, 3)
	if got != "low" {
		t.Errorf("scattered → low, got %q", got)
	}
}

func TestDeriveNWiseConfidence_SingleSampleIsMedium(t *testing.T) {
	rankings := [][]int{
		{1, 2, 3},
	}
	got := deriveNWiseConfidence(rankings, 0, 1)
	if got != "medium" {
		t.Errorf("single sample → medium (downgrade), got %q", got)
	}
}

// --- truncateForNWise ---

func TestTruncateForNWise_ShortOutputUnchanged(t *testing.T) {
	got := truncateForNWise("hello world", 1000)
	if got != "hello world" {
		t.Errorf("short output should not be truncated, got %q", got)
	}
}

func TestTruncateForNWise_LongOutputGetsMarker(t *testing.T) {
	input := strings.Repeat("x", 5000)
	got := truncateForNWise(input, 100)
	if !strings.Contains(got, "[... truncated ...]") {
		t.Error("truncated output missing marker")
	}
	if len([]rune(got)) > 100 {
		t.Errorf("truncated output longer than cap: %d > 100", len([]rune(got)))
	}
}

// --- labelForAgentIndexInOrdering ---

func TestLabelForAgentIndexInOrdering(t *testing.T) {
	// ordering [2, 0, 1] → slot A=agent[2], slot B=agent[0], slot C=agent[1]
	ordering := []int{2, 0, 1}
	if got := labelForAgentIndexInOrdering(2, ordering); got != "A" {
		t.Errorf("agent 2 → slot A, got %q", got)
	}
	if got := labelForAgentIndexInOrdering(0, ordering); got != "B" {
		t.Errorf("agent 0 → slot B, got %q", got)
	}
	if got := labelForAgentIndexInOrdering(1, ordering); got != "C" {
		t.Errorf("agent 1 → slot C, got %q", got)
	}
	if got := labelForAgentIndexInOrdering(99, ordering); got != "" {
		t.Errorf("unknown agent → empty, got %q", got)
	}
}

// --- buildNWisePrompt golden test ---

const goldenNWiseUser = `RANKING PROMPT:
Rank the agents by overall quality of their responses.

=== AGENT A OUTPUT ===
alpha output
=== END AGENT A OUTPUT ===

=== AGENT B OUTPUT ===
beta output
=== END AGENT B OUTPUT ===

RESPONSE SCHEMA: respond with a JSON object containing a "ranking" array. Each entry must include "agent_label" (one of the labels shown above) and "rank" (integer, 1 = best, higher = worse). Optionally add "reasoning" per agent. Every agent shown must appear in the ranking exactly once.

Your response (JSON only):`

func TestBuildNWisePrompt_GoldenMinimal(t *testing.T) {
	judge := scoring.LLMJudgeDeclaration{
		Mode:   scoring.JudgeMethodNWise,
		Key:    "rank_q",
		Prompt: "Rank the agents by overall quality of their responses.",
		Model:  "claude-sonnet-4-6",
	}
	agents := []NWiseAgent{
		{RunAgentID: uuid.New(), Label: "alpha", FinalOutput: "alpha output"},
		{RunAgentID: uuid.New(), Label: "beta", FinalOutput: "beta output"},
	}
	_, user, truncated := buildNWisePrompt(judge, agents, []int{0, 1}, nil, 4000)
	if len(truncated) != 0 {
		t.Errorf("no agents should be truncated, got %v", truncated)
	}
	if user != goldenNWiseUser {
		t.Errorf("user prompt drift.\nGOT:\n%s\n---\nWANT:\n%s", user, goldenNWiseUser)
	}
}

func TestBuildNWisePrompt_ShuffledOrdering(t *testing.T) {
	// ordering [1, 0] should show agent[1] under label A and agent[0] under label B.
	judge := scoring.LLMJudgeDeclaration{
		Mode:   scoring.JudgeMethodNWise,
		Key:    "k",
		Prompt: "Rank them.",
		Model:  "claude-sonnet-4-6",
	}
	agents := []NWiseAgent{
		{RunAgentID: uuid.New(), Label: "alpha", FinalOutput: "FIRST"},
		{RunAgentID: uuid.New(), Label: "beta", FinalOutput: "SECOND"},
	}
	_, user, _ := buildNWisePrompt(judge, agents, []int{1, 0}, nil, 4000)
	// Agent A block should contain SECOND (agents[1]'s output)
	aBlock := "=== AGENT A OUTPUT ===\nSECOND\n=== END AGENT A OUTPUT ==="
	bBlock := "=== AGENT B OUTPUT ===\nFIRST\n=== END AGENT B OUTPUT ==="
	if !strings.Contains(user, aBlock) {
		t.Errorf("agent A block missing SECOND: %s", user)
	}
	if !strings.Contains(user, bBlock) {
		t.Errorf("agent B block missing FIRST: %s", user)
	}
}

func TestBuildNWisePrompt_TruncationReported(t *testing.T) {
	judge := scoring.LLMJudgeDeclaration{
		Mode:   scoring.JudgeMethodNWise,
		Key:    "k",
		Prompt: "Rank.",
		Model:  "claude-sonnet-4-6",
	}
	long := strings.Repeat("a", 5000)
	agents := []NWiseAgent{
		{RunAgentID: uuid.New(), Label: "alpha", FinalOutput: long},
		{RunAgentID: uuid.New(), Label: "beta", FinalOutput: "short"},
	}
	_, user, truncated := buildNWisePrompt(judge, agents, []int{0, 1}, nil, 100)
	if len(truncated) != 1 || truncated[0] != "A" {
		t.Errorf("truncation should report [A], got %v", truncated)
	}
	if !strings.Contains(user, "[... truncated ...]") {
		t.Error("user prompt should contain truncation marker")
	}
}

// --- EvaluateNWise end-to-end ---

func TestEvaluateNWise_ThreeAgentsHappyPath(t *testing.T) {
	// 3 agents × 3 samples with cyclic shifts.
	// Sample 0 ordering [0,1,2] → A=agent[0], B=agent[1], C=agent[2]
	// Sample 1 ordering [1,2,0] → A=agent[1], B=agent[2], C=agent[0]
	// Sample 2 ordering [2,0,1] → A=agent[2], B=agent[0], C=agent[1]
	// LLM consistently ranks the "true" best agent (agent[0]) first.
	// Per sample, that means:
	//   Sample 0: agent[0] gets rank 1 (shown as A)
	//   Sample 1: agent[0] gets rank 1 (shown as C)
	//   Sample 2: agent[0] gets rank 1 (shown as B)
	// We need the LLM response to put agent[0] first in every sample,
	// which means: sample 0 → A=1, sample 1 → C=1, sample 2 → B=1.
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: `{"ranking":[{"agent_label":"A","rank":1},{"agent_label":"B","rank":2},{"agent_label":"C","rank":3}]}`},
			{body: `{"ranking":[{"agent_label":"C","rank":1},{"agent_label":"A","rank":2},{"agent_label":"B","rank":3}]}`},
			{body: `{"ranking":[{"agent_label":"B","rank":1},{"agent_label":"C","rank":2},{"agent_label":"A","rank":3}]}`},
		},
	}
	e := newSerialEvaluatorWithFake(t, fake)
	agents := makeAgents("gpt-4o", "claude", "gemini")
	judge := scoring.LLMJudgeDeclaration{
		Mode:              scoring.JudgeMethodNWise,
		Key:               "quality_ranking",
		Prompt:            "Rank the agents by overall quality.",
		Model:             "claude-sonnet-4-6",
		Samples:           3,
		PositionDebiasing: true,
	}
	result, err := e.EvaluateNWise(context.Background(), NWiseInput{
		RunID:            uuid.New(),
		EvaluationSpecID: uuid.New(),
		Judges:           []scoring.LLMJudgeDeclaration{judge},
		Agents:           agents,
	})
	if err != nil {
		t.Fatalf("EvaluateNWise error: %v", err)
	}
	if len(result.PerAgent) != 3 {
		t.Fatalf("PerAgent has %d entries, want 3", len(result.PerAgent))
	}

	// agent[0] should have highest normalized score (always rank 1)
	agent0Result := result.PerAgent[agents[0].RunAgentID]
	if len(agent0Result) != 1 {
		t.Fatalf("agent[0] should have 1 judge result, got %d", len(agent0Result))
	}
	jr := agent0Result[0]
	if jr.State != scoring.OutputStateAvailable {
		t.Fatalf("agent[0] state = %q, reason = %q", jr.State, jr.Reason)
	}
	if jr.NormalizedScore == nil || *jr.NormalizedScore != 1.0 {
		t.Errorf("agent[0] always-first → normalized 1.0, got %v", jr.NormalizedScore)
	}
	if jr.Confidence != "high" {
		t.Errorf("agent[0] full consistency → high, got %q", jr.Confidence)
	}
}

func TestEvaluateNWise_TwoAgentsSingleSample(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: `{"ranking":[{"agent_label":"A","rank":1},{"agent_label":"B","rank":2}]}`},
		},
	}
	e := newEvaluatorWithFake(t, fake)
	agents := makeAgents("alpha", "beta")
	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodNWise, Key: "k",
		Prompt: "Rank.", Model: "claude-sonnet-4-6", Samples: 1,
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	jr0 := result.PerAgent[agents[0].RunAgentID][0]
	jr1 := result.PerAgent[agents[1].RunAgentID][0]
	if jr0.NormalizedScore == nil || *jr0.NormalizedScore != 1.0 {
		t.Errorf("agent[0] rank 1 → 1.0, got %v", jr0.NormalizedScore)
	}
	if jr1.NormalizedScore == nil || *jr1.NormalizedScore != 0.0 {
		t.Errorf("agent[1] rank 2 → 0.0, got %v", jr1.NormalizedScore)
	}
	if jr0.Confidence != "medium" {
		t.Errorf("single sample → medium confidence, got %q", jr0.Confidence)
	}
}

func TestEvaluateNWise_SingleAgentIsUnavailable(t *testing.T) {
	e := newEvaluatorWithFake(t, &sequencedFakeClient{})
	agents := makeAgents("only")
	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodNWise, Key: "k",
		Prompt: "Rank.", Model: "claude-sonnet-4-6",
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	jr := result.PerAgent[agents[0].RunAgentID][0]
	if jr.State != scoring.OutputStateUnavailable {
		t.Errorf("N=1 → unavailable, got %q", jr.State)
	}
	if !strings.Contains(jr.Reason, "at least 2 agents") {
		t.Errorf("reason should mention at-least-2, got %q", jr.Reason)
	}
}

func TestEvaluateNWise_TooManyAgentsIsError(t *testing.T) {
	e := newEvaluatorWithFake(t, &sequencedFakeClient{})
	labels := make([]string, 27)
	for i := range labels {
		labels[i] = fmt.Sprintf("agent-%d", i)
	}
	agents := makeAgents(labels...)
	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodNWise, Key: "k",
		Prompt: "Rank.", Model: "claude-sonnet-4-6",
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	jr := result.PerAgent[agents[0].RunAgentID][0]
	if jr.State != scoring.OutputStateError {
		t.Errorf("N=27 → error, got %q", jr.State)
	}
}

func TestEvaluateNWise_EmptyOutputFiltered(t *testing.T) {
	// 3 agents declared but agent[1] has empty output → effective N=2
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: `{"ranking":[{"agent_label":"A","rank":1},{"agent_label":"B","rank":2}]}`},
		},
	}
	e := newEvaluatorWithFake(t, fake)
	agents := []NWiseAgent{
		{RunAgentID: uuid.New(), Label: "a", FinalOutput: "first"},
		{RunAgentID: uuid.New(), Label: "b", FinalOutput: ""},
		{RunAgentID: uuid.New(), Label: "c", FinalOutput: "third"},
	}
	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodNWise, Key: "k",
		Prompt: "Rank.", Model: "claude-sonnet-4-6", Samples: 1,
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	// Warning should mention the filtered agent
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, agents[1].RunAgentID.String()) && strings.Contains(w, "empty final output") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("should warn about filtered agent, warnings = %v", result.Warnings)
	}
}

func TestEvaluateNWise_AllSamplesParseFail(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: "not json"},
			{body: "{invalid"},
			{body: "still not json"},
		},
	}
	e := newEvaluatorWithFake(t, fake)
	agents := makeAgents("a", "b")
	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodNWise, Key: "k",
		Prompt: "Rank.", Model: "claude-sonnet-4-6", Samples: 3,
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	jr := result.PerAgent[agents[0].RunAgentID][0]
	if jr.State != scoring.OutputStateUnavailable {
		t.Errorf("all parse-fail → unavailable, got %q", jr.State)
	}
}

func TestEvaluateNWise_UnableToJudgeAbstains(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: `{"unable_to_judge": true, "reason": "too similar"}`},
			{body: `{"unable_to_judge": true}`},
			{body: `{"unable_to_judge": true}`},
		},
	}
	e := newEvaluatorWithFake(t, fake)
	agents := makeAgents("a", "b")
	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodNWise, Key: "k",
		Prompt: "Rank.", Model: "claude-sonnet-4-6", Samples: 3,
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	jr := result.PerAgent[agents[0].RunAgentID][0]
	if jr.State != scoring.OutputStateUnavailable {
		t.Errorf("all abstain → unavailable, got %q", jr.State)
	}
}

func TestEvaluateNWise_MultiModelUsesFirstAndWarns(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: `{"ranking":[{"agent_label":"A","rank":1},{"agent_label":"B","rank":2}]}`},
		},
	}
	e := newEvaluatorWithFake(t, fake)
	agents := makeAgents("a", "b")
	judge := scoring.LLMJudgeDeclaration{
		Mode:    scoring.JudgeMethodNWise,
		Key:     "k",
		Prompt:  "Rank.",
		Models:  []string{"claude-sonnet-4-6", "gpt-4o"},
		Samples: 1,
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	jr := result.PerAgent[agents[0].RunAgentID][0]
	if jr.State != scoring.OutputStateAvailable {
		t.Errorf("multi-model should still produce a result, got %q", jr.State)
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "multi-model") && strings.Contains(w, "Phase 7") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("multi-model should warn about Phase 7 scope, warnings = %v", result.Warnings)
	}
}

func TestEvaluateNWise_NoNWiseJudgesIsNoop(t *testing.T) {
	e := newEvaluatorWithFake(t, &sequencedFakeClient{})
	agents := makeAgents("a", "b")
	// Spec has only non-n_wise judges
	judges := []scoring.LLMJudgeDeclaration{
		{Mode: scoring.JudgeMethodAssertion, Key: "x", Assertion: "Check.", Model: "claude-haiku-4-5-20251001"},
	}
	result, err := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: judges, Agents: agents,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// perAgent is pre-seeded with empty slices but has no actual results
	for _, entries := range result.PerAgent {
		if len(entries) != 0 {
			t.Errorf("expected no results, got %d", len(entries))
		}
	}
}

func TestEvaluateNWise_PayloadRoundTripValid(t *testing.T) {
	fake := &sequencedFakeClient{
		responses: []sequencedResponse{
			{body: `{"ranking":[{"agent_label":"A","rank":1,"reasoning":"clearest"},{"agent_label":"B","rank":2}]}`},
		},
	}
	e := newEvaluatorWithFake(t, fake)
	agents := makeAgents("a", "b")
	judge := scoring.LLMJudgeDeclaration{
		Mode: scoring.JudgeMethodNWise, Key: "k",
		Prompt: "Rank.", Model: "claude-sonnet-4-6", Samples: 1,
	}
	result, _ := e.EvaluateNWise(context.Background(), NWiseInput{
		Judges: []scoring.LLMJudgeDeclaration{judge}, Agents: agents,
	})
	jr := result.PerAgent[agents[0].RunAgentID][0]
	if len(jr.Payload) == 0 {
		t.Fatal("payload empty")
	}
	var decoded map[string]any
	if err := json.Unmarshal(jr.Payload, &decoded); err != nil {
		t.Fatalf("payload not valid JSON: %v\npayload: %s", err, jr.Payload)
	}
	if decoded["mode"] != "n_wise" {
		t.Errorf("mode = %v, want n_wise", decoded["mode"])
	}
	// Verify rank + borda_points are populated
	if _, ok := decoded["final_rank"]; !ok {
		t.Error("payload missing final_rank")
	}
	if _, ok := decoded["borda_points"]; !ok {
		t.Error("payload missing borda_points")
	}
}
