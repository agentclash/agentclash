package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func TestVibeCompilerTestSuiteUsesActualCaseCapacity(t *testing.T) {
	for _, count := range []int{1, 3, 5, 9, 20} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			proposal := vibe.DraftProposal{TestsOnly: true, Title: "Support routing", Summary: "Route requests to the correct team.", SuccessCriteria: "Send billing requests to the billing team."}
			for i := 0; i < count; i++ {
				proposal.Scenarios = append(proposal.Scenarios, vibe.TestScenario{Input: fmt.Sprintf("Billing question %d", i+1), Expected: "Route this request to billing."})
			}
			l := vibe.LimitsFor(false)
			blueprint, err := (VibePackCompiler{}).Draft(proposal, l)
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := (VibePackCompiler{}).Compile(blueprint, "deepseek/deepseek-v3.2", uuid.New(), l)
			if err != nil {
				t.Fatal(err)
			}
			if len(compiled.Cases) != count {
				t.Fatalf("compiled %d of %d cases", len(compiled.Cases), count)
			}
		})
	}
}

func TestVibeCompilerCountLimitDoesNotTruncateOrExpandLegacyPreview(t *testing.T) {
	proposal := vibe.DraftProposal{TestsOnly: true, Title: "Five tests", AgentPrompt: "Answer requests.", SuccessCriteria: "Answer the request."}
	for i := 0; i < 5; i++ {
		proposal.Scenarios = append(proposal.Scenarios, vibe.TestScenario{Input: fmt.Sprintf("Request %d", i), Expected: "Answer the request."})
	}
	if _, err := (VibePackCompiler{}).Draft(proposal, vibe.LimitsFor(true)); err == nil || !strings.Contains(err.Error(), "no examples were removed") {
		t.Fatalf("anonymous over-capacity suite was not rejected without truncation: %v", err)
	}
	proposal.TestsOnly = false
	if _, err := (VibePackCompiler{}).Draft(proposal, vibe.LimitsFor(false)); err == nil {
		t.Fatal("legacy conversational preview silently expanded beyond three examples")
	}
}
