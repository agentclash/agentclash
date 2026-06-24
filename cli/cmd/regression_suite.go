package cmd

import (
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(regressionSuiteCmd)
	regressionSuiteCmd.AddCommand(regressionSuiteListCmd)
	regressionSuiteCmd.AddCommand(regressionSuiteGetCmd)
	regressionSuiteCmd.AddCommand(regressionSuiteCreateCmd)
	regressionSuiteCmd.AddCommand(regressionSuiteUpdateCmd)
	regressionSuiteCmd.AddCommand(regressionSuiteCasesCmd)
	regressionSuiteCmd.AddCommand(regressionCaseCmd)
	regressionCaseCmd.AddCommand(regressionCaseUpdateCmd)
	regressionCaseCmd.AddCommand(regressionCaseCaptureProductionCmd)

	regressionSuiteCreateCmd.Flags().String("from-file", "", "JSON file with regression suite create payload")
	regressionSuiteCreateCmd.Flags().String("source-challenge-pack-id", "", "Source challenge pack ID")
	regressionSuiteCreateCmd.Flags().String("name", "", "Suite name")
	regressionSuiteCreateCmd.Flags().String("description", "", "Suite description")
	regressionSuiteCreateCmd.Flags().String("default-gate-severity", "", "Default gate severity: info, warning, or blocking")

	regressionSuiteUpdateCmd.Flags().String("from-file", "", "JSON file with regression suite patch payload")
	regressionSuiteUpdateCmd.Flags().String("name", "", "Suite name")
	regressionSuiteUpdateCmd.Flags().String("description", "", "Suite description")
	regressionSuiteUpdateCmd.Flags().String("status", "", "Suite status: active or archived")
	regressionSuiteUpdateCmd.Flags().String("default-gate-severity", "", "Default gate severity: info, warning, or blocking")

	regressionCaseUpdateCmd.Flags().String("from-file", "", "JSON file with regression case patch payload")
	regressionCaseUpdateCmd.Flags().String("title", "", "Case title")
	regressionCaseUpdateCmd.Flags().String("description", "", "Case description")
	regressionCaseUpdateCmd.Flags().String("status", "", "Case status: proposed, active, muted, archived, or rejected")
	regressionCaseUpdateCmd.Flags().String("severity", "", "Case severity: info, warning, or blocking")

	regressionCaseCaptureProductionCmd.Flags().String("from-file", "", "JSON file with production failure capture payload")
	regressionCaseCaptureProductionCmd.Flags().String("source-challenge-pack-version-id", "", "Source challenge pack version ID")
	regressionCaseCaptureProductionCmd.Flags().String("source-challenge-input-set-id", "", "Source challenge input set ID")
	regressionCaseCaptureProductionCmd.Flags().String("source-challenge-identity-id", "", "Source challenge identity ID")
	regressionCaseCaptureProductionCmd.Flags().String("source-case-key", "", "Source production case or incident key")
	regressionCaseCaptureProductionCmd.Flags().String("source-item-key", "", "Source item key")
	regressionCaseCaptureProductionCmd.Flags().String("title", "", "Regression case title")
	regressionCaseCaptureProductionCmd.Flags().String("failure-summary", "", "Failure summary")
	regressionCaseCaptureProductionCmd.Flags().String("failure-class", "", "Failure class")
	regressionCaseCaptureProductionCmd.Flags().String("evidence-tier", "", "Evidence tier")
	regressionCaseCaptureProductionCmd.Flags().String("severity", "", "Case severity: info, warning, or blocking")
	regressionCaseCaptureProductionCmd.Flags().String("promotion-mode", "", "Promotion mode: full_executable, output_only, or manual")
	regressionCaseCaptureProductionCmd.Flags().String("incident-id", "", "Production incident ID")
	regressionCaseCaptureProductionCmd.Flags().String("external-url", "", "Production incident URL")
	regressionCaseCaptureProductionCmd.Flags().String("source", "", "Production source label")
	regressionCaseCaptureProductionCmd.Flags().String("observed-at", "", "Production observation timestamp (RFC3339)")
}

var regressionSuiteCmd = &cobra.Command{
	Use:     "regression-suite",
	Aliases: []string{"regression-suites"},
	Short:   "Manage regression suites and cases",
}

var regressionSuiteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List regression suites",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/regression-suites", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result struct {
			Items []map[string]any `json:"items"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		renderRegressionSuitesTable(rc, result.Items)
		return nil
	},
}

var regressionSuiteGetCmd = &cobra.Command{
	Use:   "get <suiteId>",
	Short: "Get a regression suite",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/regression-suites/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var suite map[string]any
		if err := resp.DecodeJSON(&suite); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(suite)
		}
		renderRegressionSuiteDetail(rc, suite)
		return nil
	},
}

var regressionSuiteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a regression suite",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}
		setFlagIfChanged(cmd, body, "source-challenge-pack-id", "source_challenge_pack_id")
		setFlagIfChanged(cmd, body, "name", "name")
		setFlagIfChanged(cmd, body, "description", "description")
		setFlagIfChanged(cmd, body, "default-gate-severity", "default_gate_severity")

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/regression-suites", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var suite map[string]any
		if err := resp.DecodeJSON(&suite); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(suite)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created regression suite %s (%s)", str(suite["name"]), str(suite["id"])))
		return nil
	},
}

var regressionSuiteUpdateCmd = &cobra.Command{
	Use:   "update <suiteId>",
	Short: "Update a regression suite",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}
		setFlagIfChanged(cmd, body, "name", "name")
		setFlagIfChanged(cmd, body, "description", "description")
		setFlagIfChanged(cmd, body, "status", "status")
		setFlagIfChanged(cmd, body, "default-gate-severity", "default_gate_severity")

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/workspaces/"+wsID+"/regression-suites/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var suite map[string]any
		if err := resp.DecodeJSON(&suite); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(suite)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Updated regression suite %s", args[0]))
		return nil
	},
}

var regressionSuiteCasesCmd = &cobra.Command{
	Use:   "cases <suiteId>",
	Short: "List regression cases in a suite",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/regression-suites/"+args[0]+"/cases", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result struct {
			Items []map[string]any `json:"items"`
		}
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		renderRegressionCasesTable(rc, result.Items)
		return nil
	},
}

var regressionCaseCmd = &cobra.Command{
	Use:   "case",
	Short: "Manage individual regression cases",
}

var regressionCaseUpdateCmd = &cobra.Command{
	Use:   "update <caseId>",
	Short: "Update a regression case",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}
		setFlagIfChanged(cmd, body, "title", "title")
		setFlagIfChanged(cmd, body, "description", "description")
		setFlagIfChanged(cmd, body, "status", "status")
		setFlagIfChanged(cmd, body, "severity", "severity")

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/workspaces/"+wsID+"/regression-cases/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var regressionCase map[string]any
		if err := resp.DecodeJSON(&regressionCase); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(regressionCase)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Updated regression case %s", args[0]))
		return nil
	},
}

var regressionCaseCaptureProductionCmd = &cobra.Command{
	Use:   "capture-production <suiteId>",
	Short: "Capture a production failure as a proposed regression case",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}
		setFlagIfChanged(cmd, body, "source-challenge-pack-version-id", "source_challenge_pack_version_id")
		setFlagIfChanged(cmd, body, "source-challenge-input-set-id", "source_challenge_input_set_id")
		setFlagIfChanged(cmd, body, "source-challenge-identity-id", "source_challenge_identity_id")
		setFlagIfChanged(cmd, body, "source-case-key", "source_case_key")
		setFlagIfChanged(cmd, body, "source-item-key", "source_item_key")
		setFlagIfChanged(cmd, body, "title", "title")
		setFlagIfChanged(cmd, body, "failure-summary", "failure_summary")
		setFlagIfChanged(cmd, body, "failure-class", "failure_class")
		setFlagIfChanged(cmd, body, "evidence-tier", "evidence_tier")
		setFlagIfChanged(cmd, body, "severity", "severity")
		setFlagIfChanged(cmd, body, "promotion-mode", "promotion_mode")
		setFlagIfChanged(cmd, body, "incident-id", "incident_id")
		setFlagIfChanged(cmd, body, "external-url", "external_url")
		setFlagIfChanged(cmd, body, "source", "source")
		setFlagIfChanged(cmd, body, "observed-at", "observed_at")

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/regression-suites/"+args[0]+"/production-failures", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var regressionCase map[string]any
		if err := resp.DecodeJSON(&regressionCase); err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(regressionCase)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Captured production failure as regression case %s", str(regressionCase["id"])))
		return nil
	},
}

func renderRegressionSuitesTable(rc *RunContext, items []map[string]any) {
	cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}, {Header: "Cases"}, {Header: "Severity"}, {Header: "Created"}}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{
			str(item["id"]),
			str(item["name"]),
			output.StatusColor(str(item["status"])),
			str(item["case_count"]),
			str(item["default_gate_severity"]),
			str(item["created_at"]),
		}
	}
	rc.Output.PrintTable(cols, rows)
}

func renderRegressionCasesTable(rc *RunContext, items []map[string]any) {
	cols := []output.Column{{Header: "ID"}, {Header: "Title"}, {Header: "Status"}, {Header: "Severity"}, {Header: "Class"}, {Header: "Created"}}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = []string{
			str(item["id"]),
			str(item["title"]),
			output.StatusColor(str(item["status"])),
			str(item["severity"]),
			str(item["failure_class"]),
			str(item["created_at"]),
		}
	}
	rc.Output.PrintTable(cols, rows)
}

func renderRegressionSuiteDetail(rc *RunContext, suite map[string]any) {
	rc.Output.PrintDetail("ID", str(suite["id"]))
	rc.Output.PrintDetail("Name", str(suite["name"]))
	rc.Output.PrintDetail("Status", output.StatusColor(str(suite["status"])))
	rc.Output.PrintDetail("Source Challenge Pack", str(suite["source_challenge_pack_id"]))
	rc.Output.PrintDetail("Default Gate Severity", str(suite["default_gate_severity"]))
	rc.Output.PrintDetail("Cases", str(suite["case_count"]))
	rc.Output.PrintDetail("Created", str(suite["created_at"]))
}
