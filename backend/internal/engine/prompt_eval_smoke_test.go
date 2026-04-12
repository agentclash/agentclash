//go:build promptevalsmoke

package engine

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type smokeTestCase struct {
	name           string
	instructions   string
	outputContains string
	checkJSON      bool
}

var smokePrompts = []smokeTestCase{
	{
		name:           "Translation",
		instructions:   "Translate 'hello world' to French. Reply with only the translation, nothing else.",
		outputContains: "bonjour",
	},
	{
		name:           "Factual",
		instructions:   "What is the capital of Japan? Reply with only the city name, nothing else.",
		outputContains: "Tokyo",
	},
	{
		name:         "JSON",
		instructions: `Return a JSON object with keys "name" and "age". Name is "Alice", age is 30. Reply with only valid JSON, no markdown fences.`,
		checkJSON:    true,
	},
}

func TestPromptEvalSmokeGemini(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}
	t.Setenv("GEMINI_API_KEY", apiKey)

	client := provider.NewGeminiClient(nil, "", provider.EnvCredentialResolver{})

	for _, tc := range smokePrompts {
		t.Run(tc.name, func(t *testing.T) {
			runSmokeTest(t, client, "gemini", "env://GEMINI_API_KEY", "gemini-2.0-flash", tc)
		})
	}
}

func TestPromptEvalSmokeAnthropic(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY is not set")
	}
	t.Setenv("ANTHROPIC_API_KEY", apiKey)

	client := provider.NewAnthropicClient(nil, "", "", provider.EnvCredentialResolver{})

	for _, tc := range smokePrompts {
		t.Run(tc.name, func(t *testing.T) {
			runSmokeTest(t, client, "anthropic", "env://ANTHROPIC_API_KEY", "claude-sonnet-4-20250514", tc)
		})
	}
}

func runSmokeTest(t *testing.T, client provider.Client, providerKey, credRef, model string, tc smokeTestCase) {
	t.Helper()

	observer := &recordingObserver{}
	executor := NewPromptEvalExecutor(client, observer)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	execCtx := smokeExecutionContext(providerKey, credRef, model, tc.instructions)
	result, err := executor.Execute(ctx, execCtx)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	t.Logf("Provider: %s | Model: %s", providerKey, model)
	t.Logf("Output: %q", result.FinalOutput)
	t.Logf("Tokens: in=%d out=%d total=%d", result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.TotalTokens)

	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
	if result.StepCount != 1 {
		t.Fatalf("step count = %d, want 1", result.StepCount)
	}
	if result.ToolCallCount != 0 {
		t.Fatalf("tool call count = %d, want 0", result.ToolCallCount)
	}
	if result.FinalOutput == "" {
		t.Fatalf("final output is empty")
	}
	if result.Usage.TotalTokens <= 0 {
		t.Fatalf("total tokens = %d, want > 0", result.Usage.TotalTokens)
	}

	if tc.outputContains != "" {
		if !strings.Contains(strings.ToLower(result.FinalOutput), strings.ToLower(tc.outputContains)) {
			t.Fatalf("output %q does not contain expected %q (case-insensitive)", result.FinalOutput, tc.outputContains)
		}
	}
	if tc.checkJSON {
		trimmed := strings.TrimSpace(result.FinalOutput)
		if !json.Valid([]byte(trimmed)) {
			t.Fatalf("output is not valid JSON: %q", trimmed)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			t.Fatalf("output is not a JSON object: %v", err)
		}
		if _, ok := parsed["name"]; !ok {
			t.Fatalf("JSON output missing 'name' key: %s", trimmed)
		}
		if _, ok := parsed["age"]; !ok {
			t.Fatalf("JSON output missing 'age' key: %s", trimmed)
		}
	}

	if !observer.runComplete {
		t.Fatalf("observer did not receive OnRunComplete")
	}
	if observer.runFailure {
		t.Fatalf("observer received unexpected OnRunFailure")
	}
	if observer.providerCalls != 1 {
		t.Fatalf("observer provider calls = %d, want 1", observer.providerCalls)
	}
	if observer.providerResponses != 1 {
		t.Fatalf("observer provider responses = %d, want 1", observer.providerResponses)
	}
}

func smokeExecutionContext(providerKey, credRef, model, instructions string) repository.RunAgentExecutionContext {
	runID := uuid.New()
	runAgentID := uuid.New()
	return repository.RunAgentExecutionContext{
		Run:      domain.Run{ID: runID},
		RunAgent: domain.RunAgent{ID: runAgentID, RunID: runID, Status: domain.RunAgentStatusQueued, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: []byte(`{"version":{"execution_mode":"prompt_eval"}}`),
			Challenges: []repository.ChallengeDefinitionExecutionContext{
				{
					ID:           uuid.New(),
					ChallengeKey: "smoke-test",
					Title:        "Smoke Test",
					Definition:   mustJSON(map[string]string{"instructions": instructions}),
				},
			},
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			DeploymentType: "native",
			SnapshotConfig: []byte(`{}`),
			AgentBuildVersion: repository.AgentBuildVersionExecutionContext{
				ID:         uuid.New(),
				AgentKind:  "llm_agent",
				AgentSpec:  []byte(`{}`),
				PolicySpec: []byte(`{}`),
			},
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget:    "prompt_eval",
				TraceMode:          "preferred",
				StepTimeoutSeconds: 45,
				RunTimeoutSeconds:  55,
				ProfileConfig:      []byte(`{}`),
			},
			ProviderAccount: &repository.ProviderAccountExecutionContext{
				ID:                  uuid.New(),
				ProviderKey:         providerKey,
				CredentialReference: credRef,
			},
			ModelAlias: &repository.ModelAliasExecutionContext{
				ModelCatalogEntry: repository.ModelCatalogEntryExecutionContext{
					ProviderModelID: model,
				},
			},
		},
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
