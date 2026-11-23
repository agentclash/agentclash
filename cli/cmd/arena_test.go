package cmd

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

const (
	arenaTasksPath   = "/v1/workspaces/ws-1/arena/tasks"
	arenaVotesPath   = "/v1/workspaces/ws-1/arena/votes"
	arenaTaskID      = "aaaaaaaa-1111-4111-8111-111111111111"
	arenaLeftID      = "22222222-2222-4222-8222-222222222222"
	arenaRightID     = "33333333-3333-4333-8333-333333333333"
	arenaOtherTaskID = "44444444-4444-4444-8444-444444444444"
)

func arenaTaskFixture() map[string]any {
	return map[string]any{
		"id":                 arenaTaskID,
		"case_key":           "refund-recovery",
		"left_run_agent_id":  arenaLeftID,
		"right_run_agent_id": arenaRightID,
		"status":             "pending",
	}
}

func arenaVoteArgs(taskID string, extra ...string) []string {
	return append([]string{"arena", "vote", taskID, "-w", "ws-1"}, extra...)
}

func TestArenaTasksListsPendingTasks(t *testing.T) {
	isolateCLIEnv(t)
	task := arenaTaskFixture()
	task["case_key"] = "\x1b[31mrefund-recovery\x1b[0m\nForged row"
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + arenaTasksPath: jsonHandler(http.StatusOK, map[string]any{"items": []any{task}}),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"arena", "tasks", "-w", "ws-1"}, srv.URL)
	out := stdout.finish()
	if err != nil || count.Load() != 1 {
		t.Fatalf("arena tasks: %v, requests = %d, want one GET", err, count.Load())
	}
	for _, fragment := range []string{arenaTaskID, "refund-recovery", arenaLeftID, arenaRightID, "pending"} {
		if !strings.Contains(out, fragment) {
			t.Errorf("table does not contain %q:\n%s", fragment, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("table contains a control sequence: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Forged") {
			t.Fatalf("a newline in a server value made a separate row:\n%s", out)
		}
	}
}

func TestArenaTasksJSONIsRaw(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"items": []any{arenaTaskFixture()}, "extra": true}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + arenaTasksPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, []string{"arena", "tasks", "-w", "ws-1", "--json"}, srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("arena tasks --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}

func TestArenaTasksFullPageWarns(t *testing.T) {
	isolateCLIEnv(t)
	items := make([]any, 20)
	for i := range items {
		items[i] = arenaTaskFixture()
	}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + arenaTasksPath: jsonHandler(http.StatusOK, map[string]any{"items": items}),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommandWithQuiet(t, []string{"arena", "tasks", "-w", "ws-1"}, srv.URL, false)
	stdout.finish()
	out := stderr.finish()
	if err != nil {
		t.Fatalf("arena tasks: %v", err)
	}
	if !strings.Contains(out, "20 tasks") {
		t.Errorf("stderr = %q, want a warning at the 20-task limit", out)
	}
}

func TestArenaTasksNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + arenaTasksPath: captureHandler(t, &called, http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "not_found", "message": "workspace not found"},
		}),
	})

	err := executeCommand(t, []string{"arena", "tasks", "-w", "ws-1"}, srv.URL)
	if !called || count.Load() != 1 {
		t.Fatalf("GET handler called = %v, requests = %d, want one GET", called, count.Load())
	}
	assertExitCode(t, err, exitNotFound)
}

func TestArenaTasksRejectsExtraArg(t *testing.T) {
	isolateCLIEnv(t)
	srv, count := countingFakeAPI(t, nil)

	err := executeCommand(t, []string{"arena", "tasks", arenaTaskID, "-w", "ws-1"}, srv.URL)
	if err == nil {
		t.Fatal("arena tasks accepted an extra argument")
	}
	if count.Load() != 0 {
		t.Fatalf("requests = %d, want 0", count.Load())
	}
}

func TestArenaVoteResolvesLeftSide(t *testing.T) {
	isolateCLIEnv(t)
	var gotTasks atomic.Bool
	var body map[string]any
	tasks := jsonHandler(http.StatusOK, map[string]any{"items": []any{arenaTaskFixture()}})
	record := recordJSONBody(t, &body, nil, http.StatusCreated, map[string]any{"status": "recorded"})
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + arenaTasksPath: func(w http.ResponseWriter, r *http.Request) {
			gotTasks.Store(true)
			tasks(w, r)
		},
		"POST " + arenaVotesPath: func(w http.ResponseWriter, r *http.Request) {
			if !gotTasks.Load() {
				t.Error("POST ran before GET")
			}
			record(w, r)
		},
	})

	args := arenaVoteArgs(arenaTaskID, "--winner", "left", "--rubric-score", "recovery=4.5", "--rubric-score", " clarity = 2", "--rubric-score", "status=refunded=4")
	err := executeCommand(t, args, srv.URL)
	want := map[string]any{
		"task_id":             arenaTaskID,
		"winner_run_agent_id": arenaLeftID,
		"rubric_scores":       map[string]any{"recovery": 4.5, "clarity": float64(2), "status=refunded": float64(4)},
	}
	if err != nil || count.Load() != 2 || !reflect.DeepEqual(body, want) {
		t.Fatalf("arena vote: %v, requests = %d, POST body = %v, want GET then POST with %v", err, count.Load(), body, want)
	}
}

func TestArenaVoteResolvesRightSide(t *testing.T) {
	isolateCLIEnv(t)
	other := arenaTaskFixture()
	other["id"] = arenaOtherTaskID
	other["right_run_agent_id"] = arenaLeftID
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + arenaTasksPath:  jsonHandler(http.StatusOK, map[string]any{"items": []any{other, arenaTaskFixture()}}),
		"POST " + arenaVotesPath: recordJSONBody(t, &body, nil, http.StatusCreated, map[string]any{"status": "recorded"}),
	})

	err := executeCommand(t, arenaVoteArgs(strings.ToUpper(arenaTaskID), "--winner", "right"), srv.URL)
	if err != nil || count.Load() != 2 || body["winner_run_agent_id"] != arenaRightID {
		t.Fatalf("arena vote: %v, requests = %d, POST body = %v, want the right run agent", err, count.Load(), body)
	}
}

func TestArenaVoteWithWinnerRunAgentSkipsLookup(t *testing.T) {
	isolateCLIEnv(t)
	var body map[string]any
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + arenaVotesPath: recordJSONBody(t, &body, nil, http.StatusCreated, map[string]any{"status": "recorded"}),
	})

	err := executeCommand(t, arenaVoteArgs(arenaTaskID, "--winner-run-agent", arenaRightID), srv.URL)
	want := map[string]any{"task_id": arenaTaskID, "winner_run_agent_id": arenaRightID}
	if err != nil || count.Load() != 1 || !reflect.DeepEqual(body, want) {
		t.Fatalf("arena vote: %v, requests = %d, POST body = %v, want one POST with %v", err, count.Load(), body, want)
	}
}

func TestArenaVoteTaskNotInPendingListExits64(t *testing.T) {
	isolateCLIEnv(t)
	var posted bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"GET " + arenaTasksPath:  jsonHandler(http.StatusOK, map[string]any{"items": []any{arenaTaskFixture()}}),
		"POST " + arenaVotesPath: captureHandler(t, &posted, http.StatusCreated, map[string]any{"status": "recorded"}),
	})

	err := executeCommand(t, arenaVoteArgs(arenaOtherTaskID, "--winner", "left"), srv.URL)
	local := assertCLIErrorCode(t, err, "invalid_argument")
	assertExitCode(t, err, exitValidationError)
	if posted || count.Load() != 1 {
		t.Fatalf("POST called = %v, requests = %d, want one GET and no POST", posted, count.Load())
	}
	if !strings.Contains(local.NextStep, "--winner-run-agent") || !strings.Contains(err.Error(), "--winner-run-agent") {
		t.Errorf("error = %v, next step = %q, want --winner-run-agent in both", err, local.NextStep)
	}
}

func TestArenaVoteRequiresExactlyOneWinnerFlag(t *testing.T) {
	for name, flags := range map[string][]string{
		"none": nil,
		"both": {"--winner", "left", "--winner-run-agent", arenaRightID},
	} {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)

			err := executeCommand(t, arenaVoteArgs(arenaTaskID, flags...), srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestArenaVoteRejectsBadRubricScore(t *testing.T) {
	for name, scores := range map[string][]string{
		"no value":      {"recovery"},
		"no key":        {"=2"},
		"blank key":     {" =2"},
		"not a number":  {"recovery=high"},
		"NaN":           {"recovery=NaN"},
		"Inf":           {"recovery=Inf"},
		"duplicate key": {"recovery=4", " recovery =5"},
	} {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			srv, count := countingFakeAPI(t, nil)

			args := arenaVoteArgs(arenaTaskID, "--winner", "left")
			for _, score := range scores {
				args = append(args, "--rubric-score", score)
			}
			err := executeCommand(t, args, srv.URL)
			assertCLIErrorCode(t, err, "invalid_argument")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestArenaVoteRejectsBadIDsAndSide(t *testing.T) {
	for name, args := range map[string][]string{
		"task ID":   arenaVoteArgs("bad-id", "--winner", "left"),
		"winner ID": arenaVoteArgs(arenaTaskID, "--winner-run-agent", "bad-id"),
		"side":      arenaVoteArgs(arenaTaskID, "--winner", "middle"),
	} {
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

func TestArenaVoteNotPendingExits1(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + arenaVotesPath: captureHandler(t, &called, http.StatusConflict, map[string]any{
			"error": map[string]any{"code": "arena_task_not_pending", "message": "arena task is not pending"},
		}),
	})

	err := executeCommand(t, arenaVoteArgs(arenaTaskID, "--winner-run-agent", arenaLeftID), srv.URL)
	if !called || count.Load() != 1 {
		t.Fatalf("POST called = %v, requests = %d, want one POST", called, count.Load())
	}
	assertExitCode(t, err, 1)
	var apiErr *cliapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "arena_task_not_pending" {
		t.Fatalf("error = %v, want the arena_task_not_pending API error", err)
	}
}

func TestArenaVoteTaskNotFoundExits66(t *testing.T) {
	isolateCLIEnv(t)
	var called bool
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + arenaVotesPath: captureHandler(t, &called, http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "not_found", "message": "arena task not found"},
		}),
	})

	err := executeCommand(t, arenaVoteArgs(arenaTaskID, "--winner-run-agent", arenaLeftID), srv.URL)
	if !called || count.Load() != 1 {
		t.Fatalf("POST called = %v, requests = %d, want one POST", called, count.Load())
	}
	assertExitCode(t, err, exitNotFound)
}

func TestArenaVoteJSONPrintsServerBody(t *testing.T) {
	isolateCLIEnv(t)
	fixture := map[string]any{"status": "recorded", "extra": "server value"}
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + arenaVotesPath: jsonHandler(http.StatusCreated, fixture),
	})

	stdout := captureStdout(t)
	err := executeCommand(t, arenaVoteArgs(arenaTaskID, "--winner-run-agent", arenaLeftID, "--json"), srv.URL)
	out := stdout.finish()
	if err != nil {
		t.Fatalf("arena vote --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
}
