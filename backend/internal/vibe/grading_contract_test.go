package vibe

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVibeGradingIdentity(t *testing.T) {
	cfg := testConfig()
	cfg.GroundedJudging = true
	svc := Service{Config: cfg}
	evidence := EvidenceSet{Conversations: []EvidenceConversation{{Messages: []EvidenceMessage{{ID: "u", Role: "user", Content: " Refund?\r\n"}, {ID: "a", Role: "assistant", Content: "No."}}}}}
	artifact := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{Expectations: []Expectation{{ID: "policy", Statement: "No refunds."}}}}
	plan := Plan{Submission: Submission{Models: cfg.DefaultModels()}, Artifact: &artifact, Evidence: &evidence}
	if err := svc.freezeGrading(&plan); err != nil {
		t.Fatal(err)
	}
	before := *plan.Grading
	profile := cfg.Profiles[plan.Submission.Models.Evaluator]
	profile.ExpiresAt = profile.ExpiresAt.Add(time.Hour)
	profile.Name = "Renewed"
	profile.InputNanoPerToken++
	cfg.Profiles[profile.ID] = profile
	if err := svc.freezeGrading(&plan); err != nil || before.Hash != plan.Grading.Hash {
		t.Fatal("metadata renewal broke comparison", err)
	}
	if err := validateGradingDispatch(plan, Evaluator, profile); err != nil {
		t.Fatal(err)
	}
	evidence.Conversations[0].Messages[1].Content = "Yes."
	evidence.Conversations[0].Messages[0].Content = "Refund?"
	if err := svc.freezeGrading(&plan); err != nil || before.Hash != plan.Grading.Hash {
		t.Fatal("replies or harmless newline changes broke comparison", err)
	}
	for _, field := range []string{"route", "model", "reasoning", "output", "parser", "normalization", "schema", "prompt", "criteria", "aggregation"} {
		t.Run(field, func(t *testing.T) {
			p := plan
			g := *plan.Grading
			p.Grading = &g
			changed := profile
			switch field {
			case "route":
				changed.Route = "different"
			case "model":
				changed.ID = "different"
			case "reasoning":
				changed.DisableReasoning = !changed.DisableReasoning
			case "output":
				g.Evaluator.MaxOutput++
			case "parser":
				g.Parser += "2"
			case "normalization":
				g.Normalization += "2"
			case "schema":
				g.Schema += "2"
			case "prompt":
				g.PromptHash = "other"
			case "criteria":
				g.CriteriaHash = "other"
			case "aggregation":
				g.Aggregation += "2"
			}
			if validateGradingDispatch(p, Evaluator, changed) == nil {
				t.Fatal("changed contract accepted")
			}
		})
	}
	artifact.ConversationEvaluation.Expectations[0].Statement = "Allow refunds."
	if err := svc.freezeGrading(&plan); err != nil || before.Hash == plan.Grading.Hash {
		t.Fatal("changed rules retained identity", err)
	}
	if err := validateGradingDispatch(Plan{}, Evaluator, ModelProfile{}); err != nil {
		t.Fatal("historical contract rejected")
	}
}

func TestVibeGradingRejectsFutureParser(t *testing.T) {
	cfg := testConfig()
	cfg.GroundedJudging = true
	svc := Service{Config: cfg}
	p := Plan{Submission: Submission{Models: cfg.DefaultModels()}, Artifact: &Artifact{ConversationEvaluation: &ConversationEvaluation{}}, Evidence: &EvidenceSet{}}
	if err := svc.freezeGrading(&p); err != nil {
		t.Fatal(err)
	}
	p.Grading.Parser = "grounded-judge-v2"
	p.Grading.Hash = p.Grading.fingerprint()
	if gradingSupported(p.Grading) {
		t.Fatal("future parser silently executed by old worker")
	}
}
