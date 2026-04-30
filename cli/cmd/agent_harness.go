package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(agentHarnessCmd)
	agentHarnessCmd.AddCommand(agentHarnessListCmd)
	agentHarnessCmd.AddCommand(agentHarnessCreateCmd)
	agentHarnessCmd.AddCommand(agentHarnessGetCmd)

	agentHarnessCreateCmd.Flags().String("from-file", "", "JSON file with agent harness spec")
	agentHarnessCreateCmd.Flags().String("name", "", "Harness name")
	agentHarnessCreateCmd.Flags().String("description", "", "Harness description")
	agentHarnessCreateCmd.Flags().String("task", "", "Task prompt for the coding harness")
	agentHarnessCreateCmd.Flags().String("codex-template", "codex", "E2B template with Codex installed")
	agentHarnessCreateCmd.Flags().String("codex-model", "", "Codex model override")
	agentHarnessCreateCmd.Flags().String("auth-mode", "chatgpt_device", "Codex auth mode: chatgpt_device, api_key_secret, bring_your_own_env")
	agentHarnessCreateCmd.Flags().String("openai-api-key-secret", "", "Workspace secret name containing OPENAI_API_KEY")
	agentHarnessCreateCmd.Flags().String("e2b-api-key-secret", "E2B_API_KEY", "Workspace secret name containing E2B_API_KEY")
	agentHarnessCreateCmd.Flags().String("repository-url", "", "Repository URL for the harness task")
	agentHarnessCreateCmd.Flags().String("base-branch", "", "Base branch for repository work")
	agentHarnessCreateCmd.Flags().String("execution-config", "", "Inline JSON execution config")
	agentHarnessCreateCmd.Flags().String("evaluation-config", "", "Inline JSON evaluation config")
	agentHarnessCreateCmd.Flags().String("evaluation-config-file", "", "JSON file with validators and LLM judges")
}

var agentHarnessCmd = &cobra.Command{
	Use:     "agent-harness",
	Aliases: []string{"harness"},
	Short:   "Manage coding-agent harnesses",
	Long: `Manage Agent Harnesses: workspace-scoped coding-agent task definitions.

Agent Harnesses are not challenge packs. They store a task prompt, Codex/E2B
execution settings, and reusable evaluation config for long-running autonomous
coding checks.`,
}

var agentHarnessListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent harnesses",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harnesses", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Auth"}, {Header: "Template"}, {Header: "Status"}, {Header: "Updated"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				str(item["auth_mode"]),
				str(item["codex_template"]),
				output.StatusColor(str(item["status"])),
				str(item["updated_at"]),
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var agentHarnessGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get an agent harness",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harnesses/"+args[0], nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var harness map[string]any
		if err := resp.DecodeJSON(&harness); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(harness)
		}

		rc.Output.PrintDetail("ID", str(harness["id"]))
		rc.Output.PrintDetail("Name", str(harness["name"]))
		rc.Output.PrintDetail("Auth", str(harness["auth_mode"]))
		rc.Output.PrintDetail("Codex Template", str(harness["codex_template"]))
		rc.Output.PrintDetail("Repository", str(harness["repository_url"]))
		rc.Output.PrintDetail("Status", output.StatusColor(str(harness["status"])))
		rc.Output.PrintDetail("Task", str(harness["task_prompt"]))
		return nil
	},
}

var agentHarnessCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an agent harness",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		body, err := buildAgentHarnessCreateBody(cmd)
		if err != nil {
			return err
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/agent-harnesses", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var harness map[string]any
		if err := resp.DecodeJSON(&harness); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(harness)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created agent harness %s (%s)", str(harness["name"]), str(harness["id"])))
		rc.Output.PrintDetail("Auth", str(harness["auth_mode"]))
		rc.Output.PrintDetail("Codex Template", str(harness["codex_template"]))
		return nil
	},
}

func buildAgentHarnessCreateBody(cmd *cobra.Command) (map[string]any, error) {
	if fromFile, _ := cmd.Flags().GetString("from-file"); fromFile != "" {
		data, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}
		var body map[string]any
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("parsing file: %w", err)
		}
		return body, nil
	}

	missing := requiredAgentHarnessCreateFlags(cmd)
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required flags when --from-file is not used: %s", strings.Join(missing, ", "))
	}

	body := make(map[string]any)
	setFlagIfChanged(cmd, body, "name", "name")
	setFlagIfChanged(cmd, body, "description", "description")
	setFlagIfChanged(cmd, body, "task", "task_prompt")
	setFlagIfChanged(cmd, body, "codex-model", "codex_model")
	setFlagIfChanged(cmd, body, "openai-api-key-secret", "openai_api_key_secret_name")
	setFlagIfChanged(cmd, body, "repository-url", "repository_url")
	setFlagIfChanged(cmd, body, "base-branch", "base_branch")
	codexTemplate, _ := cmd.Flags().GetString("codex-template")
	body["codex_template"] = codexTemplate
	authMode, _ := cmd.Flags().GetString("auth-mode")
	body["auth_mode"] = authMode
	if e2bSecret, _ := cmd.Flags().GetString("e2b-api-key-secret"); strings.TrimSpace(e2bSecret) != "" {
		body["e2b_api_key_secret_name"] = e2bSecret
	}

	if err := setJSONFlag(cmd, body, "execution-config", "execution_config"); err != nil {
		return nil, err
	}
	if err := setJSONFlag(cmd, body, "evaluation-config", "evaluation_config"); err != nil {
		return nil, err
	}
	if evalFile, _ := cmd.Flags().GetString("evaluation-config-file"); evalFile != "" {
		value, err := readJSONFile(evalFile)
		if err != nil {
			return nil, err
		}
		body["evaluation_config"] = value
	}

	return body, nil
}

func requiredAgentHarnessCreateFlags(cmd *cobra.Command) []string {
	required := []string{"name", "task", "auth-mode"}
	authMode, _ := cmd.Flags().GetString("auth-mode")
	if strings.TrimSpace(authMode) == "api_key_secret" {
		required = append(required, "openai-api-key-secret")
	}
	missing := make([]string, 0, len(required))
	for _, flagName := range required {
		value, _ := cmd.Flags().GetString(flagName)
		if strings.TrimSpace(value) == "" {
			missing = append(missing, "--"+flagName)
		}
	}
	return missing
}

func setJSONFlag(cmd *cobra.Command, body map[string]any, flagName, jsonKey string) error {
	if !cmd.Flags().Changed(flagName) {
		return nil
	}
	raw, _ := cmd.Flags().GetString(flagName)
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fmt.Errorf("--%s must be valid JSON: %w", flagName, err)
	}
	body[jsonKey] = value
	return nil
}

func readJSONFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}
	return value, nil
}
