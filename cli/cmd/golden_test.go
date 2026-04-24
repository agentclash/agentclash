package cmd

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/config"
	"github.com/spf13/cobra"
)

// updateGolden regenerates every testdata/help/*.golden when set. Invoke
// via `go test -run Golden ./cmd -update` after an intentional CLI grammar
// change.
var updateGolden = flag.Bool("update", false, "regenerate golden help files")

// TestCLIGrammarGolden catches accidental changes to any command's --help
// output. Every subcommand (leaf or group) is dumped to a deterministic
// buffer and compared against testdata/help/<path>.golden. Grammar changes
// should be conscious — if this fails, re-run with -update and commit the
// diff.
func TestCLIGrammarGolden(t *testing.T) {
	// Freeze version info so the output is deterministic across machines.
	SetVersionInfo("test", "test", "test")

	// Cobra lazy-initializes the `help` and `completion` subcommands the
	// first time rootCmd.Execute() is called. Force both on now so the
	// golden content is identical whether this test runs alone or after
	// another test has already called Execute().
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	specs := collectHelpSpecs(rootCmd, nil)
	if len(specs) == 0 {
		t.Fatal("collectHelpSpecs returned no commands — cobra tree wiring broke")
	}

	goldenDir := filepath.Join("testdata", "help")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", goldenDir, err)
	}

	for _, spec := range specs {
		spec := spec
		t.Run(spec.key, func(t *testing.T) {
			got := renderHelp(t, spec.cmd)
			goldenPath := filepath.Join(goldenDir, spec.key+".golden")

			if *updateGolden {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if os.IsNotExist(err) {
				t.Fatalf(
					"golden missing for %q — rerun with -update to seed it\n"+
						"(first time a golden path is captured, the diff is intentional)",
					spec.key,
				)
			}
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			if string(want) != got {
				t.Fatalf(
					"help output drifted for %q — rerun with -update if intended\n---WANT---\n%s\n---GOT---\n%s",
					spec.key, string(want), got,
				)
			}
		})
	}
}

type helpSpec struct {
	key string
	cmd *cobra.Command
}

// collectHelpSpecs walks the cobra command tree in a deterministic order,
// returning (flat-path-key, *cobra.Command) pairs for every command the CLI
// exposes. Root becomes "agentclash", nested commands become "agentclash_run_open".
func collectHelpSpecs(cmd *cobra.Command, prefix []string) []helpSpec {
	path := append([]string{}, prefix...)
	path = append(path, cmd.Name())
	key := strings.Join(path, "_")
	specs := []helpSpec{{key: key, cmd: cmd}}

	children := append([]*cobra.Command(nil), cmd.Commands()...)
	sort.SliceStable(children, func(i, j int) bool {
		return children[i].Name() < children[j].Name()
	})
	for _, child := range children {
		if child.Hidden {
			continue
		}
		// Skip the auto-generated "help" command — its output is unstable
		// (depends on cobra version) and adds no product value.
		if child.Name() == "help" {
			continue
		}
		specs = append(specs, collectHelpSpecs(child, path)...)
	}
	return specs
}

// renderHelp captures `cmd --help` output into a string without executing
// the command's RunE. Cobra lazy-initializes the per-command `-h/--help`
// flag on first Execute() — we force it here so output is identical
// whether the test runs alone or after another test has already executed
// commands.
func renderHelp(t *testing.T, cmd *cobra.Command) string {
	t.Helper()
	cmd.InitDefaultHelpFlag()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Help(); err != nil {
		t.Fatalf("help render: %v", err)
	}
	// Reattach to os.Stdout/os.Stderr so later tests see the default.
	cmd.SetOut(nil)
	cmd.SetErr(nil)
	return normalizeHelpOutput(buf.String())
}

// normalizeHelpOutput strips trailing whitespace per line so golden files
// don't flap on editor newline behavior.
func normalizeHelpOutput(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	joined := strings.Join(lines, "\n")
	return strings.TrimRight(joined, "\n") + "\n"
}

// TestCLIGrammarGoldenDetectsMissingFiles guards against the guard: if
// collectHelpSpecs returns empty or the ignore list grows unchecked we want
// CI to notice. Cheap sanity check.
func TestCLIGrammarGoldenCoversEveryVisibleCommand(t *testing.T) {
	visible := 0
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Hidden || c.Name() == "help" {
			return
		}
		visible++
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(rootCmd)

	specs := collectHelpSpecs(rootCmd, nil)
	if len(specs) != visible {
		t.Fatalf("golden spec count %d != visible command count %d — collectHelpSpecs is dropping commands", len(specs), visible)
	}
	if visible < 20 {
		t.Fatalf("only %d visible commands — expected > 20 across agentclash tree", visible)
	}
}

// renderAgentsMdGolden seeds the cmd/testdata/agents_md.golden file from a
// fixed input, then diffs it — same -update mechanic as the grammar
// goldens. Catches drift in the emitted AGENTS.md template independently of
// link's integration test (which only asserts a few substrings).
func TestAgentsMdGolden(t *testing.T) {
	goldenPath := filepath.Join("testdata", "agents_md.golden")
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "AGENTS.md")
	cfg := config.ProjectConfig{
		WorkspaceID:   "00000000-0000-4000-8000-000000000001",
		WorkspaceName: "raj-personal",
		OrgID:         "org-1",
		Deployments: map[string]string{
			"gpt-5":      "dep-a",
			"claude-4.7": "dep-b",
			"gemini-3":   "dep-c",
		},
	}
	if err := writeAgentsMd(out, cfg); err != nil {
		t.Fatalf("writeAgentsMd: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read emitted: %v", err)
	}
	got := normalizeAgentsMd(string(raw))

	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		t.Fatalf("golden missing — rerun with -update to seed it")
	}
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(want) != got {
		t.Fatalf("AGENTS.md drifted — rerun with -update if intentional\n---WANT---\n%s\n---GOT---\n%s", want, got)
	}
}

// normalizeAgentsMd strips the timestamp line so the golden is stable. The
// "Generated on" line ends with {{.GeneratedAt}} rendered at now() which we
// can't bake into the golden file.
func normalizeAgentsMd(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "was generated by `agentclash link` on ") {
			line = "_(timestamp stripped for golden comparison)_"
		}
		out = append(out, strings.TrimRight(line, " \t"))
	}
	joined := strings.Join(out, "\n")
	return strings.TrimRight(joined, "\n") + "\n"
}

