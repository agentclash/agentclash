package api

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestVibeReconciledSubsetQuotesAndDispatchesOnlySelectedCases(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		t.Run(map[bool]string{false: "compiled", true: "recorded"}[recorded], func(t *testing.T) {
			h := newVibeAPIHarness(t)
			h.svc.Config.GroundedJudging = true
			v, err := h.svc.Store.CreateAgent(h.ctx, h.actor, nil, uuid.New(), "test", h.svc.Config.DefaultModels())
			if err != nil {
				t.Fatal(err)
			}
			h.session = v
			if recorded {
				client := uuid.New()
				err = h.svc.AddEvidence(h.ctx, h.actor, v.ID, vibe.EvidenceInput{ClientID: &client, Revision: v.Revision, Content: `{"conversations":[{"messages":[{"role":"user","content":"First question"},{"role":"assistant","content":"First answer"}]},{"messages":[{"role":"user","content":"Second question"},{"role":"assistant","content":"Second answer"}]},{"messages":[{"role":"user","content":"Third question"},{"role":"assistant","content":"Third answer"}]}]}`, Expectations: []vibe.Expectation{{ID: "rule", Statement: "Answer each question."}}})
			} else {
				err = h.svc.Import(h.ctx, h.actor, v.ID, v.Revision, mustJSON(t, map[string]any{"format": "agentclash-vibe-v1", "agent_prompt": "Answer the customer.", "evaluation": json.RawMessage(vibeBlueprint)}))
			}
			if err != nil {
				t.Fatal(err)
			}
			h.reload()
			a := h.session.Document.Artifacts[0]
			sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true, Models: h.svc.Config.DefaultModels()}
			q, err := h.svc.QuoteRun(h.ctx, h.actor, v.ID, sub)
			if err != nil {
				t.Fatal(err)
			}
			sub.RunQuoteID = &q.ID
			source, err := h.svc.Prepare(h.ctx, h.actor, v.ID, sub)
			if err != nil {
				t.Fatal(err)
			}
			if err = h.svc.Store.Stop(h.ctx, h.actor, source.ID); err != nil {
				t.Fatal(err)
			}
			h.reload()
			source = h.session.Operations[len(h.session.Operations)-1]
			key := source.Results[1].CaseKey
			var retry vibe.Submission
			body := map[string]any{"client_id": uuid.New(), "revision": h.session.Revision, "kind": "check", "artifact_id": a.ID, "approve_artifact": true, "models": h.svc.Config.DefaultModels(), "case_keys": []string{key}, "retry_source_result_ref": map[string]any{"kind": "vibe_operation", "id": source.ID}}
			if err = json.Unmarshal(mustJSON(t, body), &retry); err != nil {
				t.Fatal(err)
			}
			quote, err := h.svc.QuoteRun(h.ctx, h.actor, v.ID, retry)
			if err != nil {
				t.Fatal(err)
			}
			if quote.Cases != 1 || quote.Calls != q.Calls/3 {
				t.Fatal("subset quote priced full execution", quote, q)
			}
			retry.RunQuoteID = &quote.ID
			op, err := h.svc.Prepare(h.ctx, h.actor, v.ID, retry)
			if err != nil {
				t.Fatal(err)
			}
			if err = h.svc.Store.PutResult(h.ctx, op.ID, vibe.CaseResult{CaseKey: source.Results[0].CaseKey, Version: a.ID.String(), Verdict: vibe.Pass}); err == nil {
				t.Fatal("out-of-subset result accepted")
			}
			calls := 0
			h.runner.Gateway.Client = groundingClient(func(_ context.Context, r provider.Request) (provider.Response, error) {
				calls++
				zero := json.Number("0")
				text := "An exact answer."
				if strings.Contains(r.Messages[0].Content, "evaluat") || strings.Contains(r.Messages[0].Content, "judge") {
					text = `{"pass":true,"reasoning":"Fits the rule."}`
				}
				if recorded {
					text = `{"checks":[{"key":"rule","verdict":"UNKNOWN","evidence":"Needs human review."}]}`
				}
				return provider.Response{OutputText: text, ProviderModelID: r.Model, Usage: provider.Usage{CostUSD: &zero}}, nil
			})
			result, err := h.executeOperation(op.ID)
			if err != nil {
				t.Fatal(err)
			}
			if calls != quote.Calls || len(result.Results) != 1 || result.Results[0].CaseKey != key || result.Scorecard.Total != 1 {
				t.Fatal("runner escaped admitted subset", calls, result)
			}
			var count int
			if err = h.db.QueryRow(h.ctx, "SELECT count(*) FROM vibe_case_results WHERE operation_id=$1", source.ID).Scan(&count); err != nil || count != 3 {
				t.Fatal("original evidence changed", count, err)
			}

			h.reload()
			regradeSub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "retest", Purpose: "regrade", ArtifactID: &a.ID, BaselineID: &op.ID, Models: h.svc.Config.DefaultModels()}
			regradeQuote, err := h.svc.QuoteRun(h.ctx, h.actor, v.ID, regradeSub)
			if err != nil {
				t.Fatal(err)
			}
			if regradeQuote.Cases != 1 || regradeQuote.Calls != 1 {
				t.Fatal("regrade escaped original selected scope", regradeQuote)
			}
			regradeSub.RunQuoteID = &regradeQuote.ID
			regrade, err := h.svc.Prepare(h.ctx, h.actor, v.ID, regradeSub)
			if err != nil {
				t.Fatal(err)
			}
			beforeCalls := calls
			graded, err := h.executeOperation(regrade.ID)
			if err != nil {
				t.Fatal(err)
			}
			if calls-beforeCalls != 1 || len(graded.Results) != 1 || graded.Results[0].CaseKey != key || graded.EvidenceMetadata.Origin != vibe.OriginRegraded || graded.EvidenceMetadata.SourceResultRef.ID != op.ID {
				t.Fatal("regrade widened or lost immutable source", graded, calls-beforeCalls)
			}
		})
	}
}
