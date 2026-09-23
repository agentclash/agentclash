package vibe

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"
)

const (
	understandingStep    = "advisory:signals"
	understandingVersion = "vibe-signals-v1"
	jevModel             = "typesafe/jev-1.13"
	jevResponseModel     = "typesafe/jev-1.13-20260917"
	jevEndpoint          = "https://openrouter.ai/api/v1/systemone"
)

// Separate from text-model profiles: Jev charges for input, with free output.
// An operator must verify/renew this profile; enabling a mode alone never spends.
type UnderstandingProfile struct {
	Model             string    `json:"model"`
	ResponseModel     string    `json:"response_model"`
	Provider          string    `json:"provider"`
	InputNanoPerToken int64     `json:"input_nano_per_token"`
	InputLimit        int       `json:"input_limit"`
	DeadlineMillis    int       `json:"deadline_ms"`
	Conformed         bool      `json:"conformed"`
	ExpiresAt         time.Time `json:"expires_at"`
}

func (p UnderstandingProfile) valid() bool {
	return p.Model == jevModel && p.ResponseModel == jevResponseModel && p.Provider == "typesafe" &&
		p.InputNanoPerToken == 50 && p.InputLimit >= 8192 && p.InputLimit <= 28000 &&
		p.DeadlineMillis >= 100 && p.DeadlineMillis <= 3000 && p.Conformed && !p.ExpiresAt.IsZero()
}

type UnderstandingPlan struct {
	Mode        string               `json:"mode"`
	Version     string               `json:"version"`
	RubricHash  string               `json:"rubric_hash"`
	Profile     UnderstandingProfile `json:"profile"`
	Request     []byte               `json:"request"` // Preserve exact wire bytes across JSONB normalization.
	RequestHash string               `json:"request_hash"`
	InputBound  int                  `json:"input_bound"`
	MaxCost     int64                `json:"max_cost"`
}

type UnderstandingSignals map[string]string

// Admission records why an enabled optional provider was skipped, so reports
// cannot accidentally count these turns as the deliberately disabled baseline.
type UnderstandingSelection struct {
	Mode       string `json:"mode"`
	SkipReason string `json:"skip_reason,omitempty"`
}

type UnderstandingOutcome struct {
	Version     string               `json:"version"`
	RequestHash string               `json:"request_hash"`
	Mode        string               `json:"mode"`
	Status      string               `json:"status"`
	Reason      string               `json:"reason,omitempty"`
	Signals     UnderstandingSignals `json:"signals,omitempty"`
}

//go:embed understanding_rubric_v1.json
var understandingRubric []byte

type signalSpec struct {
	Question string            `json:"question"`
	Options  map[string]string `json:"options"`
}
type signalQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}
type understandingRequest struct {
	Model     string                    `json:"model"`
	State     taskInput                 `json:"state"`
	Questions map[string]signalQuestion `json:"questions"`
	Provider  json.RawMessage           `json:"provider"`
}

func understandingQuestions() map[string]signalQuestion {
	var rubric struct {
		Instructions string                `json:"instructions"`
		Signals      map[string]signalSpec `json:"signals"`
	}
	if err := json.Unmarshal(understandingRubric, &rubric); err != nil {
		panic(err)
	}
	questions := map[string]signalQuestion{}
	for name, s := range rubric.Signals {
		questions[name] = signalQuestion{Type: "choice", Criteria: s.Options, Instructions: rubric.Instructions +
			"\nThe current_message is current_request.text. History is restricted to source-linked working_brief facts (stated/accepted only) and question_answers. No other past dialogue is supplied. pending_question is active only when provided. Ownership signals describe explicit claims, not verified connection status.\n" + s.Question}
	}
	return questions
}

// Only curated setup/clarification can benefit from this optional pass. Bound
// buttons, adoption confirmations, grading, manual edits and mature result chats
// already have server-owned state; they do not need a second classifier.
func understandingEligible(p Plan) bool {
	if !p.guided() || p.Free || p.Conversation == nil || p.Conversation.Manual != nil ||
		p.Conversation.Confirmed != nil || p.Submission.Interaction != nil || boundDialogueAction(p) != nil ||
		(p.Submission.Kind != "message" && p.Submission.Kind != "build") {
		return false
	}
	s := p.Conversation.State
	return s != nil && (p.Artifact == nil || s.PendingQuestion != nil && s.PendingQuestion.Status == "active" && s.PendingQuestion.ScopeID == s.Brief.ScopeID)
}

func prepareUnderstanding(p *Plan, cfg Config) {
	if cfg.UnderstandingMode != "shadow" && cfg.UnderstandingMode != "advisory" {
		return
	}
	p.UnderstandingSelection = &UnderstandingSelection{Mode: cfg.UnderstandingMode, SkipReason: "not_eligible"}
	if !understandingEligible(*p) {
		return
	}
	p.UnderstandingSelection.SkipReason = "profile_unavailable"
	if cfg.UnderstandingProfile == nil || !cfg.UnderstandingProfile.valid() || !cfg.UnderstandingProfile.ExpiresAt.After(timestamp()) {
		return
	}
	p.UnderstandingSelection.SkipReason = "context_limit"
	profile := *cfg.UnderstandingProfile
	request, err := understandingPayload(*p, profile)
	if err != nil {
		return
	}
	// UTF-8 bytes plus explicit framing is a conservative, tokenizer-independent
	// upper bound over the complete payload, including every rubric and option.
	bound := len(request) + 4096
	if bound > profile.InputLimit {
		return
	} // No lossy truncation of evidence.
	u := &UnderstandingPlan{Mode: cfg.UnderstandingMode, Version: understandingVersion, RubricHash: Hash(understandingRubric),
		Profile: profile, Request: request, RequestHash: Hash(request), InputBound: bound, MaxCost: int64(bound) * profile.InputNanoPerToken}
	if p.Calls+1 > p.limits().ModelCalls || p.MaxCost > MaxOperationCost-u.MaxCost {
		p.UnderstandingSelection.SkipReason = "allowance_unavailable"
		return
	}
	p.UnderstandingSelection.SkipReason = ""
	p.Understanding = u
	p.Calls++
	p.MaxCost += u.MaxCost // The original five-step allowance remains fully funded.
}

func validateUnderstandingPlan(p Plan) error {
	u := p.Understanding
	if u == nil {
		return nil
	}
	if !understandingEligible(p) || (u.Mode != "shadow" && u.Mode != "advisory") || u.Version != understandingVersion ||
		u.RubricHash != Hash(understandingRubric) || !u.Profile.valid() || u.RequestHash != Hash(u.Request) ||
		u.InputBound != len(u.Request)+4096 || u.InputBound > u.Profile.InputLimit ||
		u.MaxCost != int64(u.InputBound)*u.Profile.InputNanoPerToken || p.MaxCost <= u.MaxCost || p.Calls != 6 {
		return fault("invalid_plan", "The optional understanding allowance does not match its frozen contract.")
	}
	// Reassemble to guard route, state, rubric and policy, not just a self-hash.
	request, err := understandingPayload(p, u.Profile)
	if err != nil || Hash(request) != u.RequestHash {
		return fmt.Errorf("understanding request contract changed")
	}
	return nil
}

func understandingPayload(p Plan, profile UnderstandingProfile) ([]byte, error) {
	state, err := buildTaskInput(p, taskSignals, "", nil)
	if err != nil {
		return nil, err
	}
	return raw(understandingRequest{Model: profile.Model, State: state, Questions: understandingQuestions(),
		Provider: raw(map[string]any{"only": []string{profile.Provider}, "allow_fallbacks": false,
			"max_price": map[string]any{"prompt": 0.05, "completion": 0, "request": 0}})}), nil
}
