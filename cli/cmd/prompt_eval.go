package cmd

import (
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
		result := validatePromptEvalFile(path, maxCases)
		if rc.Output.IsStructured() {
			if err := rc.Output.PrintRaw(result); err != nil {
				return err
			}
		} else {
			renderPromptEvalValidation(rc, result)
		}
		if !result.Valid {
			return &ExitCodeError{Code: promptEvalExitInvalid, Message: strings.Join(result.Errors, "; ")}
		}
		return nil
	},
}

type promptEvalConfig struct {
	SchemaVersion int                    `yaml:"schemaVersion" json:"schemaVersion"`
	Name          string                 `yaml:"name" json:"name"`
	Prompt        promptEvalPrompt       `yaml:"prompt" json:"prompt"`
	Models        []promptEvalModel      `yaml:"models" json:"models"`
	Tests         []promptEvalTest       `yaml:"tests" json:"tests"`
	Thresholds    promptEvalThresholds   `yaml:"thresholds" json:"thresholds"`
	Extra         map[string]interface{} `yaml:",inline" json:"-"`
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
	Key    string                 `yaml:"key" json:"key"`
	Vars   map[string]any         `yaml:"vars" json:"vars"`
	Expect promptEvalExpect       `yaml:"expect" json:"expect"`
	Assert []promptEvalAssertion  `yaml:"assert" json:"assert"`
	Extra  map[string]interface{} `yaml:",inline" json:"-"`
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
	SchemaVersion       int      `json:"schemaVersion" yaml:"schemaVersion"`
	Path                string   `json:"path" yaml:"path"`
	Valid               bool     `json:"valid" yaml:"valid"`
	Errors              []string `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings            []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	ModelCount          int      `json:"model_count" yaml:"model_count"`
	TestCount           int      `json:"test_count" yaml:"test_count"`
	CaseCount           int      `json:"case_count" yaml:"case_count"`
	MaxCases            int      `json:"max_cases" yaml:"max_cases"`
	AssertionSignatures []string `json:"assertion_signatures" yaml:"assertion_signatures"`
	ExitCode            int      `json:"exit_code" yaml:"exit_code"`
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
	result := promptEvalValidationResult{
		SchemaVersion: promptEvalSchemaVersion,
		Path:          path,
		Valid:         true,
		MaxCases:      maxCases,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("reading file: %v", err))
		result.ExitCode = promptEvalExitInvalid
		return result
	}
	var cfg promptEvalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("parsing yaml: %v", err))
		result.ExitCode = promptEvalExitInvalid
		return result
	}
	result.ModelCount = len(cfg.Models)
	result.TestCount = len(cfg.Tests)
	result.CaseCount = len(cfg.Models) * len(cfg.Tests)

	validatePromptEvalConfig(cfg, maxCases, &result)
	result.Valid = len(result.Errors) == 0
	if !result.Valid {
		result.ExitCode = promptEvalExitInvalid
	}
	return result
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

	seenTests := map[string]bool{}
	signatures := map[string]bool{}
	for i, test := range cfg.Tests {
		key := strings.TrimSpace(test.Key)
		if key == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].key is required", i))
		} else if seenTests[key] {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate test key %q", key))
		}
		seenTests[key] = true
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

func validatePromptEvalAssertion(testIndex, assertionIndex int, assertion promptEvalAssertion, result *promptEvalValidationResult) {
	kind := normalizePromptEvalAssertionType(assertion.Type)
	if kind == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("tests[%d].assert[%d].type %q is unsupported", testIndex, assertionIndex, assertion.Type))
		return
	}
	value := strings.TrimSpace(promptEvalAssertionValueString(assertion.Value))
	switch kind {
	case "regex_match":
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
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`).MatchString(name) {
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
