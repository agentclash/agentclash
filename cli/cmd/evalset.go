package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/evalset"
	"github.com/spf13/cobra"
)

const (
	evalsetExitFailed         = 1
	evalsetExitCancelled      = 1
	evalsetExitBudgetExceeded = 1
)

func init() {
	rootCmd.AddCommand(evalsetCmd)
	evalsetCmd.AddCommand(evalsetSubmitCmd)
	evalsetCmd.AddCommand(evalsetStatusCmd)
	evalsetCmd.AddCommand(evalsetLogsCmd)
	evalsetCmd.AddCommand(evalsetListCmd)
	evalsetCmd.AddCommand(evalsetCancelCmd)
	evalsetCmd.AddCommand(evalsetReportCmd)
	evalsetCmd.AddCommand(evalsetInitCmd)

	evalsetSubmitCmd.Flags().Bool("yes", false, "Skip confirmation after expansion")
	evalsetSubmitCmd.Flags().Bool("dry-run", false, "Expand only; do not create the eval set")
	evalsetSubmitCmd.Flags().Bool("follow", false, "Poll status until the eval set reaches a terminal state")
	evalsetSubmitCmd.Flags().Bool("local", false, "Reserved for agentclash local execution")
	evalsetSubmitCmd.Flags().Duration("poll-interval", 5*time.Second, "Status poll interval when --follow is set")
	evalsetSubmitCmd.Flags().Duration("timeout", 0, "Follow timeout (0 = no limit)")

	evalsetStatusCmd.Flags().Bool("watch", false, "Poll until terminal; TTY shows progress, non-TTY prints status lines")
	evalsetStatusCmd.Flags().Duration("poll-interval", 5*time.Second, "Watch poll interval")
	evalsetStatusCmd.Flags().Duration("timeout", 0, "Watch timeout (0 = no limit)")

	evalsetLogsCmd.Flags().BoolP("follow", "f", false, "Follow event streams until runs finish")
	evalsetLogsCmd.Flags().String("combination", "", "Filter to a single matrix_key")

	evalsetReportCmd.Flags().String("format", "table", "Output format: table, json, or csv")
	evalsetListCmd.Flags().Int("limit", 20, "Max eval sets to list")
}

var evalsetCmd = &cobra.Command{
	Use:   "evalset",
	Short: "Submit and manage multi-pack eval sets from a YAML manifest",
	Long: `Fleet eval-set commands: one manifest (packs × agents × models × repeats)
expands into sessions and runs. Typical loop:

  agentclash evalset submit sweep.yaml --yes --follow
  agentclash evalset status <id> --json
  agentclash evalset report <id> --format csv`,
}

var evalsetSubmitCmd = &cobra.Command{
	Use:   "submit <file.yaml>",
	Short: "Expand a manifest, optionally confirm, and create an eval set",
	Example: `  # Dry-run expansion table
  agentclash evalset submit sweep.yaml --dry-run

  # Submit without prompt and follow to completion
  agentclash evalset submit sweep.yaml --yes --follow

  # CI-friendly: non-zero exit if the set failed or was cancelled
  agentclash evalset submit sweep.yaml --yes --follow --json`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalsetSubmit,
}

var evalsetStatusCmd = &cobra.Command{
	Use:   "status <eval-set-id>",
	Short: "Show eval-set status and roll-up",
	Example: `  agentclash evalset status <id>
  agentclash evalset status <id> --json
  agentclash evalset status <id> --watch`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalsetStatus,
}

var evalsetLogsCmd = &cobra.Command{
	Use:   "logs <eval-set-id>",
	Short: "Tail multiplexed run-event streams for an eval set",
	Example: `  agentclash evalset logs <id> -f
  agentclash evalset logs <id> -f --combination pack/agent/1`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalsetLogs,
}

var evalsetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List eval sets in the current workspace",
	RunE:  runEvalsetList,
}

var evalsetCancelCmd = &cobra.Command{
	Use:   "cancel <eval-set-id>",
	Short: "Cancel an in-flight eval set",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvalsetCancel,
}

var evalsetReportCmd = &cobra.Command{
	Use:   "report <eval-set-id>",
	Short: "Render set-level scorecard (table, json, or csv)",
	Example: `  agentclash evalset report <id>
  agentclash evalset report <id> --format csv > scorecard.csv
  agentclash evalset report <id> --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalsetReport,
}

var evalsetInitCmd = &cobra.Command{
	Use:   "init [file.yaml]",
	Short: "Write a commented starter evalset/v1 manifest",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEvalsetInit,
}

func runEvalsetSubmit(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	wsID := RequireWorkspace(cmd)

	local, _ := cmd.Flags().GetBool("local")
	if local {
		return &cliError{
			Code:    "local_not_available",
			Message: "evalset submit --local requires agentclash local (#1147)",
		}
	}

	path := args[0]
	raw, err := os.ReadFile(path)
	if err != nil {
		return &cliError{Code: "not_found", Message: fmt.Sprintf("read manifest: %v", err)}
	}
	manifest, err := evalset.ParseManifest(raw)
	if err != nil {
		return &cliError{Code: "invalid_argument", Message: err.Error()}
	}
	resolved, err := resolveEvalsetManifest(cmd, rc, wsID, manifest)
	if err != nil {
		return err
	}
	manifestJSON, err := json.Marshal(resolved)
	if err != nil {
		return err
	}

	expandBody := map[string]any{
		"workspace_id": wsID,
		"manifest":     json.RawMessage(manifestJSON),
	}
	resp, err := rc.Client.Post(cmd.Context(), "/v1/eval-sets/expand", expandBody)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	var report evalset.ExpansionReport
	if err := resp.DecodeJSON(&report); err != nil {
		return err
	}
	if !rc.Output.IsStructured() {
		renderEvalsetExpansionTable(rc, report)
	} else if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
		return rc.Output.PrintRaw(report)
	}

	dry, _ := cmd.Flags().GetBool("dry-run")
	if dry {
		if !rc.Output.IsStructured() {
			rc.Output.PrintSuccess(fmt.Sprintf("Dry-run: %d combinations (not submitted)", report.Count))
		}
		return nil
	}

	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && !nonInteractiveMode() && isInteractiveTerminal(rc) {
		fmt.Fprintf(os.Stderr, "Submit %d combinations as an eval set? [y/N] ", report.Count)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" && strings.ToLower(strings.TrimSpace(line)) != "yes" {
			return &cliError{Code: "aborted", Message: "submit cancelled by user"}
		}
	}

	createBody := map[string]any{
		"workspace_id": wsID,
		"manifest":     json.RawMessage(manifestJSON),
	}
	createResp, err := rc.Client.Post(cmd.Context(), "/v1/eval-sets", createBody)
	if err != nil {
		return err
	}
	if apiErr := createResp.ParseError(); apiErr != nil {
		return apiErr
	}
	var created map[string]any
	if err := createResp.DecodeJSON(&created); err != nil {
		return err
	}

	follow, _ := cmd.Flags().GetBool("follow")
	if rc.Output.IsStructured() && !follow {
		return rc.Output.PrintRaw(created)
	}
	if !rc.Output.IsStructured() {
		set := mapObject(created, "eval_set")
		rc.Output.PrintSuccess("Eval set created")
		rc.Output.PrintDetail("ID", mapString(set, "id"))
		rc.Output.PrintDetail("Combinations", mapString(created, "combination_count"))
		rc.Output.PrintDetail("Sessions", fmt.Sprintf("%d", len(mapSlice(created, "eval_session_ids"))))
	}

	if !follow {
		return nil
	}
	setID := mapString(mapObject(created, "eval_set"), "id")
	poll, _ := cmd.Flags().GetDuration("poll-interval")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	final, err := followEvalSet(cmd, rc, setID, timeout, poll)
	if err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		if err := rc.Output.PrintRaw(final); err != nil {
			return err
		}
	} else {
		renderEvalsetStatus(rc, final)
	}
	return evalsetTerminalExit(mapString(mapObject(final, "eval_set"), "status"))
}

func runEvalsetStatus(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	watch, _ := cmd.Flags().GetBool("watch")
	poll, _ := cmd.Flags().GetDuration("poll-interval")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	var result map[string]any
	var err error
	if watch {
		result, err = followEvalSet(cmd, rc, args[0], timeout, poll)
	} else {
		result, err = getEvalSet(cmd, rc, args[0])
	}
	if err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		if err := rc.Output.PrintRaw(result); err != nil {
			return err
		}
	} else {
		renderEvalsetStatus(rc, result)
	}
	return evalsetTerminalExit(mapString(mapObject(result, "eval_set"), "status"))
}

func runEvalsetList(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	wsID := RequireWorkspace(cmd)
	limit, _ := cmd.Flags().GetInt("limit")
	result, err := listEvalSets(cmd, rc, wsID, limit)
	if err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(result)
	}
	renderEvalsetList(rc, result)
	return nil
}

func runEvalsetCancel(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	resp, err := rc.Client.Post(cmd.Context(), "/v1/eval-sets/"+args[0]+"/cancel", nil)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	var set map[string]any
	if err := resp.DecodeJSON(&set); err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(set)
	}
	rc.Output.PrintSuccess("Eval set cancelled")
	rc.Output.PrintDetail("ID", mapString(set, "id"))
	rc.Output.PrintDetail("Status", mapString(set, "status"))
	return nil
}

func runEvalsetReport(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	format, _ := cmd.Flags().GetString("format")
	format = strings.ToLower(strings.TrimSpace(format))
	result, err := getEvalSet(cmd, rc, args[0])
	if err != nil {
		return err
	}
	switch format {
	case "json":
		if err := rc.Output.PrintRaw(result); err != nil {
			return err
		}
	case "csv":
		if err := renderEvalsetReportCSV(rc, result); err != nil {
			return err
		}
	case "table", "":
		renderEvalsetReportTable(rc, result)
	default:
		return &cliError{Code: "invalid_argument", Message: "--format must be table, json, or csv"}
	}
	return evalsetTerminalExit(mapString(mapObject(result, "eval_set"), "status"))
}

func runEvalsetInit(cmd *cobra.Command, args []string) error {
	path := "agentclash.evalset.yaml"
	if len(args) == 1 {
		path = args[0]
	}
	if _, err := os.Stat(path); err == nil {
		return &cliError{Code: "invalid_argument", Message: fmt.Sprintf("%s already exists", path)}
	}
	content := `# agentclash.evalset.yaml — Fleet eval-set manifest (evalset/v1)
# Resolve pack/agent refs with workspace UUIDs, slugs, or catalog/<slug>.
schema: evalset/v1
name: nightly-coding-sweep

packs:
  # challenge_pack_version UUID, workspace pack slug, or catalog/<slug>
  - catalog/code-review

agents:
  # agent_deployment UUID or exact deployment name
  - deployment: claude-opus-5-default
  - deployment: gpt-5-default

# Optional model override axis (leave empty for harness defaults)
models: []

repeats: 5
seeds:
  strategy: auto
limits:
  max_concurrent_runs: 20
  budget_usd: 50   # reserved; enforced by Fleet budgets
case_fanout: true
# Optional post-completion scanners (built-ins in runtime/scanners/catalog)
# scanners:
#   - reward-hacking
#   - sandbox-escape-attempt
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	rc := GetRunContext(cmd)
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(map[string]any{"path": path})
	}
	rc.Output.PrintSuccess("Wrote " + path)
	return nil
}

func evalsetTerminalExit(status string) error {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed":
		return &ExitCodeError{Code: evalsetExitFailed, Message: "eval set failed"}
	case "cancelled", "canceled":
		return &ExitCodeError{Code: evalsetExitCancelled, Message: "eval set cancelled"}
	case string(domain.EvalSetStatusBudgetExceeded):
		return &ExitCodeError{Code: evalsetExitBudgetExceeded, Message: "eval set stopped: budget exceeded"}
	default:
		return nil
	}
}

// evalsetStatusTerminal reports whether an eval set will launch no more work.
// It reads the server's eval-set lifecycle, so budget_exceeded ends a follow
// loop. The single-l "canceled" is accepted defensively, as in
// isTerminalRunStatus.
func evalsetStatusTerminal(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "canceled" {
		return true
	}
	return domain.IsEvalSetTerminal(domain.EvalSetStatus(normalized))
}
