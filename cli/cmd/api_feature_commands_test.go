package cmd

import (
	"strings"
	"testing"
)

// apiFeatureCommandPaths lists the commands that issue #1418 added for API
// features that had no CLI command. Each one must have a one-line Short text
// and at least one example, because agents read both from --help and from
// `agentclash schema`.
var apiFeatureCommandPaths = []string{
	"agentclash dataset baseline create",
	"agentclash dataset baseline list",
	"agentclash run insights",
	"agentclash ci setup",
	"agentclash workspace emergency-stop",
	"agentclash evalset scan",
	"agentclash evalset finding list",
	"agentclash evalset finding update",
	"agentclash share create",
	"agentclash share update",
	"agentclash share revoke",
}

func TestAPIFeatureCommandsHaveShortAndExample(t *testing.T) {
	for _, path := range apiFeatureCommandPaths {
		t.Run(path, func(t *testing.T) {
			args := strings.Fields(strings.TrimPrefix(path, "agentclash "))
			cmd, _, err := rootCmd.Find(args)
			if err != nil || cmd.CommandPath() != path {
				t.Fatalf("command %q not found (err: %v)", path, err)
			}
			if cmd.Short == "" || strings.Contains(cmd.Short, "\n") || strings.HasSuffix(cmd.Short, ".") {
				t.Fatalf("Short = %q, want one line with no full stop", cmd.Short)
			}
			if !strings.Contains(cmd.Example, "agentclash "+strings.Join(args, " ")) {
				t.Fatalf("Example does not show %q:\n%s", path, cmd.Example)
			}
			if cmd.RunE == nil {
				t.Fatalf("%q is not runnable", path)
			}
		})
	}
}
