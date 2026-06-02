package cmd

import (
	"strings"
	"testing"
)

func TestCompletionEmitsScripts(t *testing.T) {
	cases := []struct {
		shell  string
		needle string
	}{
		{"zsh", "#compdef agentclash"},
		{"bash", "bash completion"},
		{"fish", "complete -c agentclash"},
		{"powershell", "Register-ArgumentCompleter"},
	}
	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			cap := captureStdout(t)
			err := executeCommand(t, []string{"completion", tc.shell}, "http://unused")
			out := cap.finish()
			if err != nil {
				t.Fatalf("completion %s returned error: %v", tc.shell, err)
			}
			if !strings.Contains(out, tc.needle) {
				t.Errorf("completion %s output missing %q", tc.shell, tc.needle)
			}
		})
	}
}

func TestCompletionUnknownShellFails(t *testing.T) {
	err := executeCommand(t, []string{"completion", "fish-shell"}, "http://unused")
	if err == nil {
		t.Fatal("completion with an unknown shell should return a non-nil error")
	}
}

func TestCompletionNoArgFails(t *testing.T) {
	err := executeCommand(t, []string{"completion"}, "http://unused")
	if err == nil {
		t.Fatal("completion with no shell argument should return a non-nil error")
	}
}
