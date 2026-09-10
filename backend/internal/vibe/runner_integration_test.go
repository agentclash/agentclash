package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

type repairCompiler struct{}

func (repairCompiler) Draft(a DraftProposal, _ Limits) (json.RawMessage, error) { return raw(a), nil }

func (repairCompiler) Instructions() string {
	return "Create three cases grounded in the supplied policy."
}
func (repairCompiler) Compile(json.RawMessage, string, uuid.UUID, Limits) (Compiled, error) {
	return Compiled{}, nil
}

func TestIntegrationVibeFreeAuthoringRepair(t *testing.T) {
	for _, mode := range []string{"first_valid", "corrected", "still_invalid", "question"} {
		t.Run(mode, func(t *testing.T) {
			s := integrationStore(t)
			v := anonSession(t, s)
			cfg := freeConfig()
			profile := cfg.Profiles[cfg.DefaultModel]
			profile.StructuredOutputs = true
			cfg.Profiles[cfg.DefaultModel] = profile
			gate := testGate(t)
			ctx := context.Background()
			proposal := DraftProposal{Title: "Refund support", AgentPrompt: "Allow refunds within 30 days. Ask for a missing date.", Examples: []string{"Refund at 10 days?", "Refund at 45 days?", "Can I get a refund?"}, SuccessCriteria: "Refund within 30 days; decline late requests; ask for missing dates."}
			proposal.AgentPrompt = PreviewPrompt(proposal.AgentPrompt)
			proposal.SuccessCriteria = PreviewCriteria(proposal.SuccessCriteria)
			blueprint := raw(proposal)
			var authored map[string]any
			_ = json.Unmarshal(raw(proposal), &authored)
			authored["kind"] = "agent_draft"
			delete(authored, "examples")
			authored["positive_example"], authored["negative_example"], authored["insufficient_example"] = proposal.Examples[0], proposal.Examples[1], proposal.Examples[2]
			valid := string(raw(map[string]any{"reply_kind": "design", "journey": JourneyProposal{Mode: "idea"}, "reply": "Review the three examples.", "requirement_changes": []RequirementChange{{Action: "add", Statement: "Refund within 30 days."}}, "assumptions": []string{"Use a friendly tone."}, "criteria_requirement_ids": []string{}, "artifact": authored}))
			// Like the live failure: a model echoes a large prompt metadata field.
			// Put it last so partial decoding has already populated the draft.
			invalid := strings.TrimSuffix(valid, "}") + `,"prompt_metadata":"` + strings.Repeat("metadata", 1500) + `"}`
			calls := 0
			fake := callFunc(func(_ context.Context, request provider.Request) (provider.Response, error) {
				calls++
				if string(request.ResponseFormat) != string(strictAuthoringV3Format) {
					t.Fatal("verified schema support did not reach the provider request")
				}
				if _, err := CountContext(request, cfg.Profiles[cfg.DefaultModel], LimitsFor(true)); err != nil {
					t.Fatal("oversized request reached provider", err)
				}
				output := invalid
				if mode == "first_valid" {
					output = valid
				}
				if calls > 1 {
					if strings.Contains(request.Messages[len(request.Messages)-1].Content, "invalid_response") || !strings.Contains(request.Messages[1].Content, "within 30 days") {
						t.Fatal("repair did not retain original intent and omit oversized invalid output")
					}
					switch mode {
					case "corrected":
						output = valid
					case "question":
						output = `{"reply_kind":"design","journey":{"mode":"idea","stack":"","evidence":""},"requirement_changes":[],"reply":"What should happen if the date is missing?","assumptions":[],"criteria_requirement_ids":[],"artifact":null}`
					}
				}
				zero := json.Number("0")
				return provider.Response{OutputText: output, Usage: provider.Usage{InputTokens: 100, OutputTokens: 200, CostUSD: &zero}}, nil
			})
			svc := &Service{Store: s, Config: cfg, Gate: gate, Compiler: repairCompiler{}}
			runner := Runner{Service: svc, Gateway: &Gateway{Store: s, Config: cfg, Gate: gate, Client: fake}}
			o, err := svc.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Models: cfg.DefaultModels(), Kind: "message", Content: "Build a support agent: refunds within 30 days, decline late requests, ask for missing dates. Create three cases."})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = s.Finish(ctx, o.ID, &Fault{Code: "test_cleanup", Message: "Test ended."})
				_, _ = s.DB.Exec(ctx, "DELETE FROM vibe_attempts WHERE operation_id=$1", o.ID)
			})
			err = runner.Execute(ctx, o.ID)
			if mode == "still_invalid" {
				var f *Fault
				if !errors.As(err, &f) || f.Code != "invalid_draft" {
					t.Fatalf("unexpected correction failure: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			wantCalls := 2
			if mode == "first_valid" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("correction escaped its two-call bound: %d", calls)
			}
			if err = s.Finish(ctx, o.ID, issueFrom(err)); err != nil {
				t.Fatal(err)
			}
			current, err := s.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "corrected" || mode == "first_valid" {
				if len(current.Document.Artifacts) != 1 {
					t.Fatal("correction did not produce exactly one draft")
				}
				var got, want any
				if json.Unmarshal(current.Document.Artifacts[0].Blueprint, &got) != nil || json.Unmarshal(blueprint, &want) != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("correction lost requested coverage")
				}
				if len(current.Document.Requirements) != 2 {
					t.Fatal("lost requirement or assumption")
				}
				for _, requirement := range current.Document.Requirements {
					if requirement.Status != "proposed" || requirement.ProposedBy != "assistant" || requirement.SourceMessageID == uuid.Nil || requirement.ProposalMessageID == nil || requirement.AcceptedBy != "" || requirement.AcceptedAt != nil {
						t.Fatalf("proposal gained false provenance: %+v", requirement)
					}
				}
				last := current.Document.Messages[len(current.Document.Messages)-1].Content
				if !strings.Contains(last, "Assumptions to review:") || current.Document.Requirements[1].Statement != "Assumption: Use a friendly tone." {
					t.Fatal("assumption lost its visible proposed label")
				}
			} else if len(current.Document.Artifacts) != 0 {
				t.Fatal("invalid first-response draft leaked into the final document")
			}
			finished, err := s.Operation(ctx, o.ID)
			if err != nil || finished.ActualCost == nil || *finished.ActualCost != 0 || finished.Billing != Released {
				t.Fatalf("zero-cost accounting was lost: %+v %v", finished, err)
			}
		})
	}
}
