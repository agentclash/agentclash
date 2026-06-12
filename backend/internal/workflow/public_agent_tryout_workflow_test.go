package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

// fakePublicTryoutRepo is a minimal PublicAgentTryoutRepository capturing
// recorded timeline events for assertions.
type fakePublicTryoutRepo struct {
	events []repository.RecordAgentTryoutEventParams
}

func (f *fakePublicTryoutRepo) GetAgentTryoutByID(context.Context, uuid.UUID) (repository.AgentTryout, error) {
	return repository.AgentTryout{}, nil
}

func (f *fakePublicTryoutRepo) UpdateAgentTryoutStatus(_ context.Context, params repository.UpdateAgentTryoutStatusParams) (repository.AgentTryout, error) {
	return repository.AgentTryout{ID: params.ID}, nil
}

func (f *fakePublicTryoutRepo) RecordAgentTryoutEvent(_ context.Context, params repository.RecordAgentTryoutEventParams) (repository.AgentTryoutEvent, error) {
	f.events = append(f.events, params)
	return repository.AgentTryoutEvent{}, nil
}

func (f *fakePublicTryoutRepo) ClaimNextPendingAgentTryoutTurn(context.Context, uuid.UUID) (repository.AgentTryoutTurn, bool, error) {
	return repository.AgentTryoutTurn{}, false, nil
}

func (f *fakePublicTryoutRepo) MarkAgentTryoutTurnProcessed(context.Context, int64) error {
	return nil
}

func publicJudgeSpecSnapshot() json.RawMessage {
	return json.RawMessage(`{
		"validators":[{"key":"has_summary","type":"json_field","field":"summary"}],
		"judge_mode":"hybrid",
		"llm_judges":[
			{"mode":"rubric","key":"overall_quality","model":"gpt-5-mini","rubric":"Score the work 1-5 against the operator's bar.","context_from":["final_output"],"samples":1,"score_scale":{"min":1,"max":5}},
			{"mode":"assertion","key":"instant_fail","model":"gpt-5-mini","assertion":"The output does not invent numbers.","context_from":["final_output"],"samples":1}
		],
		"judge_meta":{"model":"gpt-5-mini","strictness":"standard","labels":{"overall_quality":"Overall, against your bar","instant_fail":"Avoids your instant fail"}}
	}`)
}

func TestRunPublicTryoutJudgesMergesVerdictAndReasoning(t *testing.T) {
	repo := &fakePublicTryoutRepo{}
	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5-mini",
			OutputText:      `{"score":5,"pass":true,"confidence":"high","reasoning":"Grounded in the supplied notes; nothing fabricated."}`,
		},
	}
	activities := &Activities{publicTryoutRepo: repo, judgeClient: client}

	section := activities.runPublicTryoutJudges(context.Background(), repository.AgentTryout{
		ID:                     uuid.New(),
		EvaluationSpecSnapshot: publicJudgeSpecSnapshot(),
	}, []map[string]any{
		{"key": "structured_minutes", "type": "json", "relative_path": "minutes.json", "preview": `{"summary":"Weekly sync","action_items":[{"owner":"Dana"}]}`},
	})

	if section == nil {
		t.Fatal("runPublicTryoutJudges() = nil, want judge section")
	}
	if section["verdict"] != "approved" {
		t.Fatalf("verdict = %v, want approved", section["verdict"])
	}
	if section["model"] != "gpt-5-mini" {
		t.Fatalf("model = %v, want gpt-5-mini", section["model"])
	}
	criteria, ok := section["criteria"].([]map[string]any)
	if !ok || len(criteria) != 2 {
		t.Fatalf("criteria = %#v, want 2 entries", section["criteria"])
	}
	for _, criterion := range criteria {
		if criterion["status"] != "passed" {
			t.Fatalf("criterion %v status = %v, want passed", criterion["key"], criterion["status"])
		}
		if reasoning, _ := criterion["reasoning"].(string); !strings.Contains(reasoning, "Grounded") {
			t.Fatalf("criterion %v reasoning = %v, want judge reasoning", criterion["key"], criterion["reasoning"])
		}
	}
	if len(client.Requests) != 2 {
		t.Fatalf("judge calls = %d, want one per declaration", len(client.Requests))
	}
	for _, request := range client.Requests {
		if request.CredentialReference != "env://OPENAI_API_KEY" {
			t.Fatalf("credential = %q, want platform env key", request.CredentialReference)
		}
	}
	if len(repo.events) == 0 || repo.events[0].EventType != string(runevents.EventTypeScoringStarted) {
		t.Fatalf("events = %#v, want scoring.started first", repo.events)
	}
}

func TestRunPublicTryoutJudgesWithoutJudgesReturnsNil(t *testing.T) {
	activities := &Activities{publicTryoutRepo: &fakePublicTryoutRepo{}, judgeClient: &provider.FakeClient{}}
	section := activities.runPublicTryoutJudges(context.Background(), repository.AgentTryout{
		ID:                     uuid.New(),
		EvaluationSpecSnapshot: json.RawMessage(`{"validators":[]}`),
	}, nil)
	if section != nil {
		t.Fatalf("section = %#v, want nil when no judges configured", section)
	}
}

func TestRunPublicTryoutJudgesDegradesWhenJudgeFails(t *testing.T) {
	repo := &fakePublicTryoutRepo{}
	client := &provider.FakeClient{Err: fmt.Errorf("provider unavailable")}
	activities := &Activities{publicTryoutRepo: repo, judgeClient: client}

	section := activities.runPublicTryoutJudges(context.Background(), repository.AgentTryout{
		ID:                     uuid.New(),
		EvaluationSpecSnapshot: publicJudgeSpecSnapshot(),
	}, []map[string]any{
		{"key": "structured_minutes", "type": "json", "relative_path": "minutes.json", "preview": `{"summary":"Weekly sync"}`},
	})

	if section == nil {
		t.Fatal("runPublicTryoutJudges() = nil, want degraded section")
	}
	if section["verdict"] != "not_judged" {
		t.Fatalf("verdict = %v, want not_judged when every call fails", section["verdict"])
	}
}

func TestRunPublicTryoutJudgesNoOutputsIsNotJudged(t *testing.T) {
	repo := &fakePublicTryoutRepo{}
	activities := &Activities{publicTryoutRepo: repo, judgeClient: &provider.FakeClient{}}

	section := activities.runPublicTryoutJudges(context.Background(), repository.AgentTryout{
		ID:                     uuid.New(),
		EvaluationSpecSnapshot: publicJudgeSpecSnapshot(),
	}, nil)

	if section == nil || section["verdict"] != "not_judged" {
		t.Fatalf("section = %#v, want not_judged verdict", section)
	}
}

func TestNormalizePublicAgentTryoutConfigDefaultsToEnvCredential(t *testing.T) {
	config := NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{})

	if config.HarnessKind != domain.AgentHarnessKindCodexE2B {
		t.Fatalf("harness kind = %q, want codex_e2b", config.HarnessKind)
	}
	if config.E2BTemplateID != "agentclash-tryout-office" {
		t.Fatalf("template = %q, want agentclash-tryout-office", config.E2BTemplateID)
	}
	if config.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", config.Provider)
	}
	if config.CredentialRef != "env://OPENAI_API_KEY" {
		t.Fatalf("credential ref = %q, want env://OPENAI_API_KEY", config.CredentialRef)
	}
}

func TestPublicTryoutScorecardEvaluatesJSONFieldValidators(t *testing.T) {
	spec := json.RawMessage(`{"validators":[{"key":"has_summary","type":"json_field","field":"summary"},{"key":"has_action_items","type":"json_field","field":"action_items"}],"scorecard":{"dimensions":["correctness","latency"]}}`)
	outputs := []map[string]any{
		{"type": "json", "preview": `{"summary":"did the thing","action_items":[]}`},
	}

	card := publicTryoutScorecard(spec, outputs, 1234)

	if card["total_validators"].(int) != 2 {
		t.Fatalf("total = %v, want 2", card["total_validators"])
	}
	// summary is present+non-empty (pass); action_items is an empty array (fail).
	if card["passed_validators"].(int) != 1 {
		t.Fatalf("passed = %v, want 1", card["passed_validators"])
	}
	if card["passed"].(bool) {
		t.Fatal("scorecard should not be fully passed when a validator fails")
	}
	if card["latency_ms"].(int64) != 1234 {
		t.Fatalf("latency = %v, want 1234", card["latency_ms"])
	}
}

func TestPublicTryoutScorecardArtifactProducedValidator(t *testing.T) {
	spec := json.RawMessage(`{"validators":[{"key":"has_presentation","type":"artifact_produced","artifact_key":"presentation"},{"key":"has_pdf","type":"artifact_produced","artifact_key":"presentation_pdf"}],"scorecard":{"dimensions":["correctness"]}}`)
	outputs := []map[string]any{
		{"key": "presentation", "type": "pptx", "size_bytes": 1200, "preview": "UEsDB"},
	}

	card := publicTryoutScorecard(spec, outputs, 500)
	if card["passed_validators"].(int) != 1 {
		t.Fatalf("passed = %v, want 1", card["passed_validators"])
	}
	if card["passed"].(bool) {
		t.Fatal("scorecard should fail when pdf artifact is missing")
	}
}

func TestPublicTryoutTaskPromptIncludesEvalSetup(t *testing.T) {
	prompt := publicTryoutTaskPrompt(repository.AgentTryout{
		TemplateSlug: "slide-deck",
		InputSnapshot: json.RawMessage(`{
			"brief":"Make a partner pitch deck",
			"eval_setup":{
				"unacceptable_mistakes":"invented customer logos",
				"derived_rubric":[{"key":"accuracy","label":"Grounded claims","checks":["no invented logos"]}]
			}
		}`),
		TemplateSnapshot: json.RawMessage(`{"name":"Brief to Slide Deck","description":"Make slides","runtime":{}}`),
	})

	if !strings.Contains(prompt, "Business eval setup") {
		t.Fatalf("prompt = %q, want business eval setup section", prompt)
	}
	if !strings.Contains(prompt, "invented customer logos") {
		t.Fatalf("prompt = %q, want eval failure mode", prompt)
	}
	if !strings.Contains(prompt, "Treat this as the user's acceptance criteria") {
		t.Fatalf("prompt = %q, want acceptance criteria instruction", prompt)
	}
}

func TestPublicTurnCommandResumesSessionAfterFirstTurn(t *testing.T) {
	join := func(parts []string) string { return strings.Join(parts, " ") }

	// Codex: opening turn vs resume.
	first, _ := publicTurnCommand(domain.AgentHarnessKindCodexE2B, "/workspace", "hi", true, "")
	resume, _ := publicTurnCommand(domain.AgentHarnessKindCodexE2B, "/workspace", "next", false, "")
	if strings.Contains(join(first), "resume") {
		t.Fatalf("codex opening turn should not resume: %v", first)
	}
	if !strings.Contains(join(resume), "exec resume --last") {
		t.Fatalf("codex follow-up must resume --last: %v", resume)
	}

	// Claude: --continue only on follow-ups.
	cFirst, _ := publicTurnCommand(domain.AgentHarnessKindClaudeE2B, "/workspace", "hi", true, "")
	cResume, _ := publicTurnCommand(domain.AgentHarnessKindClaudeE2B, "/workspace", "next", false, "")
	if strings.Contains(join(cFirst), "--continue") {
		t.Fatalf("claude opening turn should not continue: %v", cFirst)
	}
	if !strings.Contains(join(cResume), "--continue") {
		t.Fatalf("claude follow-up must --continue: %v", cResume)
	}

	// OpenClaw: same session id across turns.
	oFirst, _ := publicTurnCommand(domain.AgentHarnessKindOpenClawE2B, "/workspace", "hi", true, "")
	oResume, _ := publicTurnCommand(domain.AgentHarnessKindOpenClawE2B, "/workspace", "next", false, "")
	if !strings.Contains(join(oFirst), "session-id agentclash-tryout") || !strings.Contains(join(oResume), "session-id agentclash-tryout") {
		t.Fatalf("openclaw turns must reuse session id: %v / %v", oFirst, oResume)
	}
	if !strings.Contains(join(oFirst), "onboard") || strings.Contains(join(oResume), "onboard") {
		t.Fatalf("openclaw onboard should run only on opening turn: %v / %v", oFirst, oResume)
	}
}

func TestPublicTurnCommandAppliesSelectedModel(t *testing.T) {
	codex, _ := publicTurnCommand(domain.AgentHarnessKindCodexE2B, "/workspace", "hi", true, "gpt-5")
	if !strings.Contains(strings.Join(codex, " "), "--model gpt-5") {
		t.Fatalf("codex command missing selected model: %v", codex)
	}

	claude, _ := publicTurnCommand(domain.AgentHarnessKindClaudeE2B, "/workspace", "hi", true, "claude-sonnet-4-5")
	if !strings.Contains(strings.Join(claude, " "), "--model claude-sonnet-4-5") {
		t.Fatalf("claude command missing selected model: %v", claude)
	}

	openclaw, _ := publicTurnCommand(domain.AgentHarnessKindOpenClawE2B, "/workspace", "hi", true, "google/gemini-2.5-pro")
	if !strings.Contains(strings.Join(openclaw, " "), "AGENTCLASH_SELECTED_MODEL") ||
		!strings.Contains(strings.Join(openclaw, " "), "--model") {
		t.Fatalf("openclaw command missing selected model routing: %v", openclaw)
	}
}

func TestPublicTryoutRunningSummaryExposesOutputsWhileActive(t *testing.T) {
	summary := publicTryoutRunningSummary(
		[]map[string]any{{
			"key":           "presentation",
			"type":          "pptx",
			"relative_path": "deck.pptx",
			"preview":       "UEsDB",
			"encoding":      "base64",
		}},
		domain.AgentHarnessKindCodexE2B,
		"gpt-5",
	)

	var decoded map[string]any
	if err := json.Unmarshal(summary, &decoded); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if decoded["code"] != "outputs_ready" {
		t.Fatalf("code = %v, want outputs_ready", decoded["code"])
	}
	if decoded["selected_model"] != "gpt-5" {
		t.Fatalf("selected_model = %v, want gpt-5", decoded["selected_model"])
	}
	outputs, ok := decoded["outputs"].([]any)
	if !ok || len(outputs) != 1 {
		t.Fatalf("outputs = %#v, want one output", decoded["outputs"])
	}
}

func TestPublicTryoutRunnerEnvUsesHostedCredential(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-hosted")
	config := NormalizePublicAgentTryoutConfig(PublicAgentTryoutConfig{})
	credential, err := provider.EnvCredentialResolver{}.Resolve(t.Context(), config.CredentialRef)
	if err != nil {
		t.Fatalf("resolve hosted credential: %v", err)
	}
	harnessKind := publicTryoutHarnessKind(config, nil)
	harness := publicTryoutHarnessSnapshot(config, repository.AgentTryout{
		TemplateSlug:           "meeting-minutes",
		InputSnapshot:          json.RawMessage(`{"notes":"hello"}`),
		TemplateSnapshot:       json.RawMessage(`{"name":"Meeting minutes","description":"Summarize","runtime":{}}`),
		ToolPolicySnapshot:     json.RawMessage(`{"tools":[]}`),
		EvaluationSpecSnapshot: json.RawMessage(`{}`),
	}, harnessKind, config.CredentialRef)

	env := publicTryoutRunnerEnv(harnessKind, harness, credential)
	if env["OPENAI_API_KEY"] != "sk-hosted" || env["CODEX_API_KEY"] != "sk-hosted" {
		t.Fatalf("runner env did not use hosted key: %#v", env)
	}
	if _, ok := os.LookupEnv("AGENT_TRYOUT_PUBLIC_WORKSPACE_ID"); ok {
		t.Fatal("test must not depend on AGENT_TRYOUT_PUBLIC_WORKSPACE_ID")
	}
}
