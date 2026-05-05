package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	promptEvalSchemaVersion = 1
	promptEvalExitInvalid   = 5
	promptEvalExitGate      = 3
	promptEvalExitExecution = 4
)

func init() {
	rootCmd.AddCommand(promptEvalCmd)
	promptEvalCmd.AddCommand(promptEvalInitCmd)
	promptEvalCmd.AddCommand(promptEvalValidateCmd)
	promptEvalCmd.AddCommand(promptEvalRunCmd)
	promptEvalCmd.AddCommand(promptEvalResultsCmd)

	promptEvalInitCmd.Flags().Bool("force", false, "Overwrite an existing file")
	promptEvalInitCmd.Flags().String("name", "", "Prompt eval name (defaults from the file name)")

	promptEvalValidateCmd.Flags().Int("max-cases", 100, "Maximum model x test cases allowed before launch")
	promptEvalValidateCmd.Flags().Bool("remote", false, "Validate referenced AgentClash workspace resources without creating them")
	promptEvalValidateCmd.Flags().Bool("ci", false, "Apply CI-safe validation rules")

	promptEvalRunCmd.Flags().Int("max-cases", 100, "Maximum model x test cases allowed before launch")
	promptEvalRunCmd.Flags().Bool("ci", false, "Apply CI-safe validation rules")
	promptEvalRunCmd.Flags().Bool("follow", false, "Wait for launched experiments and print results")
	promptEvalRunCmd.Flags().Duration("poll-interval", 3*time.Second, "Polling interval while following experiments")
	promptEvalRunCmd.Flags().Duration("timeout", 20*time.Minute, "Maximum time to wait while following experiments; 0 disables the timeout")
	promptEvalRunCmd.Flags().Float64("threshold", -1, "Override thresholds.assertion_pass_rate for this run")
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
		if result.Valid && ciMode && !remote {
			result.Valid = false
			result.Errors = append(result.Errors, "--ci requires --remote so CI-safe provider and workspace checks run")
			result.ExitCode = promptEvalExitInvalid
		}
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

var promptEvalRunCmd = &cobra.Command{
	Use:   "run [file]",
	Short: "Compile a prompt eval config and launch playground experiments",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		path := ".agentclash/prompt-eval.yaml"
		if len(args) > 0 {
			path = args[0]
		}
		maxCases, _ := cmd.Flags().GetInt("max-cases")
		ciMode, _ := cmd.Flags().GetBool("ci")
		follow, _ := cmd.Flags().GetBool("follow")
		pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		threshold, _ := cmd.Flags().GetFloat64("threshold")
		result, err := executePromptEvalRun(cmd, rc, path, maxCases, ciMode)
		if err == nil && follow && result != nil {
			err = followPromptEvalRun(cmd, rc, result, promptEvalFollowOptions{PollInterval: pollInterval, Timeout: timeout, ThresholdOverride: threshold})
		}
		if rc.Output.IsStructured() {
			if result != nil {
				if printErr := rc.Output.PrintRaw(result); printErr != nil {
					return printErr
				}
			}
		} else if result != nil {
			renderPromptEvalRun(rc, *result)
		}
		if err != nil {
			return err
		}
		return nil
	},
}

var promptEvalResultsCmd = &cobra.Command{
	Use:   "results <experiment-id>",
	Short: "Fetch prompt eval experiment results",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		envelope, err := fetchPromptEvalResultsEnvelope(cmd, rc, args[0], -1)
		if rc.Output.IsStructured() {
			if printErr := rc.Output.PrintRaw(envelope); printErr != nil {
				return printErr
			}
		} else {
			renderPromptEvalResults(rc, envelope)
		}
		if err != nil {
			return err
		}
		return promptEvalExitForResults(envelope)
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

type promptEvalRunResult struct {
	SchemaVersion int                         `json:"schemaVersion" yaml:"schemaVersion"`
	Path          string                      `json:"path" yaml:"path"`
	ConfigHash    string                      `json:"config_hash" yaml:"config_hash"`
	WorkspaceID   string                      `json:"workspace_id" yaml:"workspace_id"`
	ModelCount    int                         `json:"model_count" yaml:"model_count"`
	TestCount     int                         `json:"test_count" yaml:"test_count"`
	CaseCount     int                         `json:"case_count" yaml:"case_count"`
	Playgrounds   []promptEvalRunPlayground   `json:"playgrounds" yaml:"playgrounds"`
	Results       []promptEvalResultsEnvelope `json:"results,omitempty" yaml:"results,omitempty"`
	Summary       promptEvalResultsSummary    `json:"summary,omitempty" yaml:"summary,omitempty"`
	GateVerdict   string                      `json:"gate_verdict,omitempty" yaml:"gate_verdict,omitempty"`
	Errors        []string                    `json:"errors,omitempty" yaml:"errors,omitempty"`
	ExitCode      int                         `json:"exit_code" yaml:"exit_code"`
}

type promptEvalRunPlayground struct {
	Name          string                    `json:"name" yaml:"name"`
	Signature     string                    `json:"signature" yaml:"signature"`
	PlaygroundID  string                    `json:"playground_id" yaml:"playground_id"`
	PlaygroundURL string                    `json:"playground_url,omitempty" yaml:"playground_url,omitempty"`
	TestsCreated  int                       `json:"tests_created" yaml:"tests_created"`
	TestsUpdated  int                       `json:"tests_updated" yaml:"tests_updated"`
	TestsNoop     int                       `json:"tests_noop" yaml:"tests_noop"`
	Experiments   []promptEvalRunExperiment `json:"experiments" yaml:"experiments"`
}

type promptEvalRunExperiment struct {
	ExperimentID      string `json:"experiment_id" yaml:"experiment_id"`
	ExperimentURL     string `json:"experiment_url,omitempty" yaml:"experiment_url,omitempty"`
	ModelAliasID      string `json:"model_alias_id" yaml:"model_alias_id"`
	ProviderAccountID string `json:"provider_account_id" yaml:"provider_account_id"`
	Status            string `json:"status,omitempty" yaml:"status,omitempty"`
}

type promptEvalFollowOptions struct {
	PollInterval      time.Duration
	Timeout           time.Duration
	ThresholdOverride float64
}

type promptEvalResultsEnvelope struct {
	SchemaVersion int                      `json:"schemaVersion" yaml:"schemaVersion"`
	ExperimentID  string                   `json:"experiment_id" yaml:"experiment_id"`
	Status        string                   `json:"status,omitempty" yaml:"status,omitempty"`
	Rows          []promptEvalResultRow    `json:"rows" yaml:"rows"`
	Summary       promptEvalResultsSummary `json:"summary" yaml:"summary"`
	Thresholds    map[string]float64       `json:"thresholds,omitempty" yaml:"thresholds,omitempty"`
	GateVerdict   string                   `json:"gate_verdict" yaml:"gate_verdict"`
	Telemetry     map[string]any           `json:"telemetry,omitempty" yaml:"telemetry,omitempty"`
	Errors        []string                 `json:"errors,omitempty" yaml:"errors,omitempty"`
	ExitCode      int                      `json:"exit_code" yaml:"exit_code"`
}

type promptEvalResultRow struct {
	CaseKey      string   `json:"case_key" yaml:"case_key"`
	AssertionKey string   `json:"assertion_key,omitempty" yaml:"assertion_key,omitempty"`
	Assertion    string   `json:"assertion,omitempty" yaml:"assertion,omitempty"`
	Result       string   `json:"result" yaml:"result"`
	Score        *float64 `json:"score,omitempty" yaml:"score,omitempty"`
	Actual       string   `json:"actual,omitempty" yaml:"actual,omitempty"`
	Expected     string   `json:"expected,omitempty" yaml:"expected,omitempty"`
	LatencyMS    int64    `json:"latency_ms,omitempty" yaml:"latency_ms,omitempty"`
	Tokens       int64    `json:"tokens,omitempty" yaml:"tokens,omitempty"`
	Error        string   `json:"error,omitempty" yaml:"error,omitempty"`
}

type promptEvalResultsSummary struct {
	TotalCases        int                `json:"total_cases" yaml:"total_cases"`
	CompletedCases    int                `json:"completed_cases" yaml:"completed_cases"`
	ExecutionErrors   int                `json:"execution_errors" yaml:"execution_errors"`
	AssertionsPassed  int                `json:"assertions_passed" yaml:"assertions_passed"`
	AssertionsFailed  int                `json:"assertions_failed" yaml:"assertions_failed"`
	AssertionPassRate float64            `json:"assertion_pass_rate" yaml:"assertion_pass_rate"`
	DimensionScores   map[string]float64 `json:"dimension_scores,omitempty" yaml:"dimension_scores,omitempty"`
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
	if workspaceID == "" {
		result.Errors = append(result.Errors, "no workspace specified for --remote. Pass --workspace, set AGENTCLASH_WORKSPACE, or run agentclash link.")
		return
	}
	remote := &promptEvalRemoteValidation{WorkspaceID: workspaceID}
	result.Remote = remote

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

func executePromptEvalRun(cmd *cobra.Command, rc *RunContext, path string, maxCases int, ciMode bool) (*promptEvalRunResult, error) {
	cfg, validation := validatePromptEvalFileWithConfig(path, maxCases)
	if validation.Valid {
		validatePromptEvalRemote(cmd, rc, cfg, ciMode, &validation)
		validation.Valid = len(validation.Errors) == 0
		if !validation.Valid {
			validation.ExitCode = promptEvalExitInvalid
		}
	}
	if !validation.Valid {
		if rc.Output.IsStructured() {
			_ = rc.Output.PrintRaw(validation)
		} else {
			renderPromptEvalValidation(rc, validation)
		}
		return nil, &ExitCodeError{Code: promptEvalExitInvalid}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &promptEvalRunResult{
			SchemaVersion: promptEvalSchemaVersion,
			Path:          path,
			WorkspaceID:   validation.Remote.WorkspaceID,
			Errors:        []string{fmt.Sprintf("reading file: %v", err)},
			ExitCode:      promptEvalExitInvalid,
		}, &ExitCodeError{Code: promptEvalExitInvalid}
	}
	run := &promptEvalRunResult{
		SchemaVersion: promptEvalSchemaVersion,
		Path:          path,
		ConfigHash:    promptEvalConfigHash(data),
		WorkspaceID:   validation.Remote.WorkspaceID,
		ModelCount:    validation.ModelCount,
		TestCount:     validation.TestCount,
		CaseCount:     validation.CaseCount,
		Playgrounds:   []promptEvalRunPlayground{},
	}
	groups := promptEvalTestGroups(cfg)
	for _, group := range groups {
		compiled, err := compilePromptEvalGroup(cmd, rc, cfg, validation, group, len(groups))
		if err != nil {
			run.Errors = append(run.Errors, err.Error())
			run.ExitCode = promptEvalExitInvalid
			return run, &ExitCodeError{Code: promptEvalExitInvalid}
		}
		run.Playgrounds = append(run.Playgrounds, compiled)
	}
	return run, nil
}

func compilePromptEvalGroup(cmd *cobra.Command, rc *RunContext, cfg promptEvalConfig, validation promptEvalValidationResult, group promptEvalTestGroup, groupCount int) (promptEvalRunPlayground, error) {
	name := promptEvalPlaygroundName(cfg.Name, group.Signature, groupCount)
	playgroundID, playgroundURL, err := upsertPromptEvalPlayground(cmd, rc, validation.Remote.WorkspaceID, name, cfg.Prompt.Template, promptEvalEvaluationSpec(cfg.Name, group))
	if err != nil {
		return promptEvalRunPlayground{}, err
	}
	result := promptEvalRunPlayground{
		Name:          name,
		Signature:     group.Signature,
		PlaygroundID:  playgroundID,
		PlaygroundURL: playgroundURL,
		Experiments:   []promptEvalRunExperiment{},
	}
	created, updated, noop, err := upsertPromptEvalTestCases(cmd, rc, playgroundID, group)
	if err != nil {
		return promptEvalRunPlayground{}, err
	}
	result.TestsCreated = created
	result.TestsUpdated = updated
	result.TestsNoop = noop
	for _, model := range validation.Remote.Models {
		experiment, err := createPromptEvalExperiment(cmd, rc, playgroundID, name, model)
		if err != nil {
			return promptEvalRunPlayground{}, err
		}
		result.Experiments = append(result.Experiments, experiment)
	}
	return result, nil
}

func upsertPromptEvalPlayground(cmd *cobra.Command, rc *RunContext, workspaceID, name, promptTemplate string, evaluationSpec map[string]any) (string, string, error) {
	playgrounds, err := promptEvalListRemoteItems(cmd, rc, fmt.Sprintf("/v1/workspaces/%s/playgrounds", workspaceID))
	if err != nil {
		return "", "", err
	}
	matches := promptEvalItemsByName(playgrounds, name)
	body := map[string]any{"name": name, "prompt_template": promptTemplate, "evaluation_spec": evaluationSpec}
	switch len(matches) {
	case 0:
		resp, err := rc.Client.Post(cmd.Context(), fmt.Sprintf("/v1/workspaces/%s/playgrounds", workspaceID), body)
		if err != nil {
			return "", "", err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return "", "", apiErr
		}
		var created map[string]any
		if err := resp.DecodeJSON(&created); err != nil {
			return "", "", err
		}
		return mapString(created, "id"), promptEvalUIURL(created, "playground", mapString(created, "id")), nil
	case 1:
		id := mapString(matches[0], "id")
		resp, err := rc.Client.Patch(cmd.Context(), "/v1/playgrounds/"+id, body)
		if err != nil {
			return "", "", err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return "", "", apiErr
		}
		var updated map[string]any
		if err := resp.DecodeJSON(&updated); err != nil {
			return "", "", err
		}
		if got := mapString(updated, "id"); got != "" {
			id = got
		}
		return id, promptEvalUIURL(updated, "playground", id), nil
	default:
		return "", "", fmt.Errorf("multiple playgrounds named %q; clean up duplicates before running prompt-eval", name)
	}
}

func upsertPromptEvalTestCases(cmd *cobra.Command, rc *RunContext, playgroundID string, group promptEvalTestGroup) (int, int, int, error) {
	items, err := promptEvalListRemoteItems(cmd, rc, fmt.Sprintf("/v1/playgrounds/%s/test-cases", playgroundID))
	if err != nil {
		return 0, 0, 0, err
	}
	remoteByKey := map[string]map[string]any{}
	for _, item := range items {
		remoteByKey[mapString(item, "case_key")] = item
	}
	var created, updated, noop int
	for _, test := range group.Tests {
		body := promptEvalTestCaseBody(test)
		remote, exists := remoteByKey[test.Key]
		if !exists {
			resp, err := rc.Client.Post(cmd.Context(), fmt.Sprintf("/v1/playgrounds/%s/test-cases", playgroundID), body)
			if err != nil {
				return 0, 0, 0, err
			}
			if apiErr := resp.ParseError(); apiErr != nil {
				return 0, 0, 0, apiErr
			}
			created++
			continue
		}
		if promptEvalCanonical(remote["variables"]) == promptEvalCanonical(test.Vars) &&
			promptEvalCanonical(remote["expectations"]) == promptEvalCanonical(promptEvalExpectationsForTest(test)) {
			noop++
			continue
		}
		resp, err := rc.Client.Patch(cmd.Context(), "/v1/playground-test-cases/"+mapString(remote, "id"), body)
		if err != nil {
			return 0, 0, 0, err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return 0, 0, 0, apiErr
		}
		updated++
	}
	return created, updated, noop, nil
}

func createPromptEvalExperiment(cmd *cobra.Command, rc *RunContext, playgroundID, playgroundName string, model promptEvalRemoteModel) (promptEvalRunExperiment, error) {
	body := map[string]any{
		"name":                playgroundName,
		"provider_account_id": model.ProviderAccountID,
		"model_alias_id":      model.ModelAliasID,
	}
	resp, err := rc.Client.Post(cmd.Context(), fmt.Sprintf("/v1/playgrounds/%s/experiments", playgroundID), body)
	if err != nil {
		return promptEvalRunExperiment{}, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return promptEvalRunExperiment{}, apiErr
	}
	var created map[string]any
	if err := resp.DecodeJSON(&created); err != nil {
		return promptEvalRunExperiment{}, err
	}
	id := mapString(created, "id")
	return promptEvalRunExperiment{
		ExperimentID:      id,
		ExperimentURL:     promptEvalUIURL(created, "experiment", id),
		ModelAliasID:      model.ModelAliasID,
		ProviderAccountID: model.ProviderAccountID,
		Status:            mapString(created, "status"),
	}, nil
}

func followPromptEvalRun(cmd *cobra.Command, rc *RunContext, run *promptEvalRunResult, options promptEvalFollowOptions) error {
	if options.PollInterval <= 0 {
		options.PollInterval = 3 * time.Second
	}
	started := time.Now()
	for {
		allTerminal := true
		var envelopes []promptEvalResultsEnvelope
		for pIndex := range run.Playgrounds {
			for eIndex := range run.Playgrounds[pIndex].Experiments {
				exp := &run.Playgrounds[pIndex].Experiments[eIndex]
				status, authErr, err := fetchPromptEvalExperimentStatus(cmd, rc, exp.ExperimentID)
				if authErr {
					run.ExitCode = promptEvalExitInvalid
					run.Errors = append(run.Errors, "auth failed while polling experiment "+exp.ExperimentID)
					return &ExitCodeError{Code: promptEvalExitInvalid}
				}
				if err != nil {
					run.ExitCode = promptEvalExitExecution
					run.Errors = append(run.Errors, err.Error())
					return &ExitCodeError{Code: promptEvalExitExecution}
				}
				exp.Status = status
				if status != "completed" && status != "failed" {
					allTerminal = false
					continue
				}
				envelope, err := fetchPromptEvalResultsEnvelope(cmd, rc, exp.ExperimentID, options.ThresholdOverride)
				if err != nil {
					run.ExitCode = promptEvalExitExecution
					run.Errors = append(run.Errors, err.Error())
					return &ExitCodeError{Code: promptEvalExitExecution}
				}
				envelopes = append(envelopes, envelope)
			}
		}
		if allTerminal {
			run.Results = envelopes
			run.Summary = combinePromptEvalSummaries(envelopes)
			run.GateVerdict = promptEvalCombinedGateVerdict(envelopes)
			run.ExitCode = promptEvalExitCodeForSummary(run.Summary, run.GateVerdict)
			if run.ExitCode != 0 {
				return &ExitCodeError{Code: run.ExitCode}
			}
			return nil
		}
		if options.Timeout > 0 && time.Since(started) >= options.Timeout {
			run.ExitCode = promptEvalExitExecution
			run.Errors = append(run.Errors, "timed out waiting for prompt eval experiments")
			return &ExitCodeError{Code: promptEvalExitExecution}
		}
		time.Sleep(options.PollInterval)
	}
}

func fetchPromptEvalExperimentStatus(cmd *cobra.Command, rc *RunContext, experimentID string) (string, bool, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/playground-experiments/"+experimentID, nil)
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode == 401 {
		return "", true, nil
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return "", false, apiErr
	}
	var payload map[string]any
	if err := resp.DecodeJSON(&payload); err != nil {
		return "", false, err
	}
	return mapString(payload, "status"), false, nil
}

func fetchPromptEvalResultsEnvelope(cmd *cobra.Command, rc *RunContext, experimentID string, thresholdOverride float64) (promptEvalResultsEnvelope, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/playground-experiments/"+experimentID+"/results", nil)
	envelope := promptEvalResultsEnvelope{
		SchemaVersion: promptEvalSchemaVersion,
		ExperimentID:  experimentID,
		Rows:          []promptEvalResultRow{},
		Thresholds:    map[string]float64{},
		Telemetry:     map[string]any{"fetched_at": time.Now().UTC().Format(time.RFC3339)},
	}
	if err != nil {
		envelope.Errors = append(envelope.Errors, err.Error())
		envelope.ExitCode = promptEvalExitExecution
		return envelope, err
	}
	if resp.StatusCode == 401 {
		err := fmt.Errorf("auth failed while fetching experiment results")
		envelope.Errors = append(envelope.Errors, err.Error())
		envelope.ExitCode = promptEvalExitInvalid
		return envelope, &ExitCodeError{Code: promptEvalExitInvalid}
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		envelope.Errors = append(envelope.Errors, apiErr.Error())
		envelope.ExitCode = promptEvalExitExecution
		return envelope, apiErr
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := resp.DecodeJSON(&payload); err != nil {
		envelope.Errors = append(envelope.Errors, err.Error())
		envelope.ExitCode = promptEvalExitExecution
		return envelope, err
	}
	envelope.Rows, envelope.Summary = aggregatePromptEvalRows(payload.Items)
	threshold := thresholdOverride
	if threshold < 0 {
		threshold = 1
	}
	envelope.Thresholds["assertion_pass_rate"] = threshold
	envelope.GateVerdict = "pass"
	if envelope.Summary.ExecutionErrors > 0 {
		envelope.GateVerdict = "error"
		envelope.ExitCode = promptEvalExitExecution
	} else if envelope.Summary.AssertionPassRate < threshold {
		envelope.GateVerdict = "fail"
		envelope.ExitCode = promptEvalExitGate
	}
	envelope.Telemetry["row_count"] = len(envelope.Rows)
	return envelope, nil
}

func aggregatePromptEvalRows(items []map[string]any) ([]promptEvalResultRow, promptEvalResultsSummary) {
	rows := []promptEvalResultRow{}
	summary := promptEvalResultsSummary{TotalCases: len(items), DimensionScores: map[string]float64{}}
	dimensionTotals := map[string]float64{}
	dimensionCounts := map[string]int{}
	for _, item := range items {
		caseKey := mapString(item, "case_key")
		status := mapString(item, "status")
		if status == "completed" {
			summary.CompletedCases++
		}
		if status == "failed" || mapString(item, "error_message") != "" {
			summary.ExecutionErrors++
			rows = append(rows, promptEvalResultRow{CaseKey: caseKey, Result: "ERROR", Error: mapString(item, "error_message"), Actual: truncateRunes(mapString(item, "actual_output"), 160), LatencyMS: promptEvalInt64(item["latency_ms"]), Tokens: promptEvalInt64(item["total_tokens"])})
			continue
		}
		for _, raw := range mapSlice(item, "validator_results") {
			validator, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			verdict := mapString(validator, "verdict")
			if verdict == "pass" {
				summary.AssertionsPassed++
			} else {
				summary.AssertionsFailed++
			}
			score := promptEvalFloatPtr(mapValue(validator, "normalized_score", "score"))
			rows = append(rows, promptEvalResultRow{
				CaseKey:      caseKey,
				AssertionKey: mapString(validator, "key"),
				Assertion:    mapString(validator, "type"),
				Result:       strings.ToUpper(verdict),
				Score:        score,
				Actual:       truncateRunes(mapString(item, "actual_output"), 160),
				Expected:     mapString(validator, "expected", "expected_value"),
				LatencyMS:    promptEvalInt64(item["latency_ms"]),
				Tokens:       promptEvalInt64(item["total_tokens"]),
				Error:        mapString(validator, "reason"),
			})
		}
		for key, value := range mapObject(item, "dimension_scores") {
			if f, ok := promptEvalFloat(value); ok {
				dimensionTotals[key] += f
				dimensionCounts[key]++
			}
		}
	}
	totalAssertions := summary.AssertionsPassed + summary.AssertionsFailed
	if totalAssertions > 0 {
		summary.AssertionPassRate = float64(summary.AssertionsPassed) / float64(totalAssertions)
	}
	for key, total := range dimensionTotals {
		summary.DimensionScores[key] = total / float64(dimensionCounts[key])
	}
	return rows, summary
}

func promptEvalTestCaseBody(test promptEvalTest) map[string]any {
	return map[string]any{
		"case_key":     test.Key,
		"variables":    test.Vars,
		"expectations": promptEvalExpectationsForTest(test),
	}
}

func promptEvalEvaluationSpec(configName string, group promptEvalTestGroup) map[string]any {
	assertions := promptEvalAssertionsForTest(group.Tests[0])
	validators := make([]map[string]any, 0, len(assertions))
	metricValidators := map[string][]string{}
	for i, assertion := range assertions {
		kind := normalizePromptEvalAssertionType(assertion.Type)
		key := fmt.Sprintf("%s_%s_%d", group.Signature, kind, i+1)
		metric := strings.TrimSpace(assertion.Metric)
		if metric == "" {
			metric = "correctness"
		}
		validators = append(validators, map[string]any{
			"key":           key,
			"type":          kind,
			"target":        "final_output",
			"expected_from": fmt.Sprintf("case.expectations.prompt_eval_assertions.%d.expected", i),
		})
		metricValidators[metric] = append(metricValidators[metric], key)
	}
	dimensions := make([]map[string]any, 0, len(metricValidators))
	metrics := sortedPromptEvalStringSliceKeys(metricValidators)
	for _, metric := range metrics {
		dimensions = append(dimensions, map[string]any{
			"key":              metric,
			"source":           "validators",
			"validators":       metricValidators[metric],
			"better_direction": "higher",
		})
	}
	return map[string]any{
		"name":           slugifyChallengePackName(configName) + "-" + group.Signature,
		"version_number": 1,
		"judge_mode":     "deterministic",
		"validators":     validators,
		"scorecard": map[string]any{
			"dimensions": dimensions,
		},
	}
}

func sortedPromptEvalStringSliceKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func promptEvalConfigHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func promptEvalUIURL(item map[string]any, fallbackKind, id string) string {
	if link := mapString(item, "url", "web_url", "html_url"); link != "" {
		return link
	}
	if id == "" {
		return ""
	}
	return "https://agentclash.dev/" + fallbackKind + "s/" + id
}

func promptEvalListRemoteItems(cmd *cobra.Command, rc *RunContext, path string) ([]map[string]any, error) {
	var out []map[string]any
	var cursor string
	for {
		query := url.Values{}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		resp, err := rc.Client.Get(cmd.Context(), path, query)
		if err != nil {
			return nil, err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return nil, apiErr
		}
		var payload struct {
			Items      []map[string]any `json:"items"`
			NextCursor string           `json:"next_cursor"`
		}
		if err := resp.DecodeJSON(&payload); err != nil {
			return nil, err
		}
		out = append(out, payload.Items...)
		cursor = strings.TrimSpace(payload.NextCursor)
		if cursor == "" {
			break
		}
	}
	return out, nil
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
			return nil, fmt.Errorf("default provider account %q from model alias was not found", aliasProviderID)
		}
		return nil, fmt.Errorf("model alias does not expose provider_account_id; set provider_account explicitly")
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

func renderPromptEvalRun(rc *RunContext, result promptEvalRunResult) {
	if result.ExitCode != 0 {
		rc.Output.PrintError("Prompt eval run failed")
		for _, msg := range result.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", msg)
		}
		return
	}
	rc.Output.PrintSuccess(fmt.Sprintf("Launched prompt eval (%d models x %d tests = %d cases)", result.ModelCount, result.TestCount, result.CaseCount))
	rows := [][]string{}
	for _, playground := range result.Playgrounds {
		for _, experiment := range playground.Experiments {
			rows = append(rows, []string{playground.PlaygroundID, experiment.ExperimentID, experiment.ModelAliasID, output.StatusColor(experiment.Status)})
		}
	}
	rc.Output.PrintTable([]output.Column{{Header: "Playground"}, {Header: "Experiment"}, {Header: "Model Alias"}, {Header: "Status"}}, rows)
}

func renderPromptEvalResults(rc *RunContext, result promptEvalResultsEnvelope) {
	if result.ExitCode != 0 {
		rc.Output.PrintError("Prompt eval results failed")
	}
	rows := make([][]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		score := "-"
		if row.Score != nil {
			score = fmt.Sprintf("%.2f", *row.Score)
		}
		rows = append(rows, []string{row.CaseKey, row.AssertionKey, row.Result, score, row.Error})
	}
	rc.Output.PrintTable([]output.Column{{Header: "Case"}, {Header: "Assertion"}, {Header: "Result"}, {Header: "Score"}, {Header: "Error"}}, rows)
}

func combinePromptEvalSummaries(envelopes []promptEvalResultsEnvelope) promptEvalResultsSummary {
	out := promptEvalResultsSummary{DimensionScores: map[string]float64{}}
	dimTotals := map[string]float64{}
	dimCounts := map[string]int{}
	for _, envelope := range envelopes {
		out.TotalCases += envelope.Summary.TotalCases
		out.CompletedCases += envelope.Summary.CompletedCases
		out.ExecutionErrors += envelope.Summary.ExecutionErrors
		out.AssertionsPassed += envelope.Summary.AssertionsPassed
		out.AssertionsFailed += envelope.Summary.AssertionsFailed
		for key, value := range envelope.Summary.DimensionScores {
			dimTotals[key] += value
			dimCounts[key]++
		}
	}
	totalAssertions := out.AssertionsPassed + out.AssertionsFailed
	if totalAssertions > 0 {
		out.AssertionPassRate = float64(out.AssertionsPassed) / float64(totalAssertions)
	}
	for key, total := range dimTotals {
		out.DimensionScores[key] = total / float64(dimCounts[key])
	}
	return out
}

func promptEvalCombinedGateVerdict(envelopes []promptEvalResultsEnvelope) string {
	verdict := "pass"
	for _, envelope := range envelopes {
		if envelope.GateVerdict == "error" {
			return "error"
		}
		if envelope.GateVerdict == "fail" {
			verdict = "fail"
		}
	}
	return verdict
}

func promptEvalExitForResults(envelope promptEvalResultsEnvelope) error {
	if envelope.ExitCode == 0 {
		return nil
	}
	return &ExitCodeError{Code: envelope.ExitCode}
}

func promptEvalExitCodeForSummary(summary promptEvalResultsSummary, verdict string) int {
	if summary.ExecutionErrors > 0 || verdict == "error" {
		return promptEvalExitExecution
	}
	if verdict == "fail" {
		return promptEvalExitGate
	}
	return 0
}

func promptEvalFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func promptEvalFloatPtr(value any) *float64 {
	if f, ok := promptEvalFloat(value); ok {
		return &f
	}
	return nil
}

func promptEvalInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
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
