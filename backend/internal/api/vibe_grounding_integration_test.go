package api

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/google/uuid"
)

type groundingClient func(context.Context, provider.Request) (provider.Response, error)

func (f groundingClient) InvokeModel(c context.Context, r provider.Request) (provider.Response, error) {
	return f(c, r)
}

func TestVibeGroundedResultsAndRegrade(t *testing.T) {
	for _, mode := range []string{"valid", "invented_quote", "provider_error", "truncated_target", "wrong_model"} {
		t.Run(mode, func(t *testing.T) {
			h := newReliabilityHarness(t, 3)
			h.svc.Config.GroundedJudging = true
			h.svc.Config.FreeOnly = false
			h.svc.Config.LocalBudget = 100 * vibe.NanoUSD
			h.svc.Config.DefaultModel = "deepseek/deepseek-v4-flash-0731"
			modelID := h.svc.Config.DefaultModel
			h.svc.Config.Profiles[modelID] = vibe.ModelProfile{ID: modelID, Route: "deepinfra/fp8", Conformed: true, StructuredOutputs: true, Context: 65536, FramingAllowance: 4096, InputNanoPerToken: 400, OutputNanoPerToken: 1600, ExpiresAt: time.Now().Add(time.Hour)}
			h.runner.Gateway.Config = h.svc.Config
			imported := mustJSON(t, map[string]any{"format": "agentclash-vibe-v1", "agent_prompt": "Explain refund eligibility within 30 days.", "evaluation": json.RawMessage(vibeBlueprint)})
			if err := h.svc.Import(h.ctx, h.actor, h.session.ID, h.session.Revision, imported); err != nil {
				t.Fatal(err)
			}
			h.reload()
			a := h.session.Document.Artifacts[0]
			targetCalls, judgeCalls := 0, 0
			regrading := false
			h.runner.Gateway.Client = groundingClient(func(_ context.Context, r provider.Request) (provider.Response, error) {
				zero := json.Number("0")
				response := provider.Response{Usage: provider.Usage{InputTokens: 100, OutputTokens: 100, CostUSD: &zero}, ProviderModelID: r.Model}
				if strings.Contains(r.Messages[0].Content, "Every finding has exactly") {
					judgeCalls++
					var data struct {
						Criteria scoring.LLMJudgeDeclaration `json:"criteria"`
						Replies  []vibe.EvidenceMessage      `json:"replies"`
					}
					if err := json.Unmarshal([]byte(r.Messages[1].Content), &data); err != nil || len(data.Replies) != 1 {
						t.Fatal("missing exact reply", err)
					}
					quote := data.Replies[0].Content
					if !regrading && mode == "provider_error" {
						return provider.Response{}, &vibe.Fault{Code: "rate_limited", Message: "Fixture provider failure"}
					}
					if !regrading && mode == "invented_quote" {
						quote = "Never actually said this."
					}
					response.OutputText = string(mustJSON(t, map[string]any{"key": data.Criteria.Key, "pass": regrading, "reasoning": "A scripted grade for the recorded reply.", "finding": map[string]any{"kind": "observed", "quotes": []any{map[string]any{"message_id": "output", "text": quote}}, "missing": "", "covered_message_ids": []string{}}}))
				} else {
					if regrading {
						t.Fatal("regrading generated a new answer")
					}
					targetCalls++
					response.OutputText = "A refund is possible within 30 days."
					if mode == "wrong_model" {
						response.ProviderModelID = "unapproved-model"
					}
					if mode == "truncated_target" {
						response.FinishReason = provider.FinishReasonMaxTokens
					}
				}
				return response, nil
			})
			prepare := func(kind, purpose string, baseline *uuid.UUID) (vibe.Operation, error) {
				return h.svc.Prepare(h.ctx, h.actor, h.session.ID, vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: kind, Purpose: purpose, ArtifactID: &a.ID, BaselineID: baseline, ApproveArtifact: true, Models: h.svc.Config.DefaultModels(), TestJourney: true})
			}
			admitted, err := prepare("check", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			original, runErr := h.executeOperation(admitted.ID)
			if original.Grading == nil || original.TargetConfig == nil {
				t.Fatal("configuration not persisted")
			}
			if mode == "provider_error" || mode == "truncated_target" || mode == "wrong_model" {
				if runErr == nil || original.Scorecard.Unknown != 3 || original.Scorecard.Passed != 0 {
					t.Fatal("provider error became a grade", original, runErr)
				}
			} else if runErr != nil {
				t.Fatal(runErr)
			}
			if mode == "invented_quote" && (original.Scorecard.Unknown != 3 || original.Scorecard.Failed != 0) {
				t.Fatal("fabricated evidence scored", original.Scorecard)
			}
			if mode == "valid" && original.Scorecard.Failed != 3 {
				t.Fatal("original failure missing", original.Scorecard)
			}
			originalCases := []vibe.CaseResult{}
			for _, summary := range original.Results {
				r, e := h.svc.Store.GetCase(h.ctx, h.actor, original.ID, summary.CaseKey)
				if e != nil {
					t.Fatal(e)
				}
				originalCases = append(originalCases, r)
			}
			beforeArtifacts := mustJSON(t, h.session.Document.Artifacts)
			if mode == "provider_error" || mode == "truncated_target" || mode == "wrong_model" {
				_, err = prepare("retest", "regrade", &original.ID)
				if err == nil {
					t.Fatal("incomplete target was regraded")
				}
				return
			}
			// An expiry/price-label renewal is not a new grader; changing its route is.
			model := h.svc.Config.DefaultModels().Evaluator
			profile := h.svc.Config.Profiles[model]
			profile.ExpiresAt = profile.ExpiresAt.Add(time.Hour)
			profile.Name = "Renewed metadata"
			h.svc.Config.Profiles[model] = profile
			renewal, err := prepare("retest", "", &original.ID)
			if err != nil {
				t.Fatal("metadata refresh rejected", err)
			}
			if renewal.Grading.Hash != original.Grading.Hash {
				t.Fatal("renewal changed grading identity")
			}
			if _, err = h.executeOperation(renewal.ID); err != nil {
				t.Fatal(err)
			}
			profile.Route = "open-inference/fp8"
			h.svc.Config.Profiles[model] = profile
			if _, err = prepare("retest", "", &original.ID); err == nil {
				t.Fatal("changed provider compared as same grader")
			}
			var fault *vibe.Fault
			if !errors.As(err, &fault) || fault.Code != "comparison_changed" {
				t.Fatal(err)
			}
			regrading = true
			beforeTargets := targetCalls
			beforeJudges := judgeCalls
			sub := vibe.Submission{ClientID: uuid.New(), Revision: h.session.Revision, Kind: "retest", Purpose: "regrade", ArtifactID: &a.ID, BaselineID: &original.ID, ApproveArtifact: true, Models: h.svc.Config.DefaultModels(), TestJourney: true}
			// A saved result does not need the old target model to still be available.
			sub.Models.Target = "retired-target"
			sub.Models.Assistant = "retired-author"
			regrade, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, sub)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := h.svc.Prepare(h.ctx, h.actor, h.session.ID, sub)
			if err != nil || receipt.ID != regrade.ID {
				t.Fatal("lost regrade idempotency", err)
			}
			if regrade.Grading.Hash == original.Grading.Hash || regrade.Source.Comparison != "regraded" {
				t.Fatal("new grading context unlabelled")
			}
			corrected, err := h.executeOperation(regrade.ID)
			if err != nil {
				t.Fatal(err)
			}
			if corrected.Scorecard.Passed != 3 || targetCalls != beforeTargets || judgeCalls-beforeJudges != 3 {
				t.Fatal("wrong regrade graph", corrected.Scorecard, targetCalls, judgeCalls)
			}
			if string(beforeArtifacts) != string(mustJSON(t, h.session.Document.Artifacts)) {
				t.Fatal("grade dispute changed suite")
			}
			for i, summary := range original.Results {
				r, e := h.svc.Store.GetCase(h.ctx, h.actor, original.ID, summary.CaseKey)
				if e != nil || !reflect.DeepEqual(r, originalCases[i]) {
					t.Fatal("historical result overwritten", e)
				}
				next, e := h.svc.Store.GetCase(h.ctx, h.actor, corrected.ID, summary.CaseKey)
				if e != nil || next.Output != r.Output || next.Checks[1].EvidenceVersion != 1 {
					t.Fatal("regrade lost exact reply or grounded evidence", e)
				}
			}
			_, err = h.svc.Prepare(h.ctx, "anon:"+uuid.NewString(), h.session.ID, sub)
			if err == nil {
				t.Fatal("foreign actor regraded private replies")
			}
		})
	}
}
