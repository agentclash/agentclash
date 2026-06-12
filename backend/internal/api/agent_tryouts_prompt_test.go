package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAgentTryoutTaskPromptForegroundsDeliverablesAndToolchain verifies the
// rewritten prompt builder surfaces the template instructions, the exact
// deliverable file paths, and the installed toolchain — the signals the agent
// needs to produce real office files instead of a markdown outline.
func TestAgentTryoutTaskPromptForegroundsDeliverablesAndToolchain(t *testing.T) {
	template := AgentTryoutTemplate{
		Slug:        "slide-deck",
		Name:        "Brief to Slide Deck",
		Description: "Turn a brief into a deck.",
		Runtime:     json.RawMessage(`{"instructions":"Build deck.pptx and export deck.pdf.","expected_artifacts":[{"key":"presentation","type":"pptx","path":"deck.pptx"},{"key":"presentation_pdf","type":"pdf","path":"deck.pdf"}]}`),
	}

	prompt := agentTryoutTaskPrompt(template, json.RawMessage(`{"brief":"Q3 results"}`))

	for _, want := range []string{
		"DELIVERABLES",
		"deck.pptx (pptx)",
		"deck.pdf (pdf)",
		"INSTRUCTIONS:",
		"Build deck.pptx and export deck.pdf.",
		"TOOLCHAIN",
		"python-pptx",
		"soffice",
		"QUALITY BAR",
		"Q3 results",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n---\n%s", want, prompt)
		}
	}
}

// TestEveryAvailableAgentTryoutTemplateHasInstructions guards the "capable
// artifacts" contract: each available office template must give the agent
// concrete instructions and an expected-artifacts manifest so the prompt builder
// can foreground real deliverables.
func TestEveryAvailableAgentTryoutTemplateHasInstructions(t *testing.T) {
	for _, template := range builtinAgentTryoutTemplates() {
		if !template.Available {
			continue
		}
		if strings.TrimSpace(agentTryoutRuntimeInstructions(template.Runtime)) == "" {
			t.Errorf("available template %q has no runtime instructions", template.Slug)
		}
		if len(expectedArtifactsFromRuntime(template.Runtime)) == 0 {
			t.Errorf("available template %q declares no expected artifacts", template.Slug)
		}
		if !json.Valid(template.Runtime) {
			t.Errorf("template %q runtime is not valid JSON: %s", template.Slug, template.Runtime)
		}
	}
}
