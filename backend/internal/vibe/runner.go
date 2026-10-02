package vibe

import (
	"context"
	"encoding/json"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/google/uuid"
)

type Runner struct {
	Service *Service
	Gateway *Gateway
}

// The assistant supplies semantic content. Scoring configuration and references
// are constructed by the compiler, outside the model's output contract.
type DraftProposal struct {
	TestsOnly       bool           `json:"-"`
	Summary         string         `json:"summary,omitempty"`
	Scenarios       []TestScenario `json:"scenarios,omitempty"`
	Title           string         `json:"title"`
	AgentPrompt     string         `json:"agent_prompt"`
	Examples        []string       `json:"examples"`
	SuccessCriteria string         `json:"success_criteria"`
}

var jsonFormat = json.RawMessage(`{"type":"json_object"}`)

func (r *Runner) Execute(ctx context.Context, id uuid.UUID) error {
	o, _, err := r.Service.Store.Start(ctx, id)
	if err != nil {
		return err
	}
	var p Plan
	if err = json.Unmarshal(o.Input, &p); err != nil {
		return err
	}
	if err = validateBoundSources(ctx, r.Service.Store.DB, o.SessionID, p); err != nil {
		return err
	}
	if p.Artifact != nil {
		if err = artifactReady(*p.Artifact, true); err != nil {
			return err
		}
	}
	if p.Grading != nil && !gradingSupported(p.Grading) {
		return fault("grading_changed", "This check needs its recorded grading version. Earlier results are preserved.")
	}
	if p.AuthoringVersion > contextualBuildAuthoringVersion {
		return fault("invalid_plan", "This request needs a newer conversation worker.")
	}
	ctx, cancel := context.WithDeadline(ctx, o.Deadline)
	defer cancel()
	if o.Kind == "message" || o.Kind == "build" {
		if _, ok := authoringPolicyFor(p.AuthoringVersion); !ok {
			return fault("invalid_plan", "This authoring contract has retired. Start a V1 project.")
		}
		return r.converseInterpreted(ctx, o, p)
	}
	if o.Kind == "playground" {
		messages := p.PreviewMessages
		if len(messages) == 0 { // Queued single-message trials from older clients.
			messages = []provider.Message{{Role: "system", Content: PreviewPrompt(p.Artifact.AgentPrompt)}, {Role: "user", Content: p.Submission.Content}}
		}
		messages, e := materialMessages(ctx, r.Service.Store.DB, o.SessionID, messages, executionInputs(p))
		if e != nil {
			return e
		}
		resp, e := r.Gateway.Call(ctx, o, "playground", Target, messages, nil)
		if e != nil {
			return e
		}
		reply := resp.OutputText
		if p.Submission.PreviewThreadID == nil {
			reply = "Agent response:\n\n" + reply
		}
		return r.Service.Store.CompleteDocument(ctx, id, reply, nil, nil)
	}
	if err = r.validateBeforeRun(ctx, o, &p); err != nil {
		return err
	}
	if p.Evidence != nil {
		return r.evaluateConversations(ctx, o, p)
	}
	return r.evaluate(ctx, o, p)
}

func (r *Runner) evaluate(ctx context.Context, o Operation, p Plan) error {
	compiled, err := r.Service.Compiler.Compile(p.Artifact.Blueprint, o.Models.Evaluator, p.Artifact.ID, p.limits())
	if err != nil {
		return err
	}
	// Compile also revalidates historical queued plans. Execute its shared
	// normalized scoring view without changing the accepted bundle/contract.
	definition, err := json.Marshal(compiled.Bundle.Version.EvaluationSpec)
	if err != nil {
		return err
	}
	spec, err := scoring.DecodeDefinition(definition)
	if err != nil {
		return err
	}
	if p.Grading != nil {
		hash, err := gradingCriteriaHash(spec)
		if err != nil {
			return err
		}
		if hash != p.Grading.CriteriaHash {
			return fault("grading_changed", "The saved grading rules no longer compile to the same settings. Start a new check.")
		}
	}
	version := p.Artifact.ID.String()
	// Persist every planned case as UNKNOWN before the first paid call. A worker
	// crash, cancellation, budget limit or provider outage cannot shrink totals.
	for _, c := range compiled.Cases {
		if err = r.Service.Store.PutResult(ctx, o.ID, CaseResult{CaseKey: c.CaseKey, Expected: ExpectedBehavior(c), ExpectedChecks: p.ChecksPerCase, Version: version, Input: raw(c.Payload), Verdict: Unknown, Checks: []CheckResult{}, Error: &Fault{Code: "not_evaluated", Message: "This case has not been evaluated yet."}}); err != nil {
			return err
		}
	}
	for _, c := range compiled.Cases {
		result := CaseResult{CaseKey: c.CaseKey, Expected: ExpectedBehavior(c), ExpectedChecks: p.ChecksPerCase, Version: version, Input: raw(c.Payload), Verdict: Unknown, Checks: []CheckResult{}}
		instructions, request := PreviewPrompt(p.Artifact.AgentPrompt), string(raw(TargetInput(c, spec)))
		if p.Artifact.IsTestSuite() {
			instructions = p.Artifact.AgentPrompt
			if question, ok := TargetInput(c, spec)["question"].(string); ok {
				request = question
			}
		}
		var response provider.Response
		var e error
		if p.RegradeOf != nil {
			response.OutputText, e = p.savedOutput(c.CaseKey)
			if e != nil {
				result.Error = &Fault{Code: "evidence_unavailable", Message: "No complete saved reply is available to recheck."}
				if err = r.Service.Store.PutResult(ctx, o.ID, result); err != nil {
					return err
				}
				continue
			}
		} else {
			messages, materialErr := materialMessages(ctx, r.Service.Store.DB, o.SessionID, []provider.Message{{Role: "system", Content: instructions}, {Role: "user", Content: request}}, p.Artifact.ReferenceInputs)
			if materialErr != nil {
				return materialErr
			}
			response, e = r.Gateway.Call(ctx, o, "target:"+c.CaseKey, Target, messages, nil)
		}
		result.Output = response.OutputText
		if e != nil {
			result.Error = issueFrom(e)
			_ = r.Service.Store.PutResult(context.WithoutCancel(ctx), o.ID, result)
			return e
		}
		input := scoring.EvaluationInput{RunAgentID: o.ID, EvaluationSpecID: p.Artifact.ID, ChallengeInputs: []scoring.EvidenceInput{CaseEvidence(c)}, Events: []scoring.Event{{Type: "system.output.finalized", OccurredAt: timestamp(), Payload: raw(map[string]any{"output": response.OutputText})}, {Type: "system.run.completed", OccurredAt: timestamp(), Payload: raw(map[string]any{"final_output": response.OutputText})}}}
		// Existing deterministic scoring primitives produce the numeric evidence.
		var eval scoring.RunAgentEvaluation
		e = nil
		for _, v := range spec.Validators {
			// The compiler bounds each resolved expected operand. At execution,
			// only the target output is new; unrelated case metadata is not an operand.
			if v.Type == scoring.ValidatorTypeFuzzyMatch && len(response.OutputText) > 2048 {
				e = fault("validator_operand_limit", "Fuzzy matching operands exceed the bounded preview limit.")
				break
			}
		}
		if e == nil {
			eval, e = scoring.EvaluateRunAgentWithLLMJudgeResults(input, spec, nil)
		}
		if e != nil {
			for _, v := range spec.Validators {
				result.Checks = append(result.Checks, CheckResult{Key: v.Key, Verdict: Unknown, Error: issueFrom(e)})
			}
			result.Error = &Fault{Code: "evaluation_invalid", Message: "The evaluation could not be applied to this evidence."}
		} else {
			for _, v := range eval.ValidatorResults {
				verdict := Unknown
				switch v.OutcomeClass {
				case scoring.ValidatorOutcomePass:
					verdict = Pass
				case scoring.ValidatorOutcomeFail:
					verdict = Fail
				}
				result.Checks = append(result.Checks, CheckResult{Key: v.Key, Verdict: verdict, Evidence: v.Reason})
			}
		}
		for _, judge := range spec.LLMJudges {
			check := CheckResult{Key: judge.Key, Verdict: Unknown}
			messages := JudgeMessages(judge, c, response.OutputText)
			if p.Grading != nil {
				messages = groundedJudgeMessagesForPlan(p, judge, c, response.OutputText)
			}
			messages, e = materialMessages(ctx, r.Service.Store.DB, o.SessionID, messages, p.Artifact.ReferenceInputs)
			if e != nil {
				return e
			}
			jr, je := r.Gateway.Call(ctx, o, "judge:"+c.CaseKey+":"+judge.Key, Evaluator, messages, jsonFormat)
			if je != nil {
				check.Error = issueFrom(je)
			} else {
				parsed, parseErr := ParseJudge(judge, []byte(jr.OutputText), p.limits())
				if p.Grading != nil {
					parsed, parseErr = parseGroundedJudge(judge, response.OutputText, []byte(jr.OutputText), p.limits())
				}
				if parseErr != nil {
					check.Error = &Fault{Code: "invalid_judge_output", Message: "The evaluator returned invalid or incomplete data. It was not repaired or counted as a behavioral failure."}
				} else {
					check = parsed
				}
			}

			result.Checks = append(result.Checks, check)
			if je != nil {
				// A provider/accounting failure ends the graph. Invalid JSON with
				// known cost is handled above as UNKNOWN and may continue.
				result.Error = issueFrom(je)
				result.Verdict = CaseVerdict(result.Checks)
				if err := r.Service.Store.PutResult(context.WithoutCancel(ctx), o.ID, result); err != nil {
					return err
				}
				return je
			}
		}
		result.Verdict = CaseVerdict(result.Checks)
		if result.Error != nil && result.Verdict == Pass {
			result.Verdict = Unknown
		}
		if err = r.Service.Store.PutResult(context.WithoutCancel(ctx), o.ID, result); err != nil {
			return err
		}
	}
	return nil
}
