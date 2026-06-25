package cmd

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(evaltestCmd)
	evaltestCmd.AddCommand(evaltestInitCmd)
	evaltestCmd.AddCommand(evaltestRunCmd)

	evaltestInitCmd.Flags().Bool("force", false, "Overwrite existing evaltest files")
	evaltestRunCmd.Flags().String("format", "json", "Output format: json, junit, or both")
	evaltestRunCmd.Flags().String("out", "agentclash-results", "Output directory for reports")
	evaltestRunCmd.Flags().String("config", ".agentclash/evaltest.yaml", "Evaltest config path")
}

var evaltestCmd = &cobra.Command{
	Use:   "evaltest",
	Short: "Run local pre-deploy agent eval tests",
}

var evaltestInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold local evaltest config and example test",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		files := map[string]string{
			".agentclash/evaltest.yaml":    evaltestConfigTemplate,
			"tests/evaltest/test_smoke.py": evaltestSmokeTestTemplate,
		}
		for path, content := range files {
			if !force {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("%s already exists; pass --force to overwrite", path)
				}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return err
			}
		}
		fmt.Fprintf(os.Stdout, "Created %s and tests/evaltest/test_smoke.py\n", ".agentclash/evaltest.yaml")
		return nil
	},
}

var evaltestRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run local eval tests and emit JSON/JUnit reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		outDir, _ := cmd.Flags().GetString("out")
		configPath, _ := cmd.Flags().GetString("config")

		if err := validateEvaltestFormat(format); err != nil {
			evaltestExit(evaltestExitConfigError)
			return err
		}
		if _, err := os.Stat(configPath); err != nil {
			evaltestExit(evaltestExitConfigError)
			return fmt.Errorf("evaltest config not found at %s; run `agentclash evaltest init` first", configPath)
		}

		report, exitCode, err := runEvaltestPython(outDir)
		if err != nil {
			if exitCode == 0 {
				exitCode = evaltestExitInternalError
			}
			evaltestExit(exitCode)
			return err
		}

		if strings.Contains(format, "json") || format == "both" {
			if err := writeEvaltestJSON(outDir, report); err != nil {
				evaltestExit(evaltestExitInternalError)
				return err
			}
		}
		if strings.Contains(format, "junit") || format == "both" {
			if err := writeEvaltestJUnit(outDir, report); err != nil {
				evaltestExit(evaltestExitInternalError)
				return err
			}
		}

		evaltestExit(exitCode)
		return nil
	},
}

func validateEvaltestFormat(format string) error {
	switch format {
	case "json", "junit", "both":
		return nil
	default:
		return fmt.Errorf("unsupported format %q; use json, junit, or both", format)
	}
}

func runEvaltestPython(outDir string) (map[string]any, int, error) {
	runnerPath, hasSourcePath, err := evaltestSDKSourcePath()
	if err != nil {
		return nil, evaltestExitConfigError, err
	}
	cmd := exec.Command("python3", "-m", "agentclash_eval.runner", "--out", outDir, "--mode", "smoke")
	cmd.Env = os.Environ()
	if hasSourcePath {
		cmd.Env = append(cmd.Env, "PYTHONPATH="+runnerPath)
	}
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Fprint(os.Stderr, string(output))
	}

	reportPath := filepath.Join(outDir, "results.json")
	data, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		if err != nil {
			if strings.Contains(string(output), "No module named agentclash_eval") {
				return nil, evaltestExitConfigError, fmt.Errorf("agentclash-evals Python package not found; install agentclash-evals or set AGENTCLASH_EVAL_SDK_SRC to the SDK src directory")
			}
			return nil, evaltestExitProviderError, fmt.Errorf("eval runner failed: %w", err)
		}
		return nil, evaltestExitInternalError, fmt.Errorf("read report: %w", readErr)
	}
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, evaltestExitInternalError, fmt.Errorf("parse report: %w", err)
	}
	exitCode := evaltestExitPass
	if raw, ok := report["exit_code"].(float64); ok {
		exitCode = int(raw)
	}
	if err != nil && exitCode == evaltestExitPass {
		exitCode = evaltestExitInternalError
	}
	return report, exitCode, nil
}

func evaltestSDKSourcePath() (string, bool, error) {
	if src := strings.TrimSpace(os.Getenv("AGENTCLASH_EVAL_SDK_SRC")); src != "" {
		if _, err := os.Stat(filepath.Join(src, "agentclash_eval", "runner.py")); err == nil {
			return src, true, nil
		}
		return "", false, fmt.Errorf("AGENTCLASH_EVAL_SDK_SRC is set but runner.py was not found under %s", src)
	}
	return "", false, nil
}

func writeEvaltestJSON(outDir string, report map[string]any) error {
	path := filepath.Join(outDir, "results.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeEvaltestJUnit(outDir string, report map[string]any) error {
	cases := evaltestCases(report)
	failures := 0
	testCases := make([]junitTestCase, 0, len(cases))
	for _, item := range cases {
		row, _ := item.(map[string]any)
		if row == nil {
			continue
		}
		caseObj, _ := row["case"].(map[string]any)
		name := mapString(caseObj, "name")
		if name == "" {
			name = mapString(caseObj, "case_id")
		}
		status := mapString(row, "status")
		tc := junitTestCase{
			Name:      name,
			Classname: "agentclash.evaltest",
		}
		if status != "passed" {
			failures++
			reason := evaltestFailureReason(row)
			tc.Failure = &junitFailure{Message: reason, Body: reason}
		}
		testCases = append(testCases, tc)
	}
	if len(testCases) == 0 {
		testCases = append(testCases, junitTestCase{Name: "evaltest", Classname: "agentclash.evaltest"})
	}
	payload := junitTestSuites{
		Tests:    len(testCases),
		Failures: failures,
		TestSuite: []junitTestSuite{{
			Name:      "agentclash-evaltest",
			Tests:     len(testCases),
			Failures:  failures,
			TestCases: testCases,
		}},
	}
	encoded, err := xml.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(outDir, "junit.xml")
	return os.WriteFile(path, append([]byte(xml.Header), encoded...), 0o644)
}

func evaltestCases(report map[string]any) []any {
	raw, _ := report["cases"].([]any)
	return raw
}

func evaltestFailureReason(row map[string]any) string {
	metrics := mapSlice(row, "metrics")
	for _, item := range metrics {
		metric, _ := item.(map[string]any)
		if metric == nil {
			continue
		}
		if passed, ok := metric["passed"].(bool); ok && !passed {
			if reason := mapString(metric, "reason"); reason != "" {
				return reason
			}
		}
	}
	if msg := mapString(row, "error"); msg != "" {
		return msg
	}
	return "eval assertion failed"
}

var evaltestExit = os.Exit

const evaltestConfigTemplate = `# AgentClash local evaltest config
schema_version: 1
# Install the local SDK first:
#   python -m pip install agentclash-evals
tests_dir: tests/evaltest
`

const evaltestSmokeTestTemplate = `from agentclash_eval import assert_agent
from agentclash_eval.metrics import Contains


def test_smoke_contains():
    assert_agent("hello world", metrics=[Contains("world")])
`
