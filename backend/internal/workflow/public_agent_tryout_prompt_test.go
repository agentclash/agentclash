package workflow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

// TestPublicTryoutTaskPromptForegroundsDeliverables verifies the public-path
// prompt builder produces the strong, structured task prompt (instructions,
// deliverable paths, toolchain, quality bar) and still carries the user input.
func TestPublicTryoutTaskPromptForegroundsDeliverables(t *testing.T) {
	tryout := repository.AgentTryout{
		TemplateSlug: "slide-deck",
		TemplateSnapshot: json.RawMessage(`{"name":"Brief to Slide Deck","description":"Turn a brief into a deck.",` +
			`"runtime":{"instructions":"Build deck.pptx with python-pptx and export deck.pdf.",` +
			`"expected_artifacts":[{"key":"presentation","type":"pptx","path":"deck.pptx"},{"key":"presentation_pdf","type":"pdf","path":"deck.pdf"}]}}`),
		InputSnapshot: json.RawMessage(`{"brief":"Q3 results"}`),
	}

	prompt := publicTryoutTaskPrompt(tryout)

	for _, want := range []string{
		"DELIVERABLES",
		"deck.pptx (pptx)",
		"deck.pdf (pdf)",
		"INSTRUCTIONS:",
		"Build deck.pptx with python-pptx",
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
