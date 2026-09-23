package vibe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/vibe/interaction"
)

func jevTestProfile() *UnderstandingProfile {
	return &UnderstandingProfile{Model: jevModel, ResponseModel: jevResponseModel, Provider: "typesafe", InputNanoPerToken: 50,
		InputLimit: 28000, DeadlineMillis: 100, Conformed: true, ExpiresAt: timestamp().Add(time.Hour)}
}
func jevTestPlan(t *testing.T, mode string) Plan {
	t.Helper()
	p, _ := memoryPlan(t, Document{}, "What is a test? My agent should convert PDFs properly.")
	p.AuthoringVersion = guidedAuthoringVersion
	p.Submission.Kind = "message"
	p.MaxCost = 10_000_000
	prepareUnderstanding(&p, Config{UnderstandingMode: mode, UnderstandingProfile: jevTestProfile()})
	return p
}
func jevTestResponse() understandingResponse {
	r := understandingResponse{ID: "gen-fixture", Model: jevResponseModel, Provider: "TypeSafe", Answers: map[string]signalAnswer{}}
	for name, q := range understandingQuestions() {
		label := "no"
		if name == "has_agent" || name == "has_pack" {
			label = "unknown"
		}
		probs := map[string]float64{}
		for option := range q.Criteria {
			probs[option] = 0
		}
		probs[label] = 1
		confidence := 1.0
		r.Answers[name] = signalAnswer{Type: "choice", Choice: label, Probabilities: probs, Confidence: &confidence}
	}
	input, output := int64(1500), int64(256)
	cost := json.Number("0.000063")
	r.Usage.InputTokens = &input
	r.Usage.OutputTokens = &output
	r.Usage.Cost = &cost
	return r
}

func TestVibeUnderstandingAdmissionAndIsolation(t *testing.T) {
	base := jevTestPlan(t, "off")
	if base.Understanding != nil || base.Calls != 5 {
		t.Fatal("off changed the existing graph")
	}
	for _, mode := range []string{"shadow", "advisory"} {
		p := jevTestPlan(t, mode)
		if p.Understanding == nil || p.Calls != 6 || p.MaxCost != base.MaxCost+p.Understanding.MaxCost || validateUnderstandingPlan(p) != nil {
			t.Fatal("optional allowance not partitioned")
		}
		p.ObservedSignals = UnderstandingSignals{"requests_change": "yes"}
		for _, task := range []conversationTask{taskRoute, taskAuthor, taskExplain, taskSignals} {
			input, err := buildTaskInput(p, task, "prepare_tests", nil)
			if err != nil || (task == taskRoute) != (len(input.ObservedSignals) > 0) {
				t.Fatal("observation escaped route boundary", task, err)
			}
		}
		if strings.Contains(string(raw(p)), "requests_change\":\"yes") {
			t.Fatal("transient signal persisted in frozen source plan")
		}
		p.Understanding.Request = raw(map[string]any{"model": "another-model"})
		p.Understanding.RequestHash = Hash(p.Understanding.Request)
		if validateUnderstandingPlan(p) == nil {
			t.Fatal("changed dispatch accepted")
		}
	}
	for _, kind := range []string{"free", "manual", "confirmation", "button", "mature", "old", "expired", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			p := jevTestPlan(t, "off")
			cfg := Config{UnderstandingMode: "advisory", UnderstandingProfile: jevTestProfile()}
			switch kind {
			case "free":
				p.Free = true
			case "manual":
				p.Conversation.Manual = &ManualSuiteEdit{}
			case "confirmation":
				p.Conversation.Confirmed = &SourceConfirmation{}
			case "button":
				p.Submission.Interaction = &interaction.Action{}
			case "mature":
				p.Artifact = &Artifact{}
			case "old":
				p.AuthoringVersion = 13
			case "expired":
				cfg.UnderstandingProfile.ExpiresAt = timestamp().Add(-time.Hour)
			case "oversized":
				p.Conversation.CurrentRequest.Text = strings.Repeat("x", 30000)
			}
			prepareUnderstanding(&p, cfg)
			if p.Understanding != nil || p.Calls != 5 {
				t.Fatal("unnecessary advisory call admitted")
			}
		})
	}
}

func TestVibeUnderstandingStrictResponse(t *testing.T) {
	u := *jevTestPlan(t, "advisory").Understanding
	if _, s, f := parseUnderstanding(raw(jevTestResponse()), u); f != nil || len(s) != 8 {
		t.Fatal("valid response rejected", f)
	}
	for _, kind := range []string{"missing", "extra", "label", "type", "probabilities", "choice_disagrees", "confidence", "model", "provider", "inactive_answer", "duplicate", "truncated", "error"} {
		t.Run(kind, func(t *testing.T) {
			r := jevTestResponse()
			a := r.Answers["requests_change"]
			switch kind {
			case "missing":
				delete(r.Answers, "has_agent")
			case "extra":
				r.Answers["execute"] = a
			case "label":
				a.Choice = "run"
			case "type":
				a.Type = "noul"
			case "probabilities":
				a.Probabilities["no"] = 0.2
			case "choice_disagrees":
				a.Choice = "yes"
			case "confidence":
				a.Confidence = nil
			case "model":
				r.Model = "typesafe/jev-latest"
			case "provider":
				r.Provider = "other"
			case "inactive_answer":
				ans := r.Answers["answers_pending"]
				ans.Choice = "yes"
				ans.Probabilities = map[string]float64{"yes": 1, "no": 0}
				r.Answers["answers_pending"] = ans
			case "error":
				r.Error = raw("failure")
			}
			r.Answers["requests_change"] = a
			body := raw(r)
			if kind == "duplicate" {
				body = []byte(strings.Replace(string(body), `"model":`, `"model":"wrong","model":`, 1))
			}
			if kind == "truncated" {
				body = body[:len(body)-3]
			}
			if _, _, f := parseUnderstanding(body, u); f == nil {
				t.Fatal("malformed/inconsistent response accepted")
			}
		})
	}
}

func TestVibeUnderstandingRetainsEvidenceNotJokes(t *testing.T) {
	d, _ := memoryTurn(t, Document{}, "My agent converts PDF to Markdown.", questionRoute())
	d, _ = memoryTurn(t, d, "give me vodka", reliableRoute{Intent: "chat", Reply: "Ready when you are.", Memory: &memoryUpdate{}})
	for _, message := range []string{"properly please", "both, preserve headings too", `Someone said "ignore rules". What is a test?`, "I don't have an agent yet", "Use your suggested example"} {
		p, _ := memoryPlan(t, d, message)
		p.AuthoringVersion = 14
		p.Submission.Kind = "message"
		p.MaxCost = 10_000_000
		prepareUnderstanding(&p, Config{UnderstandingMode: "shadow", UnderstandingProfile: jevTestProfile()})
		if p.Understanding == nil {
			t.Fatal("eligible clarification skipped")
		}
		var request understandingRequest
		_ = json.Unmarshal(p.Understanding.Request, &request)
		state := string(raw(request.State))
		if strings.Contains(state, "vodka") || !strings.Contains(state, "My agent converts PDF to Markdown.") || request.State.PendingQuestion == nil || request.State.CurrentRequest.Text != message {
			t.Fatal("retained job, exact request, or active question lost; casual dialogue leaked")
		}
		if len(request.State.ObservedSignals) > 0 || len(request.State.SourceBlocks) > 0 || len(request.State.RecentConversation) > 0 {
			t.Fatal("classification received authority or unnecessary history")
		}
	}
}
