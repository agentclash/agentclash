package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

// --- build (structured go build / compile) ---

type Build struct{}

func (t *Build) Name() string        { return "build" }
func (t *Build) Category() Category  { return CatBuild }
func (t *Build) Description() string {
	return "Build/compile the project in the workspace. Returns structured build output with success/failure and error details."
}
func (t *Build) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{
				"type":        "string",
				"description": "Build target (e.g. './...' for Go, '.' for current dir). Defaults to './...'",
			},
		},
		"required": []string{},
	}
}

func (t *Build) Execute(workDir string, args map[string]any) (string, error) {
	target := "./..."
	if tgt, ok := args["target"].(string); ok && tgt != "" {
		target = tgt
	}

	cmd := exec.Command("go", "build", target)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	result := strings.TrimSpace(string(output))
	if err != nil {
		if result == "" {
			result = err.Error()
		}
		return fmt.Sprintf("BUILD FAILED\n%s", result), fmt.Errorf("build failed: %s", result)
	}

	if result == "" {
		result = "BUILD SUCCESS — no errors"
	}
	return result, nil
}

// --- run_tests (structured go test) ---

type RunTests struct{}

func (t *RunTests) Name() string        { return "run_tests" }
func (t *RunTests) Category() Category  { return CatBuild }
func (t *RunTests) Description() string {
	return "Run the project's test suite. Returns structured output with pass/fail status and failure details."
}
func (t *RunTests) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{
				"type":        "string",
				"description": "Test target (e.g. './...' for all, './pkg/auth' for specific). Defaults to './...'",
			},
			"verbose": map[string]any{
				"type":        "boolean",
				"description": "Run tests in verbose mode. Defaults to false.",
			},
		},
		"required": []string{},
	}
}

func (t *RunTests) Execute(workDir string, args map[string]any) (string, error) {
	target := "./..."
	if tgt, ok := args["target"].(string); ok && tgt != "" {
		target = tgt
	}

	cmdArgs := []string{"test"}
	if verbose, ok := args["verbose"].(bool); ok && verbose {
		cmdArgs = append(cmdArgs, "-v")
	}
	cmdArgs = append(cmdArgs, "-count=1", target)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	result := strings.TrimSpace(string(output))

	// Cap output
	if len(result) > 15000 {
		result = result[:15000] + "\n... (output truncated)"
	}

	if err != nil {
		return fmt.Sprintf("TESTS FAILED\n%s", result), fmt.Errorf("tests failed")
	}

	return fmt.Sprintf("TESTS PASSED\n%s", result), nil
}
