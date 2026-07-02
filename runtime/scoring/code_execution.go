package scoring

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const DefaultCodeExecutionTimeoutMS int64 = 30000

type CodeExecutionScoringMode string

const (
	CodeExecutionScoringFractionPassed CodeExecutionScoringMode = "fraction_passed"
	CodeExecutionScoringAllOrNothing   CodeExecutionScoringMode = "all_or_nothing"
	CodeExecutionScoringPassAtK        CodeExecutionScoringMode = "pass_at_k"
)

type CodeExecutionConfig struct {
	TestCommand   string                   `json:"test_command"`
	TimeoutMS     *int64                   `json:"timeout_ms,omitempty"`
	Scoring       CodeExecutionScoringMode `json:"scoring,omitempty"`
	PassThreshold *float64                 `json:"pass_threshold,omitempty"`
}

type CodeExecutionResult struct {
	ValidatorKey   string   `json:"validator_key"`
	Target         string   `json:"target"`
	TargetPath     string   `json:"target_path,omitempty"`
	TestCommand    string   `json:"test_command"`
	TimeoutMS      int64    `json:"timeout_ms"`
	ExitCode       *int     `json:"exit_code,omitempty"`
	Stdout         string   `json:"stdout,omitempty"`
	Stderr         string   `json:"stderr,omitempty"`
	TimedOut       bool     `json:"timed_out,omitempty"`
	ExecutionError string   `json:"execution_error,omitempty"`
	PassedTests    *int     `json:"passed_tests,omitempty"`
	FailedTests    *int     `json:"failed_tests,omitempty"`
	ErrorTests     *int     `json:"error_tests,omitempty"`
	TotalTests     *int     `json:"total_tests,omitempty"`
	PassThreshold  *float64 `json:"pass_threshold,omitempty"`
	Scoring        string   `json:"scoring,omitempty"`
}

func (m CodeExecutionScoringMode) IsValid() bool {
	switch m {
	case CodeExecutionScoringFractionPassed, CodeExecutionScoringAllOrNothing, CodeExecutionScoringPassAtK:
		return true
	default:
		return false
	}
}

func ParseCodeExecutionConfig(raw json.RawMessage) (CodeExecutionConfig, error) {
	cfg := CodeExecutionConfig{
		Scoring: CodeExecutionScoringFractionPassed,
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return cfg, nil
	}
	if err := strictUnmarshal(raw, &cfg); err != nil {
		return CodeExecutionConfig{}, err
	}
	if cfg.Scoring == "" {
		cfg.Scoring = CodeExecutionScoringFractionPassed
	}
	cfg.TestCommand = strings.TrimSpace(cfg.TestCommand)
	return cfg, nil
}

func (c CodeExecutionConfig) EffectiveTimeoutMS() int64 {
	if c.TimeoutMS != nil && *c.TimeoutMS > 0 {
		return *c.TimeoutMS
	}
	return DefaultCodeExecutionTimeoutMS
}

func (c CodeExecutionConfig) EffectiveTimeout() time.Duration {
	return time.Duration(c.EffectiveTimeoutMS()) * time.Millisecond
}

func (c CodeExecutionConfig) EffectivePassThreshold() float64 {
	if c.PassThreshold != nil {
		return *c.PassThreshold
	}
	return 1.0
}

func ComputeCodeExecutionScore(result CodeExecutionResult, cfg CodeExecutionConfig) (*float64, string, OutputState) {
	if strings.TrimSpace(result.ExecutionError) != "" {
		return nil, firstNonEmpty(result.ExecutionError, "code execution failed to start"), OutputStateError
	}
	if result.TimedOut {
		return floatPtr(0), "test command timed out", OutputStateAvailable
	}

	total, countsAvailable := result.derivedTotalTests()
	switch cfg.Scoring {
	case CodeExecutionScoringFractionPassed:
		if countsAvailable && total > 0 && result.PassedTests != nil {
			score := float64(*result.PassedTests) / float64(total)
			return floatPtr(score), summarizeCodeExecutionCounts(result, total), OutputStateAvailable
		}
	case CodeExecutionScoringAllOrNothing:
		if countsAvailable && total > 0 {
			score := 0.0
			failed := 0
			errored := 0
			if result.FailedTests != nil {
				failed = *result.FailedTests
			}
			if result.ErrorTests != nil {
				errored = *result.ErrorTests
			}
			if failed == 0 && errored == 0 {
				score = 1.0
			}
			return floatPtr(score), summarizeCodeExecutionCounts(result, total), OutputStateAvailable
		}
	case CodeExecutionScoringPassAtK:
		return nil, "pass_at_k requires multi-sample execution and is not supported for single run-agent scoring", OutputStateError
	}

	if result.ExitCode != nil {
		score := 0.0
		reason := fmt.Sprintf("test command exited with code %d", *result.ExitCode)
		if *result.ExitCode == 0 {
			score = 1.0
			reason = "test command exited with code 0"
		}
		return floatPtr(score), reason, OutputStateAvailable
	}

	return nil, "code execution result is unavailable", OutputStateUnavailable
}

func ParseCodeExecutionCounts(stdout string, stderr string) (passed int, failed int, errored int, total int, ok bool) {
	combined := strings.TrimSpace(stdout + "\n" + stderr)
	if combined == "" {
		return 0, 0, 0, 0, false
	}

	if passed, failed, errored, total, ok = parseCodeExecutionCountsFromJSON(combined); ok {
		return passed, failed, errored, total, true
	}
	if passed, failed, errored, total, ok = parseCodeExecutionCountsFromJUnitXML(combined); ok {
		return passed, failed, errored, total, true
	}
	if passed, failed, errored, total, ok = parseCodeExecutionCountsFromText(combined); ok {
		return passed, failed, errored, total, true
	}
	return 0, 0, 0, 0, false
}

func summarizeCodeExecutionCounts(result CodeExecutionResult, total int) string {
	parts := []string{}
	if result.PassedTests != nil {
		parts = append(parts, fmt.Sprintf("%d passed", *result.PassedTests))
	}
	if result.FailedTests != nil && *result.FailedTests > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", *result.FailedTests))
	}
	if result.ErrorTests != nil && *result.ErrorTests > 0 {
		label := "errors"
		if *result.ErrorTests == 1 {
			label = "error"
		}
		parts = append(parts, fmt.Sprintf("%d %s", *result.ErrorTests, label))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d tests observed", total)
	}
	return fmt.Sprintf("%s (%d total)", strings.Join(parts, ", "), total)
}

func (r CodeExecutionResult) derivedTotalTests() (int, bool) {
	if r.TotalTests != nil && *r.TotalTests > 0 {
		return *r.TotalTests, true
	}
	total := 0
	seen := false
	if r.PassedTests != nil {
		total += *r.PassedTests
		seen = true
	}
	if r.FailedTests != nil {
		total += *r.FailedTests
		seen = true
	}
	if r.ErrorTests != nil {
		total += *r.ErrorTests
		seen = true
	}
	if !seen || total <= 0 {
		return 0, false
	}
	return total, true
}

func parseCodeExecutionCountsFromJSON(output string) (passed int, failed int, errored int, total int, ok bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return 0, 0, 0, 0, false
	}

	var found bool
	if value, exists := looseInt(payload, "passed"); exists {
		passed = value
		found = true
	}
	if value, exists := looseInt(payload, "failed"); exists {
		failed = value
		found = true
	}
	if value, exists := looseInt(payload, "errors"); exists {
		errored = value
		found = true
	} else if value, exists := looseInt(payload, "error"); exists {
		errored = value
		found = true
	}
	if value, exists := looseInt(payload, "total"); exists {
		total = value
		found = true
	}
	if !found {
		return 0, 0, 0, 0, false
	}
	if total <= 0 {
		total = passed + failed + errored
	}
	if total <= 0 {
		return 0, 0, 0, 0, false
	}
	return passed, failed, errored, total, true
}

func parseCodeExecutionCountsFromJUnitXML(output string) (passed int, failed int, errored int, total int, ok bool) {
	type testSuite struct {
		Tests    int `xml:"tests,attr"`
		Failures int `xml:"failures,attr"`
		Errors   int `xml:"errors,attr"`
	}
	type testSuites struct {
		Tests    int         `xml:"tests,attr"`
		Failures int         `xml:"failures,attr"`
		Errors   int         `xml:"errors,attr"`
		Suites   []testSuite `xml:"testsuite"`
	}

	trimmed := strings.TrimSpace(output)
	if trimmed == "" || trimmed[0] != '<' {
		return 0, 0, 0, 0, false
	}

	var suite testSuite
	if err := xml.Unmarshal([]byte(trimmed), &suite); err == nil && suite.Tests > 0 {
		total = suite.Tests
		failed = suite.Failures
		errored = suite.Errors
		passed = total - failed - errored
		return passed, failed, errored, total, true
	}

	var suites testSuites
	if err := xml.Unmarshal([]byte(trimmed), &suites); err != nil {
		return 0, 0, 0, 0, false
	}
	if suites.Tests > 0 {
		total = suites.Tests
		failed = suites.Failures
		errored = suites.Errors
		passed = total - failed - errored
		return passed, failed, errored, total, true
	}

	for _, item := range suites.Suites {
		total += item.Tests
		failed += item.Failures
		errored += item.Errors
	}
	if total <= 0 {
		return 0, 0, 0, 0, false
	}
	passed = total - failed - errored
	return passed, failed, errored, total, true
}

var testCountPattern = regexp.MustCompile(`(?i)(\d+)\s+(passed|failed|error|errors)\b`)

func parseCodeExecutionCountsFromText(output string) (passed int, failed int, errored int, total int, ok bool) {
	matches := testCountPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, 0, 0, 0, false
	}
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		value := mustAtoi(match[1])
		switch strings.ToLower(match[2]) {
		case "passed":
			passed += value
		case "failed":
			failed += value
		case "error", "errors":
			errored += value
		}
	}
	total = passed + failed + errored
	return passed, failed, errored, total, total > 0
}

func looseInt(payload map[string]any, key string) (int, bool) {
	value, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func mustAtoi(value string) int {
	result := 0
	for _, ch := range value {
		result = result*10 + int(ch-'0')
	}
	return result
}
