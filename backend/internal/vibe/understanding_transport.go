package vibe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type signalAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    *float64           `json:"confidence"`
}
type understandingResponse struct {
	ID       string                  `json:"id"`
	Model    string                  `json:"model"`
	Provider string                  `json:"provider"`
	Answers  map[string]signalAnswer `json:"answers"`
	Usage    struct {
		InputTokens  *int64       `json:"input_tokens"`
		OutputTokens *int64       `json:"output_tokens"`
		Cost         *json.Number `json:"cost"`
	} `json:"usage"`
	Error json.RawMessage `json:"error"`
}

func parseUnderstanding(body []byte, u UnderstandingPlan) (understandingResponse, UnderstandingSignals, *Fault) {
	var response understandingResponse
	invalid := &Fault{Code: "advisory_invalid", Message: "Optional understanding was not usable; using conversation."}
	if ValidateJSON(body, LimitsFor(false)) != nil || json.Unmarshal(body, &response) != nil {
		return understandingResponse{}, nil, invalid
	}
	if response.Model != u.Profile.ResponseModel || !strings.EqualFold(response.Provider, u.Profile.Provider) ||
		response.ID == "" || len(response.ID) > 256 || (len(response.Error) > 0 && string(response.Error) != "null") {
		return response, nil, invalid
	}
	var request understandingRequest
	if json.Unmarshal(u.Request, &request) != nil || len(response.Answers) != len(request.Questions) {
		return response, nil, invalid
	}
	signals := UnderstandingSignals{}
	for key, question := range request.Questions {
		a, ok := response.Answers[key]
		if !ok || a.Type != "choice" || question.Criteria[a.Choice] == "" || a.Confidence == nil ||
			math.IsNaN(*a.Confidence) || *a.Confidence < 0 || *a.Confidence > 1 || len(a.Probabilities) != len(question.Criteria) {
			return response, nil, invalid
		}
		sum := 0.0
		for label := range question.Criteria {
			v, exists := a.Probabilities[label]
			if !exists || math.IsNaN(v) || v < 0 || v > 1 || v > a.Probabilities[a.Choice]+0.000001 {
				return response, nil, invalid
			}
			sum += v
		}
		if math.Abs(sum-1) > 0.02 {
			return response, nil, invalid
		}
		signals[key] = a.Choice
	}
	if signals["answers_pending"] == "yes" && request.State.PendingQuestion == nil {
		return response, nil, invalid
	}
	return response, signals, nil
}

func understandingAttempt(o Operation, u UnderstandingPlan) Attempt {
	return Attempt{ID: uuid.New(), OperationID: o.ID, Step: understandingStep, Role: Assistant, Model: u.Profile.Model,
		Policy: raw(u), RequestHash: u.RequestHash, InputBound: u.InputBound, MaxOutput: 512, MaxCost: u.MaxCost}
}

// Invoke exactly once, at a fixed HTTPS origin. A transport may be injected by
// tests, but no user-supplied URL, redirect or provider fallback can get a key.
func (g *Gateway) callUnderstanding(ctx context.Context, o Operation, u UnderstandingPlan) (UnderstandingSignals, *Fault, error) {
	fallback := func(code string) (UnderstandingSignals, *Fault, error) {
		return nil, &Fault{Code: code, Message: "Optional understanding unavailable; using conversation."}, nil
	}
	if !g.Config.Enabled || g.Config.Credential == "" || g.Config.FreeOnly ||
		g.Config.UnderstandingMode != u.Mode ||
		g.Config.UnderstandingProfile == nil || Hash(raw(*g.Config.UnderstandingProfile)) != Hash(raw(u.Profile)) ||
		!u.Profile.ExpiresAt.After(timestamp()) {
		return fallback("advisory_disabled")
	}
	if err := g.Gate.Healthy(ctx); err != nil {
		return nil, nil, err
	}
	a := understandingAttempt(o, u)
	if err := g.Store.BeginAttempt(ctx, a); err != nil {
		return nil, nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(u.Profile.DeadlineMillis)*time.Millisecond)
	defer cancel()
	callCtx, finish := traceCall(callCtx, Assistant, u.Profile.Model, understandingStep)
	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, jevEndpoint, bytes.NewReader(u.Request))
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Authorization", "Bearer "+g.Config.Credential)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	client := &http.Client{Transport: g.UnderstandingTransport, Timeout: time.Duration(u.Profile.DeadlineMillis) * time.Millisecond,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	httpResponse, httpErr := client.Do(request)
	var body []byte
	issue := &Fault{Code: "advisory_unavailable", Message: "Optional understanding unavailable; using conversation."}
	var signals UnderstandingSignals
	var response understandingResponse
	var cost *int64
	if httpErr == nil {
		body, err = io.ReadAll(io.LimitReader(httpResponse.Body, 64*1024+1))
		_ = httpResponse.Body.Close()
		if err == nil && len(body) <= 64*1024 && httpResponse.StatusCode == http.StatusOK {
			response, signals, issue = parseUnderstanding(body, u)
			if response.Usage.Cost != nil {
				if n, e := ParseUSD(response.Usage.Cost.String()); e == nil {
					cost = &n
				}
			}
			if cost == nil {
				issue = &Fault{Code: "advisory_usage_unknown", Message: "Optional usage remains reserved; using the separately funded conversation."}
			} else if *cost > u.MaxCost || response.Usage.InputTokens != nil && *response.Usage.InputTokens > int64(u.InputBound) ||
				response.Usage.OutputTokens != nil && *response.Usage.OutputTokens > 512 {
				issue = &Fault{Code: "accounting_bound_exceeded", Message: "Optional understanding exceeded its approved bound."}
			} else if response.Usage.InputTokens == nil || *response.Usage.InputTokens < 0 || response.Usage.OutputTokens == nil || *response.Usage.OutputTokens < 0 {
				issue = &Fault{Code: "advisory_invalid", Message: "Optional understanding returned incomplete usage."}
			}
		} else {
			// Never persist arbitrary HTTP error bodies (they may echo credentials).
			body = nil
		}
	}
	if callCtx.Err() != nil && (issue == nil || issue.Code != "accounting_bound_exceeded") {
		issue = &Fault{Code: "advisory_timeout", Message: "Optional understanding timed out; using conversation."}
	}
	if issue != nil {
		signals = nil
	}
	var traceErr error
	if issue != nil {
		traceErr = issue
	}
	finish(traceErr, cost)
	journalCtx, journalCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer journalCancel()
	if issue != nil && issue.Code == "accounting_bound_exceeded" {
		if _, err = g.Store.DB.Exec(journalCtx, "INSERT INTO vibe_disabled_profiles(model,reason) VALUES($1,'advisory bound exceeded') ON CONFLICT DO NOTHING", u.Profile.Model); err != nil {
			return nil, nil, err
		}
	}
	if response.ID != "" && len(response.ID) <= 256 {
		if err = g.Store.Generation(journalCtx, a.ID, response.ID); err != nil {
			return nil, nil, err
		}
	}
	// Discard provider error prose; retain only typed, bounded evidence.
	response.Error = nil
	// The journal contains structured response evidence, not the bearer credential.
	if err = g.Store.EndAttempt(journalCtx, a, string(raw(signals)), []byte(strings.ReplaceAll(string(raw(response)), g.Config.Credential, "[redacted]")), cost, issue); err != nil {
		return nil, nil, err
	}
	if issue != nil && issue.Code == "accounting_bound_exceeded" {
		return nil, nil, issue
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	return signals, issue, nil
}
