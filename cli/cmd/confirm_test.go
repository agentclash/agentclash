package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// largeEffectCommandPaths lists the commands that call confirmLargeEffect.
// Each one must declare its own --yes flag, because the helper reads it.
var largeEffectCommandPaths = []string{
	"agentclash workspace emergency-stop",
	"agentclash share revoke",
	"agentclash challenge-pack draft delete",
	"agentclash connector approve",
	"agentclash connector revoke",
	"agentclash deployment delete",
}

func TestLargeEffectCommandsDeclareYesFlag(t *testing.T) {
	isolateCLIEnv(t)
	for _, path := range largeEffectCommandPaths {
		t.Run(path, func(t *testing.T) {
			args := strings.Fields(strings.TrimPrefix(path, "agentclash "))
			cmd, _, err := rootCmd.Find(args)
			if err != nil || cmd.CommandPath() != path {
				t.Fatalf("command %q not found (err: %v)", path, err)
			}
			flag := cmd.Flags().Lookup("yes")
			if flag == nil || flag.Value.Type() != "bool" || flag.DefValue != "false" {
				t.Fatalf("%q must declare --yes as a bool flag with the default false, got %+v", path, flag)
			}
		})
	}
}

// The confirmation tests use workspace emergency-stop, the first command that
// calls confirmLargeEffect.

func TestConfirmLargeEffect_YesSkipsPrompt(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "n\n")
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + emergencyStopPath: jsonHandler(http.StatusOK, emergencyStopFixture()),
	})

	stderr := captureStderr(t)
	err := executeCommand(t, emergencyStopArgs("--yes"), srv.URL)
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("emergency-stop --yes: %v", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
	if strings.Contains(errOut, "[y/N]") {
		t.Fatalf("stderr shows a prompt although --yes is set:\n%s", errOut)
	}
}

func TestConfirmLargeEffect_NonInteractiveRefuses(t *testing.T) {
	cases := map[string]struct {
		extra []string
		setup func(t *testing.T)
	}{
		"--non-interactive flag": {extra: []string{"--non-interactive"}},
		"AGENTCLASH_NONINTERACTIVE": {setup: func(t *testing.T) {
			t.Setenv("AGENTCLASH_NONINTERACTIVE", "1")
		}},
		"CI=true": {setup: func(t *testing.T) {
			t.Setenv("CI", "true")
		}},
		"no TTY": {setup: func(t *testing.T) {
			previous := stdioIsInteractive
			stdioIsInteractive = func() bool { return false }
			t.Cleanup(func() { stdioIsInteractive = previous })
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			forceInteractiveTTY(t)
			if tc.setup != nil {
				tc.setup(t)
			}
			// A "y" answer proves that the refusal does not read stdin.
			stubConfirmAnswer(t, "y\n")
			srv, count := countingFakeAPI(t, nil)

			stderr := captureStderr(t)
			err := executeCommand(t, emergencyStopArgs(tc.extra...), srv.URL)
			errOut := stderr.finish()
			local := assertCLIErrorCode(t, err, "interactive_input_required")
			assertExitCode(t, err, exitValidationError)
			if !strings.Contains(local.Message, "--yes") || local.Details["flag"] != "--yes" {
				t.Fatalf("error = %q (details %v), want it to name --yes", local.Message, local.Details)
			}
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
			if strings.Contains(errOut, "[y/N]") {
				t.Fatalf("stderr shows a prompt in non-interactive mode:\n%s", errOut)
			}
		})
	}
}

func TestConfirmLargeEffect_YesWorksInNonInteractiveMode(t *testing.T) {
	isolateCLIEnv(t)
	t.Setenv("CI", "true")
	srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + emergencyStopPath: jsonHandler(http.StatusOK, emergencyStopFixture()),
	})

	if err := executeCommand(t, emergencyStopArgs("--non-interactive", "--yes"), srv.URL); err != nil {
		t.Fatalf("emergency-stop --non-interactive --yes: %v", err)
	}
	if count.Load() != 1 {
		t.Fatalf("requests = %d, want 1", count.Load())
	}
}

func TestConfirmLargeEffect_TTYAccepts(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", "  YES \n", "y\r\n"} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			isolateCLIEnv(t)
			forceInteractiveTTY(t)
			stubConfirmAnswer(t, answer)
			srv, count := countingFakeAPI(t, map[string]http.HandlerFunc{
				"POST " + emergencyStopPath: jsonHandler(http.StatusOK, emergencyStopFixture()),
			})

			if err := executeCommand(t, emergencyStopArgs(), srv.URL); err != nil {
				t.Fatalf("emergency-stop with answer %q: %v", answer, err)
			}
			if count.Load() != 1 {
				t.Fatalf("requests = %d, want 1", count.Load())
			}
		})
	}
}

func TestConfirmLargeEffect_TTYDeclines(t *testing.T) {
	for name, answer := range map[string]string{
		"no":         "n\n",
		"empty line": "\n",
		"EOF":        "",
		"y then EOF": "y",
		"other":      "maybe\n",
		"near yes":   "yess\n",
	} {
		t.Run(name, func(t *testing.T) {
			isolateCLIEnv(t)
			forceInteractiveTTY(t)
			stubConfirmAnswer(t, answer)
			srv, count := countingFakeAPI(t, nil)

			err := executeCommand(t, emergencyStopArgs(), srv.URL)
			assertCLIErrorCode(t, err, "aborted")
			assertExitCode(t, err, exitValidationError)
			if count.Load() != 0 {
				t.Fatalf("requests = %d, want 0", count.Load())
			}
		})
	}
}

func TestConfirmLargeEffect_PromptOnStderrInJSONMode(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "y\n")
	fixture := emergencyStopFixture()
	srv, _ := countingFakeAPI(t, map[string]http.HandlerFunc{
		"POST " + emergencyStopPath: jsonHandler(http.StatusOK, fixture),
	})

	stdout := captureStdout(t)
	stderr := captureStderr(t)
	err := executeCommand(t, emergencyStopArgs("--json"), srv.URL)
	out := stdout.finish()
	errOut := stderr.finish()
	if err != nil {
		t.Fatalf("emergency-stop --json: %v", err)
	}
	assertJSONEqual(t, out, fixture)
	if !strings.Contains(errOut, "[y/N]") {
		t.Fatalf("stderr does not show the prompt:\n%s", errOut)
	}
}

func TestConfirmLargeEffect_PromptShowsWhenQuiet(t *testing.T) {
	isolateCLIEnv(t)
	forceInteractiveTTY(t)
	stubConfirmAnswer(t, "n\n")
	srv, _ := countingFakeAPI(t, nil)

	stderr := captureStderr(t)
	// executeCommand sets --quiet.
	err := executeCommand(t, emergencyStopArgs(), srv.URL)
	errOut := stderr.finish()
	assertCLIErrorCode(t, err, "aborted")
	want := "Cancel every active eval set in workspace " + emergencyStopWorkspaceID + "? Standalone runs and agent tryouts continue. [y/N] "
	if !strings.Contains(errOut, want) {
		t.Fatalf("stderr = %q, want the prompt %q", errOut, want)
	}
}
