package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(challengePackCmd)
	challengePackCmd.AddCommand(cpListCmd)
	challengePackCmd.AddCommand(cpInitCmd)
	challengePackCmd.AddCommand(cpPublishCmd)
	challengePackCmd.AddCommand(cpValidateCmd)

	cpInitCmd.Flags().String("template", "prompt_eval", "Starter template: prompt_eval or native")
	cpInitCmd.Flags().String("name", "", "Challenge pack display name (defaults from the file name)")
	cpInitCmd.Flags().String("slug", "", "Challenge pack slug (defaults from the file name)")
	cpInitCmd.Flags().Bool("force", false, "Overwrite an existing file")
}

var challengePackCmd = &cobra.Command{
	Use:     "challenge-pack",
	Aliases: []string{"cp"},
	Short:   "Manage challenge packs",
}

var cpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List challenge packs",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/challenge-packs", nil)
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

		cols := []output.Column{{Header: "ID"}, {Header: "Name"}, {Header: "Slug"}, {Header: "Status"}, {Header: "Versions"}}
		rows := make([][]string, len(result.Items))
		for i, item := range result.Items {
			versionCount := "0"
			if versions, ok := item["versions"].([]any); ok {
				versionCount = fmt.Sprintf("%d", len(versions))
			}
			rows[i] = []string{
				str(item["id"]),
				str(item["name"]),
				str(item["slug"]),
				output.StatusColor(str(item["lifecycle_status"])),
				versionCount,
			}
		}
		rc.Output.PrintTable(cols, rows)
		return nil
	},
}

var cpInitCmd = &cobra.Command{
	Use:   "init <file>",
	Short: "Scaffold a minimal challenge pack YAML bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		templateMode, _ := cmd.Flags().GetString("template")
		name, _ := cmd.Flags().GetString("name")
		slug, _ := cmd.Flags().GetString("slug")
		force, _ := cmd.Flags().GetBool("force")

		targetPath := args[0]
		if !force {
			if _, err := os.Stat(targetPath); err == nil {
				return fmt.Errorf("%s already exists; pass --force to overwrite", targetPath)
			}
		}

		templateMode = strings.TrimSpace(templateMode)
		switch templateMode {
		case "prompt_eval", "native":
		default:
			return fmt.Errorf("invalid template %q (want prompt_eval or native)", templateMode)
		}

		defaultName := defaultChallengePackName(targetPath)
		name = strings.TrimSpace(name)
		if name == "" {
			name = defaultName
		}
		slug = strings.TrimSpace(slug)
		if slug == "" {
			slug = slugifyChallengePackName(name)
		}
		if slug == "" {
			return fmt.Errorf("could not derive a slug from %q; pass --slug explicitly", name)
		}

		payload, err := buildChallengePackTemplate(name, slug, templateMode)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, payload, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", targetPath, err)
		}

		result := map[string]any{
			"path":     targetPath,
			"name":     name,
			"slug":     slug,
			"template": templateMode,
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		rc.Output.PrintSuccess(fmt.Sprintf("Created %s", targetPath))
		rc.Output.PrintDetail("Name", name)
		rc.Output.PrintDetail("Slug", slug)
		rc.Output.PrintDetail("Template", templateMode)
		return nil
	},
}

var cpPublishCmd = &cobra.Command{
	Use:   "publish <file>",
	Short: "Publish a challenge pack YAML bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		sp := output.NewSpinner("Publishing challenge pack...", flagQuiet)
		resp, err := rc.Client.PostRaw(cmd.Context(), "/v1/workspaces/"+wsID+"/challenge-packs", "application/octet-stream", bytes.NewReader(data))
		if err != nil {
			sp.StopWithError("Publish failed")
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			sp.StopWithError("Publish failed")
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		sp.StopWithSuccess("Published")

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}

		rc.Output.PrintDetail("Pack ID", str(result["challenge_pack_id"]))
		rc.Output.PrintDetail("Version ID", str(result["challenge_pack_version_id"]))
		return nil
	},
}

var cpValidateCmd = &cobra.Command{
	Use:   "validate <file>",
	Short: "Validate a challenge pack YAML bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		resp, err := rc.Client.PostRaw(cmd.Context(), "/v1/workspaces/"+wsID+"/challenge-packs/validate", "application/octet-stream", bytes.NewReader(data))
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
			return rc.Output.PrintRaw(result)
		}

		if valid, ok := result["valid"].(bool); ok && valid {
			rc.Output.PrintSuccess("Challenge pack is valid")
		} else {
			rc.Output.PrintError("Challenge pack has errors")
			if errors, ok := result["errors"].([]any); ok {
				for _, e := range errors {
					fmt.Fprintf(os.Stderr, "  - %v\n", e)
				}
			}
			return fmt.Errorf("validation failed")
		}
		return nil
	},
}

func defaultChallengePackName(targetPath string) string {
	base := filepath.Base(targetPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		return "Starter Eval"
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(parts) == 0 {
		return "Starter Eval"
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func slugifyChallengePackName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var builder strings.Builder
	lastHyphen := false
	for _, r := range name {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			builder.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen && builder.Len() > 0 {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func buildChallengePackTemplate(name, slug, templateMode string) ([]byte, error) {
	template := map[string]any{
		"pack": map[string]any{
			"slug":   slug,
			"name":   name,
			"family": slug,
		},
		"version": map[string]any{
			"number":         1,
			"execution_mode": templateMode,
			"evaluation_spec": map[string]any{
				"name":           slug + "-v1",
				"version_number": 1,
				"judge_mode":     "deterministic",
				"validators": []map[string]any{
					{
						"key":           "exact",
						"type":          "exact_match",
						"target":        "final_output",
						"expected_from": "challenge_input",
					},
				},
				"scorecard": map[string]any{
					"dimensions": []string{"correctness"},
				},
			},
		},
		"challenges": []map[string]any{
			{
				"key":          "task-1",
				"title":        "Starter Task",
				"category":     "general",
				"difficulty":   "medium",
				"instructions": "Read the request and produce the final answer.\n",
			},
		},
		"input_sets": []map[string]any{
			{
				"key":  "default",
				"name": "Default Inputs",
				"cases": []map[string]any{
					{
						"challenge_key": "task-1",
						"case_key":      "sample-1",
						"inputs": []map[string]any{
							{
								"key":   "prompt",
								"kind":  "text",
								"value": "hello",
							},
						},
						"expectations": []map[string]any{
							{
								"key":    "answer",
								"kind":   "text",
								"source": "input:prompt",
							},
						},
					},
				},
			},
		},
	}
	return yaml.Marshal(template)
}
