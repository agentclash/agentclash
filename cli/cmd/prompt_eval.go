package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	promptEvalSchemaVersion = 1
	promptEvalExitInvalid   = 5
)

func init() {
	rootCmd.AddCommand(promptEvalCmd)
	promptEvalCmd.AddCommand(promptEvalInitCmd)
	promptEvalCmd.AddCommand(promptEvalValidateCmd)

	promptEvalInitCmd.Flags().Bool("force", false, "Overwrite an existing file")
	promptEvalInitCmd.Flags().String("name", "", "Prompt eval name (defaults from the file name)")

	promptEvalValidateCmd.Flags().Int("max-cases", 100, "Maximum model x test cases allowed before launch")
	promptEvalValidateCmd.Flags().Bool("remote", false, "Validate referenced AgentClash workspace resources without creating them")
	promptEvalValidateCmd.Flags().Bool("ci", false, "Apply CI-safe validation rules")
}

var promptEvalCmd = &cobra.Command{
	Use:   "prompt-eval",
	Short: "Manage prompt eval configs",
}

var promptEvalInitCmd = &cobra.Command{
	Use:   "init [file]",
	Short: "Scaffold a prompt eval YAML config",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		targetPath := ".agentclash/prompt-eval.yaml"
		if len(args) > 0 {
			targetPath = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")
		name, _ := cmd.Flags().GetString("name")
		if strings.TrimSpace(name) == "" {
			name = defaultPromptEvalName(targetPath)
		}
		if !force {
			if _, err := os.Stat(targetPath); err == nil {
				return fmt.Errorf("%s already exists; pass --force to overwrite", targetPath)
			}
		}
		if dir := filepath.Dir(targetPath); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating %s: %w", dir, err)
			}
		}
		payload, err := buildPromptEvalScaffold(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, payload, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", targetPath, err)
		}
		result := map[string]any{"path": targetPath, "name": name, "schemaVersion": promptEvalSchemaVersion}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		rc.Output.PrintSuccess(fmt.Sprintf("Created %s", targetPath))
		rc.Output.PrintDetail("Name", name)
		return nil
	},
}

var promptEvalValidateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate a prompt eval YAML config locally",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		path := ".agentclash/prompt-eval.yaml"
		if len(args) > 0 {
			path = args[0]
		}
		maxCases, _ := cmd.Flags().GetInt("max-cases")
		remote, _ := cmd.Flags().GetBool("remote")
		ciMode, _ := cmd.Flags().GetBool("ci")
		cfg, result := validatePromptEvalFileWithConfig(path, maxCases)
		if result.Valid && remote {
			validatePromptEvalRemote(cmd, rc, cfg, ciMode, &result)
			result.Valid = len(result.Errors) == 0
			if !result.Valid {
				result.ExitCode = promptEvalExitInvalid
			}
		}
		if rc.Output.IsStructured() {
			if err := rc.Output.PrintRaw(result); err != nil {
				return err
			}
		} else {
			renderPromptEvalValidation(rc, result)
		}
		if !result.Valid {
			return &ExitCodeError{Code: promptEvalExitInvalid}
		}
		return nil
	},
}

type promptEvalConfig struct {
	SchemaVersion int                  `yaml:"schemaVersion" json:"schemaVersion"`
	Name          string               `yaml:"name" json:"name"`
	Prompt        promptEvalPrompt     `yaml:"prompt" json:"prompt"`
	Models        []promptEvalModel    `yaml:"models" json:"models"`
	Tests         []promptEvalTest     `yaml:"tests" json:"tests"`
	Thresholds    promptEvalThresholds `yaml:"thresholds" json:"thresholds"`
}

type promptEvalPrompt struct {
	Template string `yaml:"template" json:"template"`
}

type promptEvalModel struct {
	Alias           string `yaml:"alias,omitempty" json:"alias,omitempty"`
	ModelAliasID    string `yaml:"model_alias_id,omitempty" json:"model_alias_id,omitempty"`
	ProviderAccount string `yaml:"provider_account,omitempty" json:"provider_account,omitempty"`
}

type promptEvalTest struct {
	Key    string                `yaml:"key" json:"key"`
	Vars   map[string]any        `yaml:"vars" json:"vars"`
	Expect promptEvalExpect      `yaml:"expect" json:"expect"`
	Assert []promptEvalAssertion `yaml:"assert" json:"assert"`
}

type promptEvalExpect struct {
	Output any `yaml:"output,omitempty" json:"output,omitempty"`
}

type promptEvalAssertion struct {
	Type   string `yaml:"type" json:"type"`
	Value  any    `yaml:"value,omitempty" json:"value,omitempty"`
	Metric string `yaml:"metric,omitempty" json:"metric,omitempty"`
}

type promptEvalThresholds struct {
	AssertionPassRate *float64           `yaml:"assertion_pass_rate,omitempty" json:"assertion_pass_rate,omitempty"`
	Dimensions        map[string]float64 `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
}

type promptEvalValidationResult struct {
	SchemaVersion       int                         `json:"schemaVersion" yaml:"schemaVersion"`
	Path                string                      `json:"path" yaml:"path"`
	Valid               bool                        `json:"valid" yaml:"valid"`
	Errors              []string                    `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings            []string                    `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	ModelCount          int                         `json:"model_count" yaml:"model_count"`
	TestCount           int                         `json:"test_count" yaml:"test_count"`
	CaseCount           int                         `json:"case_count" yaml:"case_count"`
	MaxCases            int                         `json:"max_cases" yaml:"max_cases"`
	AssertionSignatures []string                    `json:"assertion_signatures" yaml:"assertion_signatures"`
	Remote              *promptEvalRemoteValidation `json:"remote,omitempty" yaml:"remote,omitempty"`
	ExitCode            int                         `json:"exit_code" yaml:"exit_code"`
}

type promptEvalRemoteValidation struct {
	WorkspaceID string                       `json:"workspace_id" yaml:"workspace_id"`
	Models      []promptEvalRemoteModel      `json:"models,omitempty" yaml:"models,omitempty"`
	Playgrounds []promptEvalRemotePlayground `json:"playgrounds,omitempty" yaml:"playgrounds,omitempty"`
	DryRun      promptEvalRemoteDryRun       `json:"dry_run" yaml:"dry_run"`
}

type promptEvalRemoteModel struct {
	Alias             string `json:"alias,omitempty" yaml:"alias,omitempty"`
	ModelAliasID      string `json:"model_alias_id" yaml:"model_alias_id"`
	ProviderAccountID string `json:"provider_account_id" yaml:"provider_account_id"`
}

type promptEvalRemotePlayground struct {
	Name         string `json:"name" yaml:"name"`
	Signature    string `json:"signature" yaml:"signature"`
	PlaygroundID string `json:"playground_id,omitempty" yaml:"playground_id,omitempty"`
	TestsCreate  int    `json:"tests_create" yaml:"tests_create"`
	TestsUpdate  int    `json:"tests_update" yaml:"tests_update"`
	TestsNoop    int    `json:"tests_noop" yaml:"tests_noop"`
	TestsOrphan  int    `json:"tests_orphan" yaml:"tests_orphan"`
}

type promptEvalRemoteDryRun struct {
	PlaygroundsCreate int `json:"playgrounds_create" yaml:"playgrounds_create"`
	PlaygroundsReuse  int `json:"playgrounds_reuse" yaml:"playgrounds_reuse"`
	TestsCreate       int `json:"tests_create" yaml:"tests_create"`
	TestsUpdate       int `json:"tests_update" yaml:"tests_update"`
	TestsNoop         int `json:"tests_noop" yaml:"tests_noop"`
	TestsOrphan       int `json:"tests_orphan" yaml:"tests_orphan"`
}

func buildPromptEvalScaffold(name string) ([]byte, error) {
	cfg := promptEvalConfig{
		SchemaVersion: promptEvalSchemaVersion,
		Name:          name,
		Prompt: promptEvalPrompt{Template: `You are a helpful assistant.
Reply to: {{input}}
`},
		Models: []promptEvalModel{{Alias: "gpt-5.5", ProviderAccount: "default"}},
		Tests: []promptEvalTest{{
			Key:  "greeting",
			Vars: map[string]any{"input": "Say hello in French"},
			Expect: promptEvalExpect{
				Output: "Bonjour",
			},
			Assert: []promptEvalAssertion{{Type: "contains", Value: "Bonjour", Metric: "correctness"}},
		}},
		Thresholds: promptEvalThresholds{
			AssertionPassRate: floatPtrPromptEval(0.9),
			Dimensions:        map[string]float64{"correctness": 0.9},
		},
	}
	return yaml.Marshal(cfg)
}

func validatePromptEvalFile(path string, maxCases int) promptEvalValidationResult {
	_, result := validatePromptEvalFileWithConfig(path, maxCases)
	return result
}

func validatePromptEvalFileWithConfig(path string, maxCases int) (promptEvalConfig, promptEvalValidationResult) {
	result := promptEvalValidationResult{
		SchemaVersion:       promptEvalSchemaVersion,
		Path:                path,
		Valid:               true,
		MaxCases:            maxCases,
		AssertionSignatures: []string{},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("reading file: %v", err))
		result.ExitCode = promptEvalExitInvalid
		return promptEvalConfig{}, result
	}
	var cfg promptEvalConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("parsing yaml: %v", err))
		result.ExitCode = promptEvalExitInvalid
		return promptEvalConfig{}, result
	}
	result.ModelCount = len(cfg.Models)
	result.TestCount = len(cfg.Tests)
	result.CaseCount = len(cfg.Models) * len(cfg.Tests)

	validatePromptEvalConfig(cfg, maxCases, &result)
	result.Valid = len(result.Errors) == 0
	if !result.Valid {
		result.ExitCode = promptEvalExitInvalid
	}
	return cfg, result
}

func validatePromptEvalConfig(cfg promptEvalConfig, maxCases int, result *promptEvalValidationResult) {
	if cfg.SchemaVersion != promptEvalSchemaVersion {
		result.Errors = append(result.Errors, "schemaVersion must be 1")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		result.Errors = append(result.Errors, "name is required")
	}
	if strings.TrimSpace(cfg.Prompt.Template) == "" {
		result.Errors = append(result.Errors, "prompt.template is required")
	}
	templateVars, templateErrs := promptEvalTemplateVars(cfg.Prompt.Template)
	result.Errors = append(result.Errors, templateErrs...)
	if len(cfg.Models) == 0 {
		result.Errors = append(result.Errors, "at least one model is required")
	}
	for i, model := range cfg.Models {
		if strings.TrimSpace(model.Alias) == "" && strings.TrimSpace(model.ModelAliasID) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("models[%d] must set alias or model_alias_id", i))
		}
		if strings.TrimSpace(model.ProviderAccount) == "default" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("models[%d].provider_account uses default; pin a provider account before committing CI configs", i))
		}
	}
	if len(cfg.Tests) == 0 {
		result.Errors = append(result.Errors, "at least one test is required")
	}
	if len(cfg.Tests) == 1 {
		result.Warnings = append(result.Warnings, "single-test evals produce coarse 0/1 pass-rate gates")
	}
	if maxCases <= 0 {
		result.Errors = append(result.Errors, "--max-cases must be greater than 0")
	} else if result.CaseCount > maxCases {
		result.Errors = append(result.Errors, fmt.Sprintf("case count %d exceeds --max-cases %d", result.CaseCount, maxCases))
	}
	if cfg.Thresholds.AssertionPassRate != nil && (*cfg.Thresholds.AssertionPassRate < 0 || *cfg.Thresholds.AssertionPassRate > 1) {
		result.Errors = append(result.Errors, "thresholds.assertion_pass_rate must be between 0 and 1")
	}
	for key, value := range cfg.Thresholds.Dimensions {
		if value < 0 || value > 1 {
			result.Errors = append(result.Errors, fmt.Sprintf("thresholds.dimensions.%s must be between 0 and 1", key))
		}
	}

	seenTests := map[string]bool{}
	signatures := map[string]bool{}
	for i, test := range cfg.Tests {
		key := strings.TrimSpace(test.Key)
		if key == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].key is required", i))
		} else if seenTests[key] {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate test key %q", key))
		} else {
			seenTests[key] = true
		}
		for _, name := range templateVars {
			if _, ok := test.Vars[name]; !ok {
				result.Errors = append(result.Errors, fmt.Sprintf("tests[%d] missing template variable %q", i, name))
			}
		}
		assertions := promptEvalAssertionsForTest(test)
		if len(assertions) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("tests[%d] must define expect.output or assert[]", i))
		}
		for j, assertion := range assertions {
			validatePromptEvalAssertion(i, j, assertion, result)
		}
		if len(assertions) > 0 {
			signatures[promptEvalAssertionSignature(assertions)] = true
		}
	}
	result.AssertionSignatures = sortedPromptEvalKeys(signatures)
}

func validatePromptEvalRemote(cmd *cobra.Command, rc *RunContext, cfg promptEvalConfig, ciMode bool, result *promptEvalValidationResult) {
	workspaceID := strings.TrimSpace(rc.Workspace)
	remote := &promptEvalRemoteValidation{WorkspaceID: workspaceID}
	result.Remote = remote
	if workspaceID == "" {
		result.Errors = append(result.Errors, "no workspace specified for --remote. Pass --workspace, set AGENTCLASH_WORKSPACE, or run agentclash link.")
		return
	}

	modelAliases, err := promptEvalListRemoteItems(cmd, rc, fmt.Sprintf("/v1/workspaces/%s/model-aliases", workspaceID))
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	providerAccounts, err := promptEvalListRemoteItems(cmd, rc, fmt.Sprintf("/v1/workspaces/%s/provider-accounts", workspaceID))
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	for i, model := range cfg.Models {
		resolvedAlias, err := promptEvalResolveModelAlias(model, modelAliases)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("models[%d]: %v", i, err))
			continue
		}
		resolvedProvider, err := promptEvalResolveProviderAccount(model, resolvedAlias, providerAccounts, ciMode)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("models[%d]: %v", i, err))
			continue
		}
		remote.Models = append(remote.Models, promptEvalRemoteModel{
			Alias:             strings.TrimSpace(model.Alias),
			ModelAliasID:      mapString(resolvedAlias, "id"),
			ProviderAccountID: mapString(resolvedProvider, "id"),
		})
	}
	if len(result.Errors) > 0 {
		return
	}

	playgrounds, err := promptEvalListRemoteItems(cmd, rc, fmt.Sprintf("/v1/workspaces/%s/playgrounds", workspaceID))
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	groups := promptEvalTestGroups(cfg)
	for _, group := range groups {
		name := promptEvalPlaygroundName(cfg.Name, group.Signature, len(groups))
		matches := promptEvalItemsByName(playgrounds, name)
		pg := promptEvalRemotePlayground{Name: name, Signature: group.Signature}
		switch len(matches) {
		case 0:
			pg.TestsCreate = len(group.Tests)
			remote.DryRun.PlaygroundsCreate++
		case 1:
			pg.PlaygroundID = mapString(matches[0], "id")
			remote.DryRun.PlaygroundsReuse++
			promptEvalCompareRemoteTestCases(cmd, rc, pg.PlaygroundID, group, &pg, result)
		default:
			result.Errors = append(result.Errors, fmt.Sprintf("multiple playgrounds named %q; clean up duplicates before running prompt-eval", name))
		}
		remote.Playgrounds = append(remote.Playgrounds, pg)
		remote.DryRun.TestsCreate += pg.TestsCreate
		remote.DryRun.TestsUpdate += pg.TestsUpdate
		remote.DryRun.TestsNoop += pg.TestsNoop
		remote.DryRun.TestsOrphan += pg.TestsOrphan
	}
}

func promptEvalListRemoteItems(cmd *cobra.Command, rc *RunContext, path string) ([]map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), path, nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := resp.DecodeJSON(&payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func promptEvalResolveModelAlias(model promptEvalModel, aliases []map[string]any) (map[string]any, error) {
	if id := strings.TrimSpace(model.ModelAliasID); id != "" {
		for _, item := range aliases {
			if mapString(item, "id") == id {
				return item, nil
			}
		}
		return nil, fmt.Errorf("model_alias_id %q was not found in the workspace", id)
	}
	alias := strings.TrimSpace(model.Alias)
	matches := []map[string]any{}
	for _, item := range aliases {
		for _, key := range []string{"alias", "alias_key", "key", "name", "display_name"} {
			if mapString(item, key) == alias {
				matches = append(matches, item)
				break
			}
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("model alias %q was not found", alias)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("model alias %q matched %d workspace aliases; use model_alias_id", alias, len(matches))
	}
}

func promptEvalResolveProviderAccount(model promptEvalModel, alias map[string]any, providers []map[string]any, ciMode bool) (map[string]any, error) {
	selector := strings.TrimSpace(model.ProviderAccount)
	if selector == "" {
		return nil, fmt.Errorf("provider_account is required for --remote")
	}
	if selector == "default" {
		if ciMode {
			return nil, fmt.Errorf("provider_account: default is not allowed with --ci; pin a provider account")
		}
		aliasProviderID := mapString(alias, "provider_account_id")
		if aliasProviderID != "" {
			for _, item := range providers {
				if promptEvalProviderActive(item) && mapString(item, "id") == aliasProviderID {
					return item, nil
				}
			}
		}
		providerKey := mapString(alias, "provider_key", "provider")
		return promptEvalSingleProviderMatch(providers, func(item map[string]any) bool {
			return providerKey != "" && mapString(item, "provider_key", "provider") == providerKey
		}, "default provider account")
	}
	return promptEvalSingleProviderMatch(providers, func(item map[string]any) bool {
		return mapString(item, "id") == selector ||
			mapString(item, "provider_key", "provider") == selector ||
			mapString(item, "name", "display_name") == selector
	}, fmt.Sprintf("provider_account %q", selector))
}

func promptEvalSingleProviderMatch(providers []map[string]any, match func(map[string]any) bool, label string) (map[string]any, error) {
	matches := []map[string]any{}
	for _, item := range providers {
		if promptEvalProviderActive(item) && match(item) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%s was not found", label)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("%s matched %d provider accounts; use a provider account id", label, len(matches))
	}
}

func promptEvalProviderActive(item map[string]any) bool {
	status := strings.TrimSpace(mapString(item, "status", "lifecycle_status"))
	return status == "" || status == "active"
}

type promptEvalTestGroup struct {
	Signature string
	Tests     []promptEvalTest
}

func promptEvalTestGroups(cfg promptEvalConfig) []promptEvalTestGroup {
	bySignature := map[string][]promptEvalTest{}
	for _, test := range cfg.Tests {
		signature := promptEvalAssertionSignature(promptEvalAssertionsForTest(test))
		bySignature[signature] = append(bySignature[signature], test)
	}
	signatures := sortedPromptEvalKeys(promptEvalBoolMapKeys(bySignature))
	groups := make([]promptEvalTestGroup, 0, len(signatures))
	for _, signature := range signatures {
		groups = append(groups, promptEvalTestGroup{Signature: signature, Tests: bySignature[signature]})
	}
	return groups
}

func promptEvalBoolMapKeys(values map[string][]promptEvalTest) map[string]bool {
	keys := map[string]bool{}
	for key := range values {
		keys[key] = true
	}
	return keys
}

func promptEvalPlaygroundName(configName, signature string, groupCount int) string {
	name := "Prompt Eval: " + strings.TrimSpace(configName)
	if groupCount > 1 {
		name += " [" + signature + "]"
	}
	return name
}

func promptEvalItemsByName(items []map[string]any, name string) []map[string]any {
	var matches []map[string]any
	for _, item := range items {
		if mapString(item, "name") == name {
			matches = append(matches, item)
		}
	}
	return matches
}

func promptEvalCompareRemoteTestCases(cmd *cobra.Command, rc *RunContext, playgroundID string, group promptEvalTestGroup, pg *promptEvalRemotePlayground, result *promptEvalValidationResult) {
	items, err := promptEvalListRemoteItems(cmd, rc, fmt.Sprintf("/v1/playgrounds/%s/test-cases", playgroundID))
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	remoteByKey := map[string]map[string]any{}
	for _, item := range items {
		remoteByKey[mapString(item, "case_key")] = item
	}
	expectedKeys := map[string]bool{}
	for _, test := range group.Tests {
		expectedKeys[test.Key] = true
		remote, exists := remoteByKey[test.Key]
		if !exists {
			pg.TestsCreate++
			continue
		}
		if promptEvalCanonical(remote["variables"]) == promptEvalCanonical(test.Vars) &&
			promptEvalCanonical(remote["expectations"]) == promptEvalCanonical(promptEvalExpectationsForTest(test)) {
			pg.TestsNoop++
			continue
		}
		pg.TestsUpdate++
	}
	for _, item := range items {
		key := mapString(item, "case_key")
		if key != "" && !expectedKeys[key] {
			pg.TestsOrphan++
		}
	}
}

func promptEvalExpectationsForTest(test promptEvalTest) map[string]any {
	assertions := make([]map[string]any, 0, len(promptEvalAssertionsForTest(test)))
	for _, assertion := range promptEvalAssertionsForTest(test) {
		assertions = append(assertions, map[string]any{
			"type":     normalizePromptEvalAssertionType(assertion.Type),
			"expected": promptEvalAssertionValueString(assertion.Value),
			"metric":   strings.TrimSpace(assertion.Metric),
		})
	}
	out := map[string]any{"prompt_eval_assertions": assertions}
	if test.Expect.Output != nil {
		out["output"] = test.Expect.Output
	}
	return out
}

func promptEvalCanonical(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func validatePromptEvalAssertion(testIndex, assertionIndex int, assertion promptEvalAssertion, result *promptEvalValidationResult) {
	kind := normalizePromptEvalAssertionType(assertion.Type)
	if kind == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].assert[%d].type %q is unsupported", testIndex, assertionIndex, assertion.Type))
		return
	}
	value := strings.TrimSpace(promptEvalAssertionValueString(assertion.Value))
	switch kind {
	case "regex_match":
		if value == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].assert[%d] regex_match requires a non-empty pattern", testIndex, assertionIndex))
			return
		}
		if _, err := regexp.Compile(value); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].assert[%d] has invalid RE2 regex: %v", testIndex, assertionIndex, err))
		}
	case "json_schema":
		if value == "" || value == "<nil>" {
			result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].assert[%d] json_schema requires a non-empty schema", testIndex, assertionIndex))
			return
		}
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].assert[%d] json_schema value must be JSON: %v", testIndex, assertionIndex, err))
		}
	}
}

func promptEvalTemplateVars(template string) ([]string, []string) {
	errors := []string{}
	if strings.Contains(template, "{%") || strings.Contains(template, "{#") {
		errors = append(errors, "prompt.template uses unsupported template control syntax")
	}
	re := regexp.MustCompile(`{{\s*([^{}]+?)\s*}}`)
	matches := re.FindAllStringSubmatch(template, -1)
	vars := map[string]bool{}
	for _, match := range matches {
		name := strings.TrimSpace(match[1])
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`).MatchString(name) {
			errors = append(errors, fmt.Sprintf("prompt.template variable %q is unsupported; use simple {{var}} interpolation", name))
			continue
		}
		vars[name] = true
	}
	return sortedPromptEvalKeys(vars), errors
}

func promptEvalAssertionsForTest(test promptEvalTest) []promptEvalAssertion {
	if len(test.Assert) > 0 {
		return test.Assert
	}
	if test.Expect.Output != nil && strings.TrimSpace(fmt.Sprint(test.Expect.Output)) != "" {
		return []promptEvalAssertion{{Type: "exact_match", Value: test.Expect.Output, Metric: "correctness"}}
	}
	return nil
}

func promptEvalAssertionSignature(assertions []promptEvalAssertion) string {
	parts := make([]string, 0, len(assertions))
	for _, assertion := range assertions {
		parts = append(parts, normalizePromptEvalAssertionType(assertion.Type)+"|"+strings.TrimSpace(assertion.Metric))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])[:12]
}

func promptEvalAssertionValueString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}

func normalizePromptEvalAssertionType(kind string) string {
	switch strings.TrimSpace(kind) {
	case "exact_match", "equals":
		return "exact_match"
	case "contains":
		return "contains"
	case "regex_match", "regex":
		return "regex_match"
	case "json_schema":
		return "json_schema"
	case "json_path_match":
		return "json_path_match"
	case "boolean_assert":
		return "boolean_assert"
	default:
		return ""
	}
}

func renderPromptEvalValidation(rc *RunContext, result promptEvalValidationResult) {
	if result.Valid {
		rc.Output.PrintSuccess(fmt.Sprintf("Prompt eval config is valid (%d models x %d tests = %d cases)", result.ModelCount, result.TestCount, result.CaseCount))
	} else {
		rc.Output.PrintError("Prompt eval config has errors")
	}
	for _, warning := range result.Warnings {
		rc.Output.PrintWarning(warning)
	}
	for _, msg := range result.Errors {
		fmt.Fprintf(os.Stderr, "  - %s\n", msg)
	}
	if len(result.AssertionSignatures) > 0 {
		cols := []output.Column{{Header: "Assertion Signatures"}}
		rows := make([][]string, len(result.AssertionSignatures))
		for i, signature := range result.AssertionSignatures {
			rows[i] = []string{signature}
		}
		rc.Output.PrintTable(cols, rows)
	}
}

func sortedPromptEvalKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func defaultPromptEvalName(targetPath string) string {
	base := filepath.Base(targetPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" || name == "prompt-eval" {
		return "starter-prompt-eval"
	}
	return slugifyChallengePackName(name)
}

func floatPtrPromptEval(value float64) *float64 {
	return &value
}
