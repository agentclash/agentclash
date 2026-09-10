package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

type Gate struct{ Redis *redis.Client }

func (g Gate) Check(ctx context.Context, actor string, l Limits) error {
	if g.Redis == nil {
		return fault("accounting_unavailable", "Hosted execution is unavailable while budget protection is offline.")
	}
	key := "vibe:rate:" + Hash([]byte(actor)) + ":" + timestamp().Format("200601021504")
	n, err := g.Redis.Eval(ctx, `local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],120) end; return n`, []string{key}).Int()
	if err != nil {
		return fault("accounting_unavailable", "Hosted execution is unavailable while budget protection is offline.")
	}
	if n > l.Rate {
		return fault("rate_limit", "Too many requests. Try again in a minute.")
	}
	return nil
}
func (g Gate) Healthy(ctx context.Context) error {
	if g.Redis == nil {
		return fault("accounting_unavailable", "Budget protection is offline.")
	}
	if err := g.Redis.Ping(ctx).Err(); err != nil {
		return fault("accounting_unavailable", "Budget protection is offline.")
	}
	return nil
}

type credential struct{ key string }

func (c credential) Resolve(_ context.Context, ref string) (string, error) {
	if ref != "vibe-hosted" || c.key == "" {
		return "", provider.ErrCredentialUnavailable
	}
	return c.key, nil
}

type Gateway struct {
	Store  *Store
	Config Config
	Gate   Gate
	Client provider.Client
}

func (g *Gateway) Call(ctx context.Context, o Operation, step string, role Role, messages []provider.Message, format json.RawMessage) (provider.Response, error) {
	var plan Plan
	if err := json.Unmarshal(o.Input, &plan); err != nil {
		return provider.Response{}, err
	}
	l := LimitsFor(plan.Anonymous)
	if !g.Config.Enabled || g.Config.Credential == "" {
		return provider.Response{}, fault("hosted_disabled", "Hosted model execution is not configured.")
	}
	if err := g.Gate.Healthy(ctx); err != nil {
		return provider.Response{}, err
	}
	model := ""
	switch role {
	case Assistant:
		model = o.Models.Assistant
	case Target:
		model = o.Models.Target
	case Evaluator:
		model = o.Models.Evaluator
	default:
		return provider.Response{}, fault("invalid_role", "Invalid model role.")
	}
	p, err := g.Config.Profile(model)
	if err != nil {
		return provider.Response{}, err
	}
	// max_price is in USD per million tokens. Nano-USD/token divided by 1000.
	policy := raw(map[string]any{"only": []string{p.Route}, "allow_fallbacks": false, "require_parameters": true, "max_price": map[string]any{"prompt": json.Number(fmt.Sprintf("%.3f", float64(p.InputNanoPerToken)/1000)), "completion": json.Number(fmt.Sprintf("%.3f", float64(p.OutputNanoPerToken)/1000))}})
	if p.Free {
		policy = raw(map[string]any{"only": []string{p.Route}, "allow_fallbacks": false, "require_parameters": true, "max_price": map[string]int{"prompt": 0, "completion": 0, "request": 0}})
	}
	temp := 0.0
	req := provider.Request{ProviderKey: "openrouter", CredentialReference: "vibe-hosted", Model: model, Messages: messages, MaxOutputTokens: l.OutputTokens, Temperature: &temp, ResponseFormat: format, OpenRouterPolicy: policy, MaxResponseBytes: MaxProviderResponseBytes, StepTimeout: time.Duration(l.ProviderSeconds) * time.Second}
	if p.DisableReasoning {
		req.Reasoning = raw(map[string]bool{"enabled": false})
	}
	count, err := CountContext(req, p, l)
	if err != nil {
		return provider.Response{}, err
	}
	cost, err := p.BoundCost(count.UpperBound, req.MaxOutputTokens)
	if err != nil {
		return provider.Response{}, err
	}
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: step, Role: role, Model: model, Policy: raw(map[string]any{"profile": p, "routing": json.RawMessage(policy), "temperature": temp, "context": count, "response_format": format, "reasoning": req.Reasoning}), RequestHash: Hash(raw(messages)), InputBound: count.UpperBound, MaxOutput: req.MaxOutputTokens, MaxCost: cost}
	if err = g.Store.BeginAttempt(ctx, a); err != nil {
		return provider.Response{}, err
	}
	journalCtx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	}
	req.OnGeneration = func(id string) error {
		c, cancel := journalCtx()
		defer cancel()
		return g.Store.Generation(c, a.ID, id)
	}
	ctx, finishTrace := traceCall(ctx, role, model)
	callCtx, cancelCall := context.WithCancel(ctx)
	done := make(chan struct{})
	defer close(done)
	defer cancelCall()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-callCtx.Done():
				return
			case <-ticker.C:
				current, e := g.Store.Operation(callCtx, o.ID)
				if e != nil || current.State != Running {
					cancelCall()
					return
				}
			}
		}
	}()
	client := g.Client
	if client == nil {
		client = provider.NewDefaultRouter(nil, credential{g.Config.Credential})
	}
	var response provider.Response
	var output strings.Builder
	if stream, ok := client.(provider.StreamingClient); ok {
		response, err = stream.StreamModel(callCtx, req, func(d provider.StreamDelta) error {
			if d.Kind == provider.StreamDeltaKindText {
				if output.Len()+len(d.Text) > MaxOutputTextBytes {
					return fault("provider_response_limit", "Provider output exceeded its byte limit.")
				}
				output.WriteString(d.Text)
				c, cancel := journalCtx()
				defer cancel()
				return g.Store.AppendOutput(c, a.ID, d.Text)
			}
			return nil
		})
	} else {
		response, err = client.InvokeModel(callCtx, req)
		output.WriteString(response.OutputText)
		if len(response.OutputText) > MaxOutputTextBytes {
			err = fault("provider_response_limit", "Provider text exceeded its byte limit.")
		}
	}
	var actual *int64
	if response.OutputText == "" && output.Len() > 0 {
		response.OutputText = output.String()
	}
	issue := issueFrom(err)
	defer func() {
		if issue != nil {
			finishTrace(issue, actual)
		} else {
			finishTrace(err, actual)
		}
	}()
	if err == nil && response.Usage.CostUSD != nil {
		n, e := ParseUSD(response.Usage.CostUSD.String())
		if e == nil {
			actual = &n
		} else {
			issue = &Fault{Code: "usage_invalid", Message: "Provider usage could not be accounted. Its reservation remains held."}
		}
	}
	if err == nil && (response.Usage.InputTokens > int64(count.UpperBound) || response.Usage.OutputTokens > int64(req.MaxOutputTokens)) {
		issue = &Fault{Code: "accounting_bound_exceeded", Message: "Provider usage exceeded its approved context limit."}
		c, cancel := journalCtx()
		_, _ = g.Store.DB.Exec(c, "INSERT INTO vibe_disabled_profiles(model,reason) VALUES($1,'token bound exceeded') ON CONFLICT DO NOTHING", model)
		cancel()
	}
	if err == nil && response.FinishReason == provider.FinishReasonMaxTokens {
		issue = &Fault{Code: "output_truncated", Message: "The model reached its output limit. The result has not been treated as a completed evaluation."}
	}
	if actual != nil && *actual > a.MaxCost {
		issue = &Fault{Code: "accounting_bound_exceeded", Message: "Provider cost exceeded its approved ceiling. Further execution is stopped for accounting review."}
	}
	if actual == nil && issue == nil {
		issue = &Fault{Code: "usage_unknown", Message: "Provider cost is still being reconciled. No further calls will be started."}
	}
	c, cancel := journalCtx()
	defer cancel()
	if e := g.Store.EndAttempt(c, a, output.String(), raw(response), actual, issue); e != nil {
		return response, e
	}
	if issue != nil {
		return response, issue
	}
	return response, nil
}
