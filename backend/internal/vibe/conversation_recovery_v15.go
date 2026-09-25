package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
)

// Frozen at admission. One alternative call in the entire operation, never a
// change to the target/evaluator or the user's selected assistant for later turns.
type AssistantRecovery struct {
	Profile ModelProfile `json:"profile"`
	MaxCost int64        `json:"max_cost_nano_usd"`
}

func understandingExpectedCalls(p Plan) int {
	if p.interpreted() {
		if p.AssistantRecovery != nil {
			return 10
		}
		return 9
	}
	return 6
}

func validateInterpretedAllowance(p Plan, cfg Config) error {
	if !p.interpreted() {
		if p.AssistantRecovery != nil {
			return fault("invalid_plan", "Alternative assistants require the versioned authoring contract.")
		}
		return nil
	}
	if !cfg.InterpretedAuthoring || p.Conversation == nil || p.Conversation.Profile == nil || p.Conversation.Manual != nil {
		return fault("invalid_plan", "The interpreted authoring contract is unavailable.")
	}
	profile := *p.Conversation.Profile
	cost, err := profile.BoundCost(profile.inputLimit(p.limits()), p.limits().OutputTokens)
	if err != nil {
		return err
	}
	expectedCost, expectedCalls := 8*cost, 8
	if p.AuthoringVersion == buildAuthoringVersion {
		expectedCost, expectedCalls = 10*cost, 10
	}
	if r := p.AssistantRecovery; r != nil {
		model := ""
		if strings.HasPrefix(profile.ID, "deepseek/") {
			model = "openai/gpt-5.4-mini"
		}
		if profile.ID == "openai/gpt-5.4-mini" {
			model = "deepseek/deepseek-v4.1-flash"
		}
		configured, ok := cfg.Profiles[r.Profile.ID]
		bound, e := r.Profile.BoundCost(r.Profile.inputLimit(p.limits()), p.limits().OutputTokens)
		if cfg.FreeOnly || !cfg.AssistantFallback || model == "" || r.Profile.ID != model || !ok || Hash(raw(configured)) != Hash(raw(r.Profile)) || e != nil || bound != r.MaxCost {
			return fault("invalid_plan", "The alternative assistant does not match its approved allowance.")
		}
		expectedCalls++
		expectedCost += bound
	}
	if p.Understanding != nil {
		expectedCalls++
		expectedCost += p.Understanding.MaxCost
	}
	if p.Calls != expectedCalls || p.MaxCost != expectedCost {
		return fault("budget_limit", "The assistant call graph is not fully reserved.")
	}
	return nil
}

func prepareInterpretedPlan(p *Plan, cfg Config, primary ModelProfile) error {
	if !cfg.InterpretedAuthoring || !p.guided() || p.Conversation.Manual != nil {
		return nil
	}
	p.AuthoringVersion = interpretedAuthoringVersion
	p.Conversation.ContractVersion = "vibe-v15"
	p.Calls = 8 // three initial/repair pairs plus one candidate patch/review pair
	if p.Cycle != nil && p.Cycle.Step != "check" {
		p.AuthoringVersion = buildAuthoringVersion
		p.Conversation.ContractVersion = "vibe-v16"
		p.Calls = 10 // v15 preparation plus an independent prototype/repair pair
	}
	l := p.limits()
	l.OperationSeconds = min(l.QueueSeconds+9*l.ProviderSeconds+30, 16*60)
	p.ExecutionLimits = &l
	if cfg.FreeOnly || !cfg.AssistantFallback {
		return nil
	}
	model := ""
	if strings.HasPrefix(primary.ID, "deepseek/") {
		model = "openai/gpt-5.4-mini"
	}
	if primary.ID == "openai/gpt-5.4-mini" {
		model = "deepseek/deepseek-v4.1-flash"
	}
	if model == "" {
		return nil
	}
	fallback, err := cfg.Profile(model)
	if err != nil {
		return nil
	} // Missing/unverified alternatives never block the primary.
	cost, err := fallback.BoundCost(fallback.inputLimit(l), l.OutputTokens)
	if err != nil {
		return err
	}
	p.AssistantRecovery = &AssistantRecovery{Profile: fallback, MaxCost: cost}
	p.Calls++
	return nil
}

func assistantStepProfile(p Plan, step string) (*ModelProfile, error) {
	if p.Conversation == nil || p.Conversation.Profile == nil {
		return nil, fmt.Errorf("missing frozen assistant profile")
	}
	if p.interpreted() && strings.HasSuffix(step, ":fallback") {
		if p.AssistantRecovery == nil || !interpretedStepAllowed(step) {
			return nil, fault("operation_limit", "This alternative was not admitted.")
		}
		return &p.AssistantRecovery.Profile, nil
	}
	return p.Conversation.Profile, nil
}

func interpretedStepAllowed(step string) bool {
	switch step {
	case "route", "route:repair", "route:fallback", "handler", "handler:repair", "handler:fallback", "review", "review:repair", "review:fallback", "candidate:patch", "candidate:review", "prototype", "prototype:repair", "prototype:fallback":
		return true
	}
	return false
}

func recoverableAssistantFault(err error) bool {
	var f *Fault
	if !errors.As(err, &f) {
		return false
	}
	switch f.Code {
	case "provider_rate_limit", "provider_unavailable", "invalid_response":
		return true
	}
	return false
}

// Validation is part of a stage. Only invalid output may be repaired/replaced;
// a well-formed negative semantic review is a valid result, never a retry cue.
func (r *Runner) interpretedStage(ctx context.Context, o Operation, p Plan, stage string, used *bool, messages []provider.Message, format func(ModelProfile) json.RawMessage, validate func([]byte) error) (provider.Response, ModelProfile, error) {
	primary, err := r.reliableProfile(o, p)
	if err != nil {
		return provider.Response{}, primary, err
	}
	profile := primary
	step := stage
	current := messages
	for {
		resp, callErr := r.reliableCall(ctx, o, step, current, format(profile))
		if callErr != nil && !recoverableAssistantFault(callErr) {
			return resp, profile, callErr
		}
		validationErr := callErr
		if callErr == nil {
			validationErr = validate([]byte(resp.OutputText))
			if e := r.journalReliable(ctx, o, step, stage, p, nil, validationErr); e != nil {
				return resp, profile, e
			}
		}
		if validationErr == nil {
			return resp, profile, nil
		}
		if ctx.Err() != nil {
			return resp, profile, ctx.Err()
		}
		if callErr == nil && step == stage {
			step = stage + ":repair"
			current = stageFeedback(messages, provider.Message{Role: "user", Content: string(raw(map[string]any{"server_validation_feedback": boundedDiagnostic(validationErr.Error()), "invalid_output": resp.OutputText, "instruction": "Correct only this stage using the original request and schema. Do not invent rules or change valid unrelated fields."}))})
			continue
		}
		if !*used && p.AssistantRecovery != nil && !strings.HasSuffix(step, ":fallback") {
			if callErr != nil {
				var unsettled bool
				if e := r.Service.Store.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND (actual_cost IS NULL OR completed_at IS NULL))`, o.ID).Scan(&unsettled); e != nil {
					return resp, profile, e
				}
				if unsettled {
					return resp, profile, callErr
				}
			}
			*used = true
			step = stage + ":fallback"
			profile = p.AssistantRecovery.Profile
			// An independent attempt sees the same evidence and precise feedback,
			// not the first model's untrusted failed state as an accepted premise.
			current = stageFeedback(messages, provider.Message{Role: "user", Content: string(raw(map[string]any{"server_validation_feedback": boundedDiagnostic(validationErr.Error()), "instruction": "Complete this stage from the original evidence. No previous output was accepted."}))})
			continue
		}
		if callErr != nil {
			return resp, profile, callErr
		}
		return resp, profile, fault("assistant_"+stage+"_failed", "I couldn't finish preparing this response. Your message is saved and your existing tests are unchanged. Please retry.")
	}
}

// Keep the exact current request last for interpretation/authoring; review has
// only system+evidence and receives feedback after that evidence.
func stageFeedback(messages []provider.Message, feedback provider.Message) []provider.Message {
	out := append([]provider.Message(nil), messages...)
	if len(out) >= 3 {
		last := out[len(out)-1]
		out[len(out)-1] = feedback
		return append(out, last)
	}
	return append(out, feedback)
}
