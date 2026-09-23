package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
)

type understandingRoundTrip func(*http.Request) (*http.Response, error)

func (f understandingRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func understandingService(t *testing.T, mode string) (*Service, Session) {
	s, v, _ := memoryService(t)
	s.Config.PreciseActions, s.Config.ContextGuidance = true, true
	s.Config.UnderstandingMode, s.Config.UnderstandingProfile = mode, jevTestProfile()
	return s, v
}
func understandingFixtureRunner(t *testing.T, s *Service, behavior string, counts *[2]int) *Runner {
	t.Helper()
	return &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate,
		UnderstandingTransport: understandingRoundTrip(func(req *http.Request) (*http.Response, error) {
			counts[0]++
			if req.URL.String() != jevEndpoint || req.Method != "POST" || req.Header.Get("Authorization") != "Bearer test-only-key" {
				t.Error("wrong provider boundary")
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			var sent understandingRequest
			if err = json.Unmarshal(body, &sent); err != nil || sent.Model != jevModel || len(sent.Questions) != 8 {
				t.Error("wrong systemone contract", err)
			}
			response := jevTestResponse()
			// Deliberately misleading advisory labels must never adopt a rule.
			for _, key := range []string{"needs_concept_explanation", "requests_change", "criteria_stated"} {
				a := response.Answers[key]
				a.Choice = "yes"
				a.Probabilities = map[string]float64{"yes": 1, "no": 0}
				response.Answers[key] = a
			}
			status := 200
			switch behavior {
			case "timeout":
				<-req.Context().Done()
				return nil, req.Context().Err()
			case "unavailable":
				return nil, fmt.Errorf("connection failed")
			case "rate_limit":
				status = 429
			case "redirect":
				status = 307
			case "unknown_cost":
				response.Usage.Cost = nil
			case "malformed":
				delete(response.Answers, "requests_change")
			case "over_cost":
				n := json.Number("0.1")
				response.Usage.Cost = &n
			}
			return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{"https://untrusted.invalid/"}}, Body: io.NopCloser(strings.NewReader(string(raw(response))))}, nil
		}),
		Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
			counts[1]++
			var input taskInput
			if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
				t.Fatal(err)
			}
			expected := s.Config.UnderstandingMode == "advisory" && behavior == "success"
			if expected != (len(input.ObservedSignals) > 0) {
				t.Error("shadow/fallback affected router context")
			}
			cost := json.Number("0.000001")
			return provider.Response{OutputText: string(raw(reliableRoute{Intent: "chat", Reply: "A test is an example message and what a good answer should do.", Memory: &memoryUpdate{}})), Usage: provider.Usage{CostUSD: &cost}}, nil
		}),
	}}
}

func TestIntegrationVibeUnderstandingModesAndFallback(t *testing.T) {
	for _, mode := range []string{"off", "shadow", "advisory"} {
		for _, behavior := range []string{"success", "malformed", "timeout", "unavailable", "rate_limit", "redirect", "unknown_cost"} {
			t.Run(mode+"/"+behavior, func(t *testing.T) {
				ctx := context.Background()
				s, v := understandingService(t, mode)
				o, p := memoryOperation(t, s, v, "What is a test?")
				counts := [2]int{}
				r := understandingFixtureRunner(t, s, behavior, &counts)
				if err := r.Execute(ctx, o.ID); err != nil {
					t.Fatal(err)
				}
				if err := r.Finalize(ctx, o.ID, nil); err != nil {
					t.Fatal(err)
				}
				actual, err := s.Store.Operation(ctx, o.ID)
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if mode == "off" {
					want = 0
				}
				if behavior == "success" {
					path := filepath.Join(t.TempDir(), "report.json")
					cmd := exec.Command("python3", "../../../scripts/vibe/understanding_report.py", "--session", v.ID.String(), "--output", path)
					cmd.Env = append(os.Environ(), "VIBE_REPORT_DATABASE_URL="+os.Getenv("VIBE_TEST_DATABASE_URL"), "PYTHONDONTWRITEBYTECODE=1")
					if output, e := cmd.CombinedOutput(); e != nil {
						t.Fatalf("whole-turn export: %v %s", e, output)
					}
					b, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					var report struct {
						Groups []struct {
							Attempts     int      `json:"all_attempts"`
							Conversation int      `json:"conversation_attempts"`
							KnownCost    int64    `json:"known_cost_nano_usd"`
							Latency      *float64 `json:"turn_p50_ms"`
						} `json:"groups"`
					}
					expectedAttempts, expectedCost := 2, int64(64000)
					if mode == "off" {
						expectedAttempts, expectedCost = 1, 1000
					}
					if json.Unmarshal(b, &report) != nil || len(report.Groups) != 1 || report.Groups[0].Attempts != expectedAttempts || report.Groups[0].Conversation != 1 || report.Groups[0].KnownCost != expectedCost || report.Groups[0].Latency == nil {
						t.Fatal("report dropped reply cost/latency", string(b))
					}
				}
				if counts != [2]int{want, 1} || actual.State != Completed || actual.Completion == nil {
					t.Fatal("optional step broke conversation", counts, actual.State)
				}
				current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
				if err != nil {
					t.Fatal(err)
				}
				if len(current.Document.Policies) != 0 || len(current.Document.Artifacts) != 0 {
					t.Fatal("advisory label became policy")
				}
				for _, fact := range current.Document.ConversationState.Brief.Facts {
					if fact.Status != "unknown" {
						t.Fatal("advisory label became a fact")
					}
				}
				out, err := s.Store.understandingOutcome(ctx, o.ID)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "off" {
					if out != nil || p.Understanding != nil {
						t.Fatal("off performed optional work")
					}
					return
				}
				if out == nil || (behavior == "success") != (out.Status == mode) {
					t.Fatal("missing fallback/observation journal", out)
				}
				uncertain := behavior != "success" && behavior != "malformed"
				if uncertain && actual.Billing != Reconciling {
					t.Fatal("uncertain advisory cost released", actual.Billing)
				}
				var amount int64
				var settled *int64
				if err = s.Store.DB.QueryRow(ctx, "SELECT amount,settled_amount FROM vibe_reservations WHERE operation_id=$1 LIMIT 1", o.ID).Scan(&amount, &settled); err != nil {
					t.Fatal(err)
				}
				if amount != p.MaxCost || uncertain && settled != nil {
					t.Fatal("fallback consumed/released advisory reservation")
				}
				var steps int
				if err = s.Store.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_attempts WHERE operation_id=$1", o.ID).Scan(&steps); err != nil || steps != 2 {
					t.Fatal("automatic paid retry", steps, err)
				}
			})
		}
	}
}

func TestIntegrationVibeUnderstandingRecoveryAndBudget(t *testing.T) {
	ctx := context.Background()
	t.Run("recorded_advisory_and_reply_recover_without_classification", func(t *testing.T) {
		s, v := understandingService(t, "advisory")
		o, p := memoryOperation(t, s, v, "What is a test?")
		o, _, err := s.Store.Start(ctx, o.ID)
		if err != nil {
			t.Fatal(err)
		}
		counts := [2]int{}
		r := understandingFixtureRunner(t, s, "success", &counts)
		if err = r.observeUnderstanding(ctx, o, &p); err != nil {
			t.Fatal(err)
		}
		profile := *p.Conversation.Profile
		if _, err = r.Gateway.Call(ctx, o, "route", Assistant, taskMessages(p, taskRoute, "", nil), reliableRouteFormat(profile, p)); err != nil {
			t.Fatal(err)
		}
		// Changed runtime config cannot reclassify or alter the recorded prompt.
		r.Gateway.Config.UnderstandingMode = "off"
		r.Gateway.Config.Profiles = nil
		if _, err = s.Store.DB.Exec(ctx, "UPDATE vibe_operations SET deadline=$2 WHERE id=$1", o.ID, timestamp().Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err = r.Finalize(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "lost acknowledgement"}); err != nil {
				t.Fatal(err)
			}
		}
		current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		if counts != [2]int{1, 1} || len(current.Document.Messages) != 2 || current.Operations[0].Completion == nil {
			t.Fatal("recovery lost/duplicated turn", counts)
		}
	})
	t.Run("missing_outcome_never_reclassifies", func(t *testing.T) {
		s, v := understandingService(t, "advisory")
		o, p := memoryOperation(t, s, v, "What is a test?")
		o, _, err := s.Store.Start(ctx, o.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Store.BeginAttempt(ctx, understandingAttempt(o, *p.Understanding)); err != nil {
			t.Fatal(err)
		}
		counts := [2]int{}
		r := understandingFixtureRunner(t, s, "success", &counts)
		if err = r.Finalize(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "lost acknowledgement"}); err != nil {
			t.Fatal(err)
		}
		current, err := s.Store.Operation(ctx, o.ID)
		if err != nil || current.State != Failed || current.Billing != Reconciling || counts != [2]int{} {
			t.Fatal("ambiguous attempt was replayed", counts, err)
		}
	})
	t.Run("partition_cannot_be_stolen", func(t *testing.T) {
		s, v := understandingService(t, "advisory")
		o, p := memoryOperation(t, s, v, "What is a test?")
		o, _, err := s.Store.Start(ctx, o.ID)
		if err != nil {
			t.Fatal(err)
		}
		a := understandingAttempt(o, *p.Understanding)
		bad := a
		bad.MaxCost++
		if s.Store.BeginAttempt(ctx, bad) == nil {
			t.Fatal("advisory stole conversation budget")
		}
		bad = a
		bad.Step = "route"
		if s.Store.BeginAttempt(ctx, bad) == nil {
			t.Fatal("Jev used as conversational model")
		}
		bad = a
		bad.Step = "route"
		bad.Model = o.Models.Assistant
		bad.MaxCost = o.MaxCost - p.Understanding.MaxCost + 1
		if s.Store.BeginAttempt(ctx, bad) == nil {
			t.Fatal("conversation stole advisory budget")
		}
		if err = s.Store.BeginAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		if s.Store.BeginAttempt(ctx, a) == nil {
			t.Fatal("duplicate dispatch accepted")
		}
	})
	t.Run("accounting_violation_stops", func(t *testing.T) {
		s, v := understandingService(t, "advisory")
		t.Cleanup(func() { _, _ = s.Store.DB.Exec(ctx, "DELETE FROM vibe_disabled_profiles WHERE model=$1", jevModel) })
		o, _ := memoryOperation(t, s, v, "What is a test?")
		counts := [2]int{}
		r := understandingFixtureRunner(t, s, "over_cost", &counts)
		if err := r.Execute(ctx, o.ID); issueFrom(err).Code != "accounting_bound_exceeded" {
			t.Fatal("overspend ignored", err)
		}
		if counts != [2]int{1, 0} {
			t.Fatal("continued after accounting discrepancy")
		}
	})
	t.Run("disabled_after_admission_uses_funded_conversation", func(t *testing.T) {
		s, v := understandingService(t, "advisory")
		o, _ := memoryOperation(t, s, v, "What is a test?")
		counts := [2]int{}
		r := understandingFixtureRunner(t, s, "disabled", &counts)
		r.Gateway.Config.UnderstandingMode = "off"
		if err := r.Execute(ctx, o.ID); err != nil {
			t.Fatal(err)
		}
		out, err := s.Store.understandingOutcome(ctx, o.ID)
		if err != nil || out == nil || out.Reason != "advisory_disabled" || counts != [2]int{0, 1} {
			t.Fatal("disabled optional provider blocked the reply", counts, err)
		}
	})
	t.Run("cancel_during_advisory_cannot_continue", func(t *testing.T) {
		s, v := understandingService(t, "advisory")
		o, _ := memoryOperation(t, s, v, "What is a test?")
		counts := [2]int{}
		r := understandingFixtureRunner(t, s, "success", &counts)
		r.Gateway.UnderstandingTransport = understandingRoundTrip(func(req *http.Request) (*http.Response, error) {
			counts[0]++
			if err := s.Store.Stop(ctx, v.Actor, o.ID); err != nil {
				t.Fatal(err)
			}
			return nil, fmt.Errorf("stopped")
		})
		if err := r.Execute(ctx, o.ID); err == nil {
			t.Fatal("stopped operation continued")
		}
		if counts != [2]int{1, 0} {
			t.Fatal("cancellation ran fallback", counts)
		}
	})
}
