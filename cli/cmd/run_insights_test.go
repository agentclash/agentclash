package cmd

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

const (
	testProviderAccountID = "99999999-9999-4999-8999-999999999999"
	testWinnerAgentID     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testRunnerUpAgentID   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

const runInsightsPath = "/v1/runs/" + testRunID + "/ranking-insights"

func runInsightsFixture() map[string]any {
	return map[string]any{
		"generated_at":       "2026-11-24T10:00:00Z",
		"grounding_scope":    "current_run_only",
		"provider_key":       "openai",
		"provider_model_id":  "gpt-4.1-mini-2025-04-14",
		"recommended_winner": map[string]any{"run_agent_id": testWinnerAgentID, "label": "claude-agent"},
		"why_it_won":         "Highest correctness on all cases.",
		"tradeoffs":          []any{"Slower than gpt-agent", "Costs 2x more per case"},
		"best_for_reliability": map[string]any{
			"run_agent_id": testWinnerAgentID, "label": "claude-agent", "reason": "No failed tool calls",
		},
		"best_for_cost": map[string]any{
			"run_agent_id": testRunnerUpAgentID, "label": "gpt-agent", "reason": "Half the token spend",
		},
		"model_summaries": []any{
			map[string]any{"run_agent_id": testWinnerAgentID, "label": "claude-agent", "strongest_dimension": "correctness", "weakest_dimension": "latency", "summary": "Correct and careful."},
			map[string]any{"run_agent_id": testRunnerUpAgentID, "label": "gpt-agent", "strongest_dimension": "cost", "weakest_dimension": "correctness", "summary": "Cheap but misses edge cases."},
		},
		"recommended_next_step": "Run the pack again with three seeds.",
		"confidence_notes":      "The gap is large; confidence is high.",
	}
}

func runInsightsArgs(extra ...string) []string {
	args := []string{"run", "insights", testRunID, "--provider-account", testProviderAccountID, "--model", "gpt-4.1-mini"}
	return append(args, extra...)
}

func TestRunInsightsPostsProviderAccountAndModel(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	var auth string
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: recordJSONBody(t, &body, &auth, http.StatusOK, runInsightsFixture()),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, runInsightsArgs(), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("run insights: %v", err)
	}

	want := map[string]any{"provider_account_id": testProviderAccountID, "model": "gpt-4.1-mini"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("request body = %v, want %v", body, want)
	}
	if auth != "Bearer test-tok" {
		t.Fatalf("Authorization = %q, want the env token only", auth)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	for _, fragment := range []string{
		"claude-agent",
		"Highest correctness on all cases.",
		"No failed tool calls",
		"gpt-agent - Half the token spend",
		"Run the pack again with three seeds.",
		"The gap is large; confidence is high.",
		"openai gpt-4.1-mini-2025-04-14 at 2026-11-24T10:00:00Z",
		"  - Slower than gpt-agent",
		"Cheap but misses edge cases.",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table output does not contain %q:\n%s", fragment, out)
		}
	}
	if strings.Contains(out, "Latency pick") {
		t.Errorf("table output shows a latency pick that the response does not have:\n%s", out)
	}
}

func TestRunInsightsJSONPrintsRawResponse(t *testing.T) {
	isolateCLIEnv(t)
	fixture := runInsightsFixture()
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, runInsightsArgs("--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("run insights --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestRunInsightsRunNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: captureHandler(t, &called, http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "run_not_found", "message": "run not found"},
		}),
	})

	err := executeCommand(t, runInsightsArgs(), srv.URL)
	if !called {
		t.Fatal("the ranking-insights route did not get the request; the 404 came from the fallback route")
	}
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "run_not_found" {
		t.Fatalf("error = %v, want API error run_not_found", err)
	}
	assertExitCode(t, err, exitNotFound)
}

func TestRunInsightsInvalidRunStatusExitsOne(t *testing.T) {
	isolateCLIEnv(t)
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: jsonHandler(http.StatusBadRequest, map[string]any{
			"error": map[string]any{"code": "invalid_run_status", "message": "ranking insights are only available for completed runs"},
		}),
	})

	err := executeCommand(t, runInsightsArgs(), srv.URL)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "invalid_run_status" {
		t.Fatalf("error = %v, want API error invalid_run_status", err)
	}
	assertExitCode(t, err, 1)
}

func TestRunInsightsRateLimitedExits75WithRetryAfter(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60")
			jsonHandler(http.StatusTooManyRequests, map[string]any{
				"error": map[string]any{"code": "ranking_insights_rate_limited", "message": "too many insight generations for this run; retry later"},
			})(w, r)
		},
	})

	err := executeCommand(t, runInsightsArgs(), srv.URL)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "ranking_insights_rate_limited" {
		t.Fatalf("error = %v, want API error ranking_insights_rate_limited", err)
	}
	if apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 60 {
		t.Fatalf("RetryAfterSeconds = %v, want 60", apiErr.RetryAfterSeconds)
	}
	assertExitCode(t, err, exitRetryableFailure)
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (a paid model call must not repeat)", count.Load())
	}
}

func TestRunInsightsServerErrorDoesNotRetry(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: jsonHandler(http.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"code": "ranking_insights_provider_unavailable", "message": "selected provider is temporarily unavailable"},
		}),
	})

	err := executeCommand(t, runInsightsArgs(), srv.URL)
	assertExitCode(t, err, exitRetryableFailure)
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1 (a paid model call must not repeat)", count.Load())
	}
}

func TestRunInsightsTimeoutExits75(t *testing.T) {
	isolateCLIEnv(t)
	previous := longRequestTimeout
	longRequestTimeout = 50 * time.Millisecond
	t.Cleanup(func() { longRequestTimeout = previous })

	release := make(chan struct{})
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-time.After(2 * time.Second):
			}
			jsonHandler(http.StatusOK, runInsightsFixture())(w, r)
		},
	})
	defer close(release)

	err := executeCommand(t, runInsightsArgs(), srv.URL)
	if err == nil {
		t.Fatal("run insights returned no error, want a timeout")
	}
	assertExitCode(t, err, exitRetryableFailure)
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
}

func TestRunInsightsUsesLongerTimeoutThanDefault(t *testing.T) {
	if longRequestTimeout <= 45*time.Second {
		t.Fatalf("longRequestTimeout = %s, want more than the 45s model limit of the server", longRequestTimeout)
	}
}

func TestRunInsightsRejectsInvalidInputBeforeRequest(t *testing.T) {
	cases := map[string][]string{
		"run id":           {"run", "insights", "run-1", "--provider-account", testProviderAccountID, "--model", "m"},
		"provider account": {"run", "insights", testRunID, "--provider-account", "acct-1", "--model", "m"},
		"blank model":      {"run", "insights", testRunID, "--provider-account", testProviderAccountID, "--model", "   "},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, args, srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestRunInsightsRequiresProviderAccountAndModel(t *testing.T) {
	cases := map[string]struct {
		args []string
		flag string
	}{
		"provider account": {[]string{"run", "insights", testRunID, "--model", "m"}, "provider-account"},
		"model":            {[]string{"run", "insights", testRunID, "--provider-account", testProviderAccountID}, "model"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)
			err := executeCommand(t, tc.args, srv.URL)
			if err == nil || !strings.Contains(err.Error(), `required flag(s) "`+tc.flag+`" not set`) {
				t.Fatalf("error = %v, want the required --%s error", err, tc.flag)
			}
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestRunInsightsSanitizesModelText(t *testing.T) {
	isolateCLIEnv(t)
	fixture := runInsightsFixture()
	fixture["why_it_won"] = "\x1b[31mred text\x1b[0m and a bell\x07"
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + runInsightsPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, runInsightsArgs(), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("run insights: %v", err)
	}
	if strings.Contains(out, "\x1b[31m") || strings.Contains(out, "\x07") {
		t.Fatalf("table output kept control bytes from the model text: %q", out)
	}
	if !strings.Contains(out, "red text") {
		t.Fatalf("table output lost the printable model text:\n%s", out)
	}
}
