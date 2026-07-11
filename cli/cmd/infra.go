package cmd

import (
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

// infraResource describes a workspace-scoped infrastructure resource for the command factory.
type infraResource struct {
	Name        string // e.g. "runtime-profile"
	Plural      string // e.g. "runtime-profiles"
	ListPath    string // e.g. "/v1/workspaces/%s/runtime-profiles"
	CreatePath  string // same as ListPath for POST
	GetPath     string // e.g. "/v1/runtime-profiles/%s" (empty = no get command)
	DeletePath  string // e.g. "/v1/runtime-profiles/%s" (empty = no delete command)
	ArchivePath string // e.g. "/v1/runtime-profiles/%s/archive" (empty = no archive command)
	Columns     []output.Column
	RowMapper   func(map[string]any) []string
}

var infraResources = []infraResource{
	{
		Name: "runtime-profile", Plural: "runtime-profiles",
		ListPath: "/v1/workspaces/%s/runtime-profiles", CreatePath: "/v1/workspaces/%s/runtime-profiles",
		GetPath: "/v1/runtime-profiles/%s", ArchivePath: "/v1/runtime-profiles/%s/archive",
		Columns: []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Target"}, {Header: "Max Iter"}, {Header: "Created"}},
		RowMapper: func(item map[string]any) []string {
			return []string{str(item["id"]), str(item["name"]), str(item["execution_target"]), str(item["max_iterations"]), str(item["created_at"])}
		},
	},
	{
		Name: "provider-account", Plural: "provider-accounts",
		ListPath: "/v1/workspaces/%s/provider-accounts", CreatePath: "/v1/workspaces/%s/provider-accounts",
		GetPath: "/v1/provider-accounts/%s", DeletePath: "/v1/provider-accounts/%s",
		Columns: []output.Column{{Header: "ID"}, {Header: "Provider"}, {Header: "Name"}, {Header: "Status"}, {Header: "Created"}},
		RowMapper: func(item map[string]any) []string {
			return []string{str(item["id"]), str(item["provider_key"]), str(item["name"]), output.StatusColor(str(item["status"])), str(item["created_at"])}
		},
	},
	{
		Name: "tool", Plural: "tools",
		ListPath: "/v1/workspaces/%s/tools", CreatePath: "/v1/workspaces/%s/tools",
		GetPath: "/v1/tools/%s",
		Columns: []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Kind"}, {Header: "Status"}, {Header: "Created"}},
		RowMapper: func(item map[string]any) []string {
			return []string{str(item["id"]), str(item["name"]), str(item["tool_kind"]), output.StatusColor(str(item["lifecycle_status"])), str(item["created_at"])}
		},
	},
	{
		Name: "routing-policy", Plural: "routing-policies",
		ListPath: "/v1/workspaces/%s/routing-policies", CreatePath: "/v1/workspaces/%s/routing-policies",
		Columns: []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Kind"}, {Header: "Created"}},
		RowMapper: func(item map[string]any) []string {
			return []string{str(item["id"]), str(item["name"]), str(item["policy_kind"]), str(item["created_at"])}
		},
	},
	{
		Name: "spend-policy", Plural: "spend-policies",
		ListPath: "/v1/workspaces/%s/spend-policies", CreatePath: "/v1/workspaces/%s/spend-policies",
		Columns: []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Window"}, {Header: "Hard Limit"}, {Header: "Created"}},
		RowMapper: func(item map[string]any) []string {
			return []string{str(item["id"]), str(item["name"]), str(item["window_kind"]), fmtScore(item["hard_limit"]), str(item["created_at"])}
		},
	},
}

func init() {
	rootCmd.AddCommand(infraCmd)

	// Generate CRUD subcommands for each workspace-scoped resource.
	for _, res := range infraResources {
		infraCmd.AddCommand(newInfraResourceCmd(res))
	}
}

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage infrastructure resources",
	Long:  "Manage runtime profiles, provider accounts,\ntools, routing policies, and spend policies.",
}

func newInfraResourceCmd(res infraResource) *cobra.Command {
	parent := &cobra.Command{
		Use:   res.Name,
		Short: fmt.Sprintf("Manage %s", res.Plural),
	}

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s", res.Plural),
		RunE: func(cmd *cobra.Command, args []string) error {
			rc := GetRunContext(cmd)
			wsID := RequireWorkspace(cmd)
			path := fmt.Sprintf(res.ListPath, wsID)

			resp, err := rc.Client.Get(cmd.Context(), path, nil)
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

			rows := make([][]string, len(result.Items))
			for i, item := range result.Items {
				rows[i] = res.RowMapper(item)
			}
			rc.Output.PrintTable(res.Columns, rows)
			return nil
		},
	}
	parent.AddCommand(listCmd)

	// create
	createCmd := &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create a %s", res.Name),
		RunE: func(cmd *cobra.Command, args []string) error {
			rc := GetRunContext(cmd)
			wsID := RequireWorkspace(cmd)
			path := fmt.Sprintf(res.CreatePath, wsID)

			body, err := loadBodyFromFileOrFlags(cmd)
			if err != nil {
				return err
			}
			applyInfraCreateFlags(cmd, res.Name, body)
			if err := validateInfraCreateBody(res.Name, body); err != nil {
				return err
			}

			resp, err := rc.Client.Post(cmd.Context(), path, body)
			if err != nil {
				return err
			}
			if apiErr := resp.ParseError(); apiErr != nil {
				return apiErr
			}

			var created map[string]any
			if err := resp.DecodeJSON(&created); err != nil {
				return err
			}

			if rc.Output.IsStructured() {
				return rc.Output.PrintRaw(created)
			}
			rc.Output.PrintSuccess(fmt.Sprintf("Created %s %s", res.Name, str(created["id"])))
			return nil
		},
	}
	createCmd.Flags().String("from-file", "", "JSON file with resource spec")
	addInfraCreateFlags(createCmd, res.Name)
	parent.AddCommand(createCmd)

	// get (if path provided)
	if res.GetPath != "" {
		getCmd := &cobra.Command{
			Use:   "get <id>",
			Short: fmt.Sprintf("Get a %s", res.Name),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				rc := GetRunContext(cmd)
				path := fmt.Sprintf(res.GetPath, args[0])

				resp, err := rc.Client.Get(cmd.Context(), path, nil)
				if err != nil {
					return err
				}
				if apiErr := resp.ParseError(); apiErr != nil {
					return apiErr
				}

				var item map[string]any
				if err := resp.DecodeJSON(&item); err != nil {
					return err
				}

				return rc.Output.PrintRaw(item)
			},
		}
		parent.AddCommand(getCmd)
	}

	// delete (if path provided)
	if res.DeletePath != "" {
		deleteCmd := &cobra.Command{
			Use:   "delete <id>",
			Short: fmt.Sprintf("Delete a %s", res.Name),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				rc := GetRunContext(cmd)
				path := fmt.Sprintf(res.DeletePath, args[0])

				resp, err := rc.Client.Delete(cmd.Context(), path)
				if err != nil {
					return err
				}
				if apiErr := resp.ParseError(); apiErr != nil {
					return apiErr
				}

				rc.Output.PrintSuccess(fmt.Sprintf("Deleted %s %s", res.Name, args[0]))
				return nil
			},
		}
		parent.AddCommand(deleteCmd)
	}
	addInfraResourceExtraCommands(parent, res.Name)

	// archive (if path provided)
	if res.ArchivePath != "" {
		archiveCmd := &cobra.Command{
			Use:   "archive <id>",
			Short: fmt.Sprintf("Archive a %s", res.Name),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				rc := GetRunContext(cmd)
				path := fmt.Sprintf(res.ArchivePath, args[0])

				resp, err := rc.Client.Post(cmd.Context(), path, nil)
				if err != nil {
					return err
				}
				if apiErr := resp.ParseError(); apiErr != nil {
					return apiErr
				}

				rc.Output.PrintSuccess(fmt.Sprintf("Archived %s %s", res.Name, args[0]))
				return nil
			},
		}
		parent.AddCommand(archiveCmd)
	}

	return parent
}

func addInfraResourceExtraCommands(parent *cobra.Command, resourceName string) {
	switch resourceName {
	case "provider-account":
		parent.AddCommand(providerAccountTestCmd())
		parent.AddCommand(providerAccountModelsCmd())
	}
}

func providerAccountModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models <id>",
		Short: "List models available to a provider account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rc := GetRunContext(cmd)

			resp, err := rc.Client.Get(cmd.Context(), "/v1/provider-accounts/"+args[0]+"/models", nil)
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

			cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Input/M"}, {Header: "Output/M"}, {Header: "Pricing Source"}}
			rows := make([][]string, len(result.Items))
			for i, item := range result.Items {
				rows[i] = []string{
					str(item["id"]),
					str(item["display_name"]),
					fmtPricingRate(item["input_cost_per_mtok"]),
					fmtPricingRate(item["output_cost_per_mtok"]),
					str(item["pricing_source"]),
				}
			}
			rc.Output.PrintTable(cols, rows)
			return nil
		},
	}
}

func providerAccountTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <id>",
		Short: "Smoke test provider account credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rc := GetRunContext(cmd)
			body := map[string]any{}
			if cmd.Flags().Changed("model") {
				model, _ := cmd.Flags().GetString("model")
				body["model"] = model
			}
			if cmd.Flags().Changed("timeout-seconds") {
				timeoutSeconds, _ := cmd.Flags().GetInt32("timeout-seconds")
				body["step_timeout_seconds"] = timeoutSeconds
			}

			resp, err := rc.Client.Post(cmd.Context(), "/v1/provider-accounts/"+args[0]+"/test", body)
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
			if rc.Output.IsStructured() {
				if err := rc.Output.PrintRaw(result); err != nil {
					return err
				}
			} else {
				printProviderAccountTestResult(rc.Output, result)
			}
			if passed, _ := result["passed"].(bool); !passed {
				return fmt.Errorf("provider account test failed: %s", str(result["message"]))
			}
			return nil
		},
	}
	cmd.Flags().String("model", "", "Provider model ID to use for the smoke test")
	cmd.Flags().Int32("timeout-seconds", 20, "Provider call timeout in seconds, capped by the API")
	return cmd
}

func printProviderAccountTestResult(formatter *output.Formatter, result map[string]any) {
	formatter.PrintDetail("Status", output.StatusColor(str(result["status"])))
	formatter.PrintDetail("Provider", str(result["provider_key"]))
	formatter.PrintDetail("Model", str(result["model"]))
	if providerModel := str(result["provider_model_id"]); providerModel != "" {
		formatter.PrintDetail("Provider Model", providerModel)
	}
	if code := str(result["code"]); code != "" {
		formatter.PrintDetail("Code", code)
	}
	if message := str(result["message"]); message != "" {
		formatter.PrintDetail("Message", message)
	}
	if duration := fmtScore(result["duration_ms"]); duration != "" {
		formatter.PrintDetail("Duration MS", duration)
	}
}

func fmtPricingRate(v any) string {
	if v == nil {
		return "-"
	}
	if f, ok := v.(float64); ok {
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
	}
	return str(v)
}

func addInfraCreateFlags(_ *cobra.Command, _ string) {}

func applyInfraCreateFlags(_ *cobra.Command, _ string, _ map[string]any) {}

func validateInfraCreateBody(_ string, _ map[string]any) error {
	return nil
}
