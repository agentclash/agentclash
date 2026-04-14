package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.AddCommand(buildListCmd)
	buildCmd.AddCommand(buildGetCmd)
	buildCmd.AddCommand(buildCreateCmd)
	buildCmd.AddCommand(buildVersionCmd)
	buildVersionCmd.AddCommand(buildVersionCreateCmd)
	buildVersionCmd.AddCommand(buildVersionGetCmd)
	buildVersionCmd.AddCommand(buildVersionUpdateCmd)
	buildVersionCmd.AddCommand(buildVersionValidateCmd)
	buildVersionCmd.AddCommand(buildVersionReadyCmd)

	buildCreateCmd.Flags().String("name", "", "Build name (required)")
	buildCreateCmd.Flags().String("description", "", "Build description")
	buildCreateCmd.MarkFlagRequired("name")

	buildVersionCreateCmd.Flags().String("agent-kind", "", "Agent kind: llm_agent, workflow_agent, programmatic_agent, multi_agent_system, hosted_external")
	buildVersionCreateCmd.Flags().String("spec-file", "", "JSON file with version spec fields")

	buildVersionUpdateCmd.Flags().String("spec-file", "", "JSON file with updated version spec fields")
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Manage agent builds",
}

var buildListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent builds",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-builds", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Slug"}, {Header: "Status"}, {Header: "Created"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				str(item["slug"]),
				output.StatusColor(str(item["lifecycle_status"])),
				str(item["created_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var buildGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get agent build with version history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/agent-builds/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var build map[string]any
		if err := resp.DecodeJSON(&build); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(build)
		}

		rc.Output.PrintDetail("ID", str(build["id"]))
		rc.Output.PrintDetail("Name", str(build["name"]))
		rc.Output.PrintDetail("Slug", str(build["slug"]))
		rc.Output.PrintDetail("Description", str(build["description"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(build["lifecycle_status"])))
		rc.Output.PrintDetail("Created", str(build["created_at"]))

		if versions, ok := build["versions"].([]any); ok && len(versions) > 0 {
			fmt.Fprintln(rc.Output.Writer())
			fmt.Fprintln(rc.Output.Writer(), output.Bold("Versions:"))
			cols := []output.Column{{Header: "ID"}, {Header: "Version"}, {Header: "Kind"}, {Header: "Status"}, {Header: "Created"}}
			rows := make([][]string, len(versions))
			for i, v := range versions {
				ver := v.(map[string]any)
				rows[i] = []string{
					str(ver["id"]),
					str(ver["version_number"]),
					str(ver["agent_kind"]),
					output.StatusColor(str(ver["status"])),
					str(ver["created_at"]),
				}
			}
			rc.Output.PrintTable(cols, rows)
		}
		return nil
	},
}

var buildCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent build",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")

		body := map[string]any{"name": name}
		if desc != "" {
			body["description"] = desc
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-builds", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var build map[string]any
		if err := resp.DecodeJSON(&build); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(build)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created build %s (%s)", str(build["name"]), str(build["id"])))
		return nil
	},
}

var buildVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Manage agent build versions",
}

var buildVersionCreateCmd = &cobra.Command{
	Use:   "create <buildId>",
	Short: "Create a new draft version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		body, err := loadSpecBody(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/agent-builds/"+args[0]+"/versions", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var version map[string]any
		if err := resp.DecodeJSON(&version); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(version)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created version %s (v%s)", str(version["id"]), str(version["version_number"])))
		return nil
	},
}

var buildVersionGetCmd = &cobra.Command{
	Use:   "get <versionId>",
	Short: "Get a build version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/agent-build-versions/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var version map[string]any
		if err := resp.DecodeJSON(&version); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(version)
		}

		rc.Output.PrintDetail("ID", str(version["id"]))
		rc.Output.PrintDetail("Version", str(version["version_number"]))
		rc.Output.PrintDetail("Agent Kind", str(version["agent_kind"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(version["status"])))
		rc.Output.PrintDetail("Created", str(version["created_at"]))
		return nil
	},
}

var buildVersionUpdateCmd = &cobra.Command{
	Use:   "update <versionId>",
	Short: "Update a draft build version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		body, err := loadSpecBody(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Patch(cmd.Context(), "/v1/agent-build-versions/"+args[0], body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var version map[string]any
		if err := resp.DecodeJSON(&version); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(version)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Updated version %s", args[0]))
		return nil
	},
}

var buildVersionValidateCmd = &cobra.Command{
	Use:   "validate <versionId>",
	Short: "Validate a build version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Post(cmd.Context(), "/v1/agent-build-versions/"+args[0]+"/validate", nil)
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

		if valid, ok := result["valid"].(bool); ok && valid {
			rc.Output.PrintSuccess("Version is valid")
		} else {
			rc.Output.PrintError("Version has validation errors")
			if errors, ok := result["errors"].([]any); ok {
				for _, e := range errors {
					fmt.Fprintf(os.Stderr, "  - %v\n", e)
				}
			}
		}
		return nil
	},
}

var buildVersionReadyCmd = &cobra.Command{
	Use:   "ready <versionId>",
	Short: "Mark a version as ready (immutable, deployable)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Post(cmd.Context(), "/v1/agent-build-versions/"+args[0]+"/ready", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var version map[string]any
		if err := resp.DecodeJSON(&version); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(version)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Version %s is now ready", args[0]))
		return nil
	},
}

func loadSpecBody(cmd *cobra.Command) (map[string]any, error) {
	body := make(map[string]any)

	if specFile, _ := cmd.Flags().GetString("spec-file"); specFile != "" {
		data, err := os.ReadFile(specFile)
		if err != nil {
			return nil, fmt.Errorf("reading spec file: %w", err)
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("parsing spec file: %w", err)
		}
	}

	if cmd.Flags().Changed("agent-kind") {
		v, _ := cmd.Flags().GetString("agent-kind")
		body["agent_kind"] = v
	}

	return body, nil
}
