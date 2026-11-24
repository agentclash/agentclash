package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/agentclash/agentclash/runtime/scanners"
	"github.com/spf13/cobra"
)

// builtinScannerNameValues is the closed set of scanner names that the scan
// workflow accepts (scanners.LookupBuiltIn in the ListScanTargets activity).
// The server starts the workflow for any name, and the workflow then fails
// with no message to the caller, so the CLI checks the names first.
var builtinScannerNameValues = builtinScannerNames()

// findingSeverityValues are the severities of runtime/scanners.
var findingSeverityValues = []string{
	string(scanners.SeverityLow),
	string(scanners.SeverityMedium),
	string(scanners.SeverityHigh),
	string(scanners.SeverityCritical),
}

// findingStatusValues are the statuses that EvalSetManager.UpdateFindingStatus
// accepts (backend/internal/api/scan_findings.go).
var findingStatusValues = []string{"open", "triaged", "dismissed"}

// findingListMaxLimit keeps one response small. The server has no maximum.
const findingListMaxLimit = 500

func builtinScannerNames() []string {
	defs, err := scanners.BuiltIns()
	if err != nil {
		// The catalog is embedded at build time.
		// TestBuiltinScannerNamesMatchCatalog fails on a broken catalog.
		return nil
	}
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	sort.Strings(names)
	return names
}

func init() {
	evalsetCmd.AddCommand(evalsetScanCmd)
	evalsetCmd.AddCommand(evalsetFindingCmd)
	evalsetFindingCmd.AddCommand(evalsetFindingListCmd)
	evalsetFindingCmd.AddCommand(evalsetFindingUpdateCmd)

	evalsetScanCmd.Flags().StringArray("scanner", nil, "Built-in scanner to run; may be repeated (default: the scanners of the eval set manifest)")

	evalsetFindingListCmd.Flags().String("severity", "", "Show only the findings of this severity: low, medium, high or critical")
	evalsetFindingListCmd.Flags().String("status", "", "Show only the findings with this status: open, triaged or dismissed")
	evalsetFindingListCmd.Flags().Int("limit", 100, "Maximum findings to return, newest first (1-500)")

	evalsetFindingUpdateCmd.Flags().String("status", "", "New status of the finding: open, triaged or dismissed")
	evalsetFindingUpdateCmd.MarkFlagRequired("status")
}

var evalsetScanCmd = &cobra.Command{
	Use:   "scan <eval-set-id>",
	Short: "Run the built-in transcript scanners on an eval set",
	Long: `Start a scan of the case transcripts of an eval set.

The scanners do not change the scores. They write findings that you can
triage with 'agentclash evalset finding list' and
'agentclash evalset finding update'.

Built-in scanners: instruction-injection-compliance, reward-hacking,
sandbox-escape-attempt and tool-misuse. instruction-injection-compliance is
an LLM judge: the server sends each case transcript to its judge model. The
other three scanners match patterns.

Without --scanner, the server runs the scanners that the eval set manifest
lists. If the manifest lists no scanner, the scan finds nothing.

The server starts the scan in the background and returns at once. Only one
scan of an eval set can run at a time: a second scan exits 1 with the error
code scan_already_running.

For each scanner that it runs, a new scan keeps the triage status of a
finding that it finds again, and deletes a finding that it does not find
again. The findings of the other scanners do not change.`,
	Example: `  # Run the scanners of the eval set manifest
  agentclash evalset scan <eval-set-id>

  # Run two built-in scanners
  agentclash evalset scan <eval-set-id> --scanner reward-hacking --scanner tool-misuse

  # Look at the findings when the scan is complete
  agentclash evalset finding list <eval-set-id> --severity high`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalsetScan,
}

var evalsetFindingCmd = &cobra.Command{
	Use:   "finding",
	Short: "Triage the scanner findings of an eval set",
	Long: `List the findings that 'agentclash evalset scan' wrote, and change their
triage status (open, triaged or dismissed).`,
}

var evalsetFindingListCmd = &cobra.Command{
	Use:   "list <eval-set-id>",
	Short: "List the scanner findings of an eval set",
	Long: `List the scanner findings of an eval set, newest first.

The table leaves out the evidence text of each finding. Use --json to get the
full findings, with the evidence.

The server returns one page only. If the count of findings is equal to
--limit, more findings can exist: filter with --severity or --status, or pass
a larger --limit (500 maximum).`,
	Example: `  # List the open findings of high severity
  agentclash evalset finding list <eval-set-id> --severity high --status open

  # Get the full findings, with the evidence, as JSON
  agentclash evalset finding list <eval-set-id> --json`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalsetFindingList,
}

var evalsetFindingUpdateCmd = &cobra.Command{
	Use:   "update <finding-id>",
	Short: "Change the triage status of a scanner finding",
	Example: `  # Mark a finding as triaged
  agentclash evalset finding update <finding-id> --status triaged

  # Dismiss a false positive
  agentclash evalset finding update <finding-id> --status dismissed --json`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalsetFindingUpdate,
}

func runEvalsetScan(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	evalSetID := args[0]
	if err := requireUUID("eval set ID", evalSetID); err != nil {
		return err
	}
	requested, _ := cmd.Flags().GetStringArray("scanner")
	names := make([]string, 0, len(requested))
	seen := make(map[string]bool, len(requested))
	for _, raw := range requested {
		name := strings.TrimSpace(raw)
		if err := requireOneOf("scanner", name, builtinScannerNameValues); err != nil {
			return err
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	// With no --scanner, send no body: the server then uses the manifest.
	var body any
	if len(names) > 0 {
		body = map[string]any{"scanners": names}
	}
	resp, err := rc.Client.Post(cmd.Context(), "/v1/eval-sets/"+evalSetID+"/scan", body)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return fmt.Errorf("decoding scan response: %w", err)
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(result)
	}

	rc.Output.PrintSuccess("Scan started for eval set " + mapString(result, "eval_set_id"))
	if len(names) == 0 {
		rc.Output.PrintDetail("Scanners", "the scanners of the eval set manifest")
	} else {
		rc.Output.PrintDetail("Scanners", strings.Join(names, ", "))
	}
	rc.Output.PrintDetail("Status", mapString(result, "status"))
	rc.Output.PrintDetail("Next", "agentclash evalset finding list "+evalSetID)
	return nil
}

func runEvalsetFindingList(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	evalSetID := args[0]
	if err := requireUUID("eval set ID", evalSetID); err != nil {
		return err
	}
	severity, _ := cmd.Flags().GetString("severity")
	status, _ := cmd.Flags().GetString("status")
	limit, _ := cmd.Flags().GetInt("limit")
	// The server returns an empty list for an unknown filter value, so a typo
	// looks like "no findings". Check the values first. Check a flag that is
	// set to an empty value too: an empty shell variable must not remove the
	// filter and list all findings.
	if cmd.Flags().Changed("severity") {
		if err := requireOneOf("severity", severity, findingSeverityValues); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("status") {
		if err := requireOneOf("status", status, findingStatusValues); err != nil {
			return err
		}
	}
	if limit < 1 || limit > findingListMaxLimit {
		return &cliError{
			Code:    "invalid_argument",
			Message: fmt.Sprintf("--limit must be from 1 to %d, got %d", findingListMaxLimit, limit),
			Details: map[string]any{"flag": "--limit", "value": limit},
		}
	}

	query := url.Values{}
	if severity != "" {
		query.Set("severity", severity)
	}
	if status != "" {
		query.Set("status", status)
	}
	query.Set("limit", strconv.Itoa(limit))
	resp, err := rc.Client.Get(cmd.Context(), "/v1/eval-sets/"+evalSetID+"/findings", query)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return fmt.Errorf("decoding findings response: %w", err)
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(result)
	}

	findings := mapSlice(result, "findings")
	rows := make([][]string, 0, len(findings))
	for _, item := range findings {
		finding, _ := item.(map[string]any)
		rows = append(rows, []string{
			mapString(finding, "id"),
			mapString(finding, "severity"),
			mapString(finding, "status"),
			mapString(finding, "scanner"),
			mapString(finding, "category"),
			mapString(finding, "matrix_key"),
			mapString(finding, "case_key"),
			formatFindingConfidence(finding["confidence"]),
		})
	}
	rc.Output.PrintTable(findingTableColumns, rows)
	if len(findings) == limit {
		rc.Output.PrintWarning(fmt.Sprintf("The list shows %d findings, which is the --limit value. More findings can exist: filter with --severity or --status, or pass a larger --limit (%d maximum).", limit, findingListMaxLimit))
	}
	if len(findings) > 0 {
		rc.Output.PrintDetail("Next", "agentclash evalset finding update <finding-id> --status triaged")
	}
	return nil
}

func runEvalsetFindingUpdate(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	findingID := args[0]
	if err := requireUUID("finding ID", findingID); err != nil {
		return err
	}
	status, _ := cmd.Flags().GetString("status")
	if err := requireOneOf("status", status, findingStatusValues); err != nil {
		return err
	}

	resp, err := rc.Client.Patch(cmd.Context(), "/v1/scan-findings/"+findingID, map[string]any{"status": status})
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	var finding map[string]any
	if err := resp.DecodeJSON(&finding); err != nil {
		return fmt.Errorf("decoding finding response: %w", err)
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(finding)
	}

	rc.Output.PrintSuccess(fmt.Sprintf("Finding %s is now %s", mapString(finding, "id"), mapString(finding, "status")))
	rc.Output.PrintDetail("Scanner", mapString(finding, "scanner"))
	rc.Output.PrintDetail("Severity", mapString(finding, "severity"))
	rc.Output.PrintDetail("Eval set", mapString(finding, "eval_set_id"))
	rc.Output.PrintDetail("Updated at", mapString(finding, "status_updated_at"))
	return nil
}

var findingTableColumns = []output.Column{
	{Header: "ID"},
	{Header: "Severity"},
	{Header: "Status"},
	{Header: "Scanner"},
	{Header: "Category"},
	{Header: "Matrix"},
	{Header: "Case"},
	{Header: "Confidence", Align: 1},
}

func formatFindingConfidence(value any) string {
	if confidence, ok := value.(float64); ok {
		return strconv.FormatFloat(confidence, 'f', 2, 64)
	}
	return str(value)
}
