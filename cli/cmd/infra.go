package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

// infraResource describes a workspace-scoped infrastructure resource for the command factory.
type infraResource struct {
	Name       string // e.g. "runtime-profile"
	Plural     string // e.g. "runtime-profiles"
	ListPath   string // e.g. "/v1/workspaces/%s/runtime-profiles"
	CreatePath string // same as ListPath for POST
	GetPath    string // e.g. "/v1/runtime-profiles/%s" (empty = no get command)
	DeletePath string // e.g. "/v1/runtime-profiles/%s" (empty = no delete command)
	ArchivePath string // e.g. "/v1/runtime-profiles/%s/archive" (empty = no archive command)
	Columns    []output.Column
	RowMapper  func(map[string]any) []string
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
		Name: "model-alias", Plural: "model-aliases",
		ListPath: "/v1/workspaces/%s/model-aliases", CreatePath: "/v1/workspaces/%s/model-aliases",
		GetPath: "/v1/model-aliases/%s", DeletePath: "/v1/model-aliases/%s",
		Columns: []output.Column{{Header: "ID"}, {Header: "Alias Key"}, {Header: "Display Name"}, {Header: "Status"}, {Header: "Created"}},
		RowMapper: func(item map[string]any) []string {
			return []string{str(item["id"]), str(item["alias_key"]), str(item["display_name"]), output.StatusColor(str(item["status"])), str(item["created_at"])}
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
		Name: "knowledge-source", Plural: "knowledge-sources",
		ListPath: "/v1/workspaces/%s/knowledge-sources", CreatePath: "/v1/workspaces/%s/knowledge-sources",
		GetPath: "/v1/knowledge-sources/%s",
		Columns: []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Kind"}, {Header: "Status"}, {Header: "Created"}},
		RowMapper: func(item map[string]any) []string {
			return []string{str(item["id"]), str(item["name"]), str(item["source_kind"]), output.StatusColor(str(item["lifecycle_status"])), str(item["created_at"])}
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

	// Add model-catalog separately (global, not workspace-scoped).
	infraCmd.AddCommand(modelCatalogCmd)
	modelCatalogCmd.AddCommand(modelCatalogListCmd)
	modelCatalogCmd.AddCommand(modelCatalogGetCmd)

	// Generate CRUD subcommands for each workspace-scoped resource.
	for _, res := range infraResources {
		infraCmd.AddCommand(newInfraResourceCmd(res))
	}
}

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage infrastructure resources",
	Long:  "Manage runtime profiles, provider accounts, model aliases,\ntools, knowledge sources, routing policies, and spend policies.",
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

			if rc.Output.IsJSON() {
				return rc.Output.PrintJSON(result)
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

			if rc.Output.IsJSON() {
				return rc.Output.PrintJSON(created)
			}
			rc.Output.PrintSuccess(fmt.Sprintf("Created %s %s", res.Name, str(created["id"])))
			return nil
		},
	}
	createCmd.Flags().String("from-file", "", "JSON file with resource spec")
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

				rc.Output.PrintRaw(item)
				return nil
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

// --- Model Catalog (global, not workspace-scoped) ---

var modelCatalogCmd = &cobra.Command{
	Use:     "model-catalog",
	Aliases: []string{"models"},
	Short:   "Browse the global model catalog",
}

var modelCatalogListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/model-catalog", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Provider"}, {Header: "Model"}, {Header: "Family"}, {Header: "Status"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["provider_key"]),
				str(item["display_name"]),
				str(item["model_family"]),
				output.StatusColor(str(item["lifecycle_status"])),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var modelCatalogGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a model catalog entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/model-catalog/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var entry map[string]any
		if err := resp.DecodeJSON(&entry); err != nil {
			return err
		}

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(entry)
		}

		rc.Output.PrintDetail("ID", str(entry["id"]))
		rc.Output.PrintDetail("Provider", str(entry["provider_key"]))
		rc.Output.PrintDetail("Model ID", str(entry["provider_model_id"]))
		rc.Output.PrintDetail("Display Name", str(entry["display_name"]))
		rc.Output.PrintDetail("Family", str(entry["model_family"]))
		rc.Output.PrintDetail("Modality", str(entry["modality"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(entry["lifecycle_status"])))

		if md, ok := entry["metadata"]; ok && md != nil {
			mdJSON, _ := json.MarshalIndent(md, "", "  ")
			fmt.Fprintf(rc.Output.Writer(), "\n%s\n%s\n", output.Bold("Metadata:"), string(mdJSON))
		}
		return nil
	},
}
