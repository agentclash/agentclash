package api

import (
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"testing"
)

func TestVibeEvaluationEditsPreserveImportedCoverage(t *testing.T) {
	p := vibe.DraftProposal{Title: "Evidence", AgentPrompt: "Use sources", Examples: []string{"No sources"}, SuccessCriteria: "Allow no qualifying recommendation"}
	blueprint, err := (VibePackCompiler{}).Draft(p, vibe.LimitsFor(true))
	if err != nil {
		t.Fatal(err)
	}
	if !canEditVibeEvaluation(blueprint, vibe.LimitsFor(true)) {
		t.Fatal("generated preview not editable")
	}
	var b map[string]any
	_ = json.Unmarshal(blueprint, &b)
	b["validators"] = append(b["validators"].([]any), map[string]any{"key": "extra", "type": "regex_match", "target": "final_output", "expected_from": "literal:required"})
	changed, _ := json.Marshal(b)
	if canEditVibeEvaluation(changed, vibe.LimitsFor(true)) {
		t.Fatal("semantic edit would silently drop imported coverage")
	}
}
