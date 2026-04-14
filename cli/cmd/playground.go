package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(playgroundCmd)
	playgroundCmd.AddCommand(pgListCmd)
	playgroundCmd.AddCommand(pgGetCmd)
	playgroundCmd.AddCommand(pgCreateCmd)
	playgroundCmd.AddCommand(pgUpdateCmd)
	playgroundCmd.AddCommand(pgDeleteCmd)
	playgroundCmd.AddCommand(pgTestCaseCmd)
	playgroundCmd.AddCommand(pgExperimentCmd)

	// Test case subcommands
	pgTestCaseCmd.AddCommand(pgTCListCmd)
	pgTestCaseCmd.AddCommand(pgTCCreateCmd)
	pgTestCaseCmd.AddCommand(pgTCUpdateCmd)
	pgTestCaseCmd.AddCommand(pgTCDeleteCmd)

	// Experiment subcommands
	pgExperimentCmd.AddCommand(pgExpListCmd)
	pgExperimentCmd.AddCommand(pgExpCreateCmd)
	pgExperimentCmd.AddCommand(pgExpBatchCmd)
	pgExperimentCmd.AddCommand(pgExpGetCmd)
	pgExperimentCmd.AddCommand(pgExpResultsCmd)
	pgExperimentCmd.AddCommand(pgExpCompareCmd)

	// Flags
	pgCreateCmd.Flags().String("from-file", "", "JSON file with playground spec")
	pgCreateCmd.Flags().String("name", "", "Playground name")

	pgUpdateCmd.Flags().String("from-file", "", "JSON file with playground spec")

	pgTCCreateCmd.Flags().String("from-file", "", "JSON file with test case spec")
	pgTCUpdateCmd.Flags().String("from-file", "", "JSON file with test case spec")

	pgExpCreateCmd.Flags().String("from-file", "", "JSON file with experiment spec")
	pgExpBatchCmd.Flags().String("from-file", "", "JSON file with batch experiment spec")

	pgExpCompareCmd.Flags().String("baseline", "", "Baseline experiment ID (required)")
	pgExpCompareCmd.Flags().String("candidate", "", "Candidate experiment ID (required)")
	pgExpCompareCmd.MarkFlagRequired("baseline")
	pgExpCompareCmd.MarkFlagRequired("candidate")
}

var playgroundCmd = &cobra.Command{
	Use:     "playground",
	Aliases: []string{"pg"},
	Short:   "Manage playgrounds, test cases, and experiments",
}

// --- Playground CRUD ---

var pgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List playgrounds",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/playgrounds", nil)
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{str(item["id"]), str(item["name"]), str(item["created_at"])}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var pgGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a playground",
	Args:  cobra.ExactArgs(1),
	RunE:  resourceGetCmd("/v1/playgrounds/"),
}

var pgCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a playground",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}
		if name, _ := cmd.Flags().GetString("name"); name != "" {
			body["name"] = name
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/playgrounds", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var pg map[string]any
		if err := resp.DecodeJSON(&pg); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(pg)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created playground %s (%s)", str(pg["name"]), str(pg["id"])))
		return nil
	},
}

var pgUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a playground",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/playgrounds/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var pg map[string]any
		if err := resp.DecodeJSON(&pg); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(pg)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Updated playground %s", args[0]))
		return nil
	},
}

var pgDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a playground",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Delete(cmd.Context(), "/v1/playgrounds/"+args[0])
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Deleted playground %s", args[0]))
		return nil
	},
}

// --- Test Cases ---

var pgTestCaseCmd = &cobra.Command{
	Use:     "test-case",
	Aliases: []string{"tc"},
	Short:   "Manage playground test cases",
}

var pgTCListCmd = &cobra.Command{
	Use:   "list <playgroundId>",
	Short: "List test cases",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/playgrounds/"+args[0]+"/test-cases", nil)
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{str(item["id"]), str(item["name"]), str(item["created_at"])}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var pgTCCreateCmd = &cobra.Command{
	Use:   "create <playgroundId>",
	Short: "Create a test case",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/playgrounds/"+args[0]+"/test-cases", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var tc map[string]any
		if err := resp.DecodeJSON(&tc); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(tc)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created test case %s", str(tc["id"])))
		return nil
	},
}

var pgTCUpdateCmd = &cobra.Command{
	Use:   "update <testCaseId>",
	Short: "Update a test case",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/playground-test-cases/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var tc map[string]any
		if err := resp.DecodeJSON(&tc); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(tc)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Updated test case %s", args[0]))
		return nil
	},
}

var pgTCDeleteCmd = &cobra.Command{
	Use:   "delete <testCaseId>",
	Short: "Delete a test case",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Delete(cmd.Context(), "/v1/playground-test-cases/"+args[0])
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Deleted test case %s", args[0]))
		return nil
	},
}

// --- Experiments ---

var pgExperimentCmd = &cobra.Command{
	Use:     "experiment",
	Aliases: []string{"exp"},
	Short:   "Manage playground experiments",
}

var pgExpListCmd = &cobra.Command{
	Use:   "list <playgroundId>",
	Short: "List experiments",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/playgrounds/"+args[0]+"/experiments", nil)
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

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		cols := []output.Column{{Header: "ID"}, {Header: "Status"}, {Header: "Model"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				output.StatusColor(str(item["status"])),
				str(item["model"]),
				str(item["created_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var pgExpCreateCmd = &cobra.Command{
	Use:   "create <playgroundId>",
	Short: "Create an experiment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/playgrounds/"+args[0]+"/experiments", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var exp map[string]any
		if err := resp.DecodeJSON(&exp); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(exp)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created experiment %s", str(exp["id"])))
		return nil
	},
}

var pgExpBatchCmd = &cobra.Command{
	Use:   "batch <playgroundId>",
	Short: "Create experiments in batch (one per model)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		body, err := loadBodyFromFileOrFlags(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/playgrounds/"+args[0]+"/experiments/batch", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}
		if items, ok := result["items"].([]any); ok {
			rc.Output.PrintSuccess(fmt.Sprintf("Created %d experiments", len(items)))
		}
		return nil
	},
}

var pgExpGetCmd = &cobra.Command{
	Use:   "get <experimentId>",
	Short: "Get an experiment",
	Args:  cobra.ExactArgs(1),
	RunE:  resourceGetCmd("/v1/playground-experiments/"),
}

var pgExpResultsCmd = &cobra.Command{
	Use:   "results <experimentId>",
	Short: "List results for an experiment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), "/v1/playground-experiments/"+args[0]+"/results", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		rc.Output.PrintRaw(result)
		return nil
	},
}

var pgExpCompareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare two experiments",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		baseline, _ := cmd.Flags().GetString("baseline")
		candidate, _ := cmd.Flags().GetString("candidate")

		q := url.Values{}
		q.Set("baseline", baseline)
		q.Set("candidate", candidate)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/playground-experiments/compare", q)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		rc.Output.PrintRaw(result)
		return nil
	},
}

// --- helpers ---

func resourceGetCmd(pathPrefix string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		resp, err := rc.Client.Get(cmd.Context(), pathPrefix+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		rc.Output.PrintRaw(result)
		return nil
	}
}

func loadBodyFromFileOrFlags(cmd *cobra.Command) (map[string]any, error) {
	body := make(map[string]any)
	if fromFile, _ := cmd.Flags().GetString("from-file"); fromFile != "" {
		data, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("parsing file: %w", err)
		}
	}
	return body, nil
}
