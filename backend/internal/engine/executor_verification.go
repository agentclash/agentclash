package engine

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/agentclash/agentclash/backend/internal/scoring"
)

// extractPostExecutionChecks loads post-execution check declarations from the
// challenge pack manifest's evaluation spec. Returns nil when the manifest
// cannot be parsed or contains no checks.
func extractPostExecutionChecks(executionContext repository.RunAgentExecutionContext) []scoring.PostExecutionCheck {
	if len(executionContext.ChallengePackVersion.Manifest) == 0 {
		return nil
	}
	spec, err := scoring.LoadEvaluationSpec(executionContext.ChallengePackVersion.Manifest)
	if err != nil {
		return nil
	}
	return spec.PostExecutionChecks
}

type codeExecutionCheck struct {
	ValidatorKey string
	Target       string
	TargetPath   string
	Config       scoring.CodeExecutionConfig
}

func collectPostExecutionVerification(
	ctx context.Context,
	session sandbox.Session,
	executionContext repository.RunAgentExecutionContext,
) []PostExecutionVerificationResult {
	results := []PostExecutionVerificationResult{}
	if checks := extractCodeExecutionChecks(executionContext); len(checks) > 0 {
		results = append(results, executeCodeExecutionChecks(ctx, session, checks)...)
	}
	if checks := extractPostExecutionChecks(executionContext); len(checks) > 0 {
		results = append(results, executePostExecutionChecks(ctx, session, checks)...)
	}
	return results
}

func extractCodeExecutionChecks(executionContext repository.RunAgentExecutionContext) []codeExecutionCheck {
	if len(executionContext.ChallengePackVersion.Manifest) == 0 {
		return nil
	}
	spec, err := scoring.LoadEvaluationSpec(executionContext.ChallengePackVersion.Manifest)
	if err != nil {
		return nil
	}

	postChecks := make(map[string]scoring.PostExecutionCheck, len(spec.PostExecutionChecks))
	for _, check := range spec.PostExecutionChecks {
		postChecks[check.Key] = check
	}

	results := make([]codeExecutionCheck, 0)
	for _, validator := range spec.Validators {
		if validator.Type != scoring.ValidatorTypeCodeExecution {
			continue
		}
		cfg, err := scoring.ParseCodeExecutionConfig(validator.Config)
		if err != nil {
			continue
		}
		targetKey := strings.TrimSpace(strings.TrimPrefix(validator.Target, "file:"))
		check, ok := postChecks[targetKey]
		if !ok || check.Type != scoring.PostExecutionCheckTypeFileCapture {
			continue
		}
		results = append(results, codeExecutionCheck{
			ValidatorKey: validator.Key,
			Target:       validator.Target,
			TargetPath:   check.Path,
			Config:       cfg,
		})
	}

	return results
}

// executePostExecutionChecks reads the specified files and directories from the
// sandbox session and returns results suitable for observer emission. Each check
// produces one PostExecutionVerificationResult with a JSON payload.
func executePostExecutionChecks(
	ctx context.Context,
	session sandbox.Session,
	checks []scoring.PostExecutionCheck,
) []PostExecutionVerificationResult {
	results := make([]PostExecutionVerificationResult, 0, len(checks))
	var totalCaptured int64

	for _, check := range checks {
		switch check.Type {
		case scoring.PostExecutionCheckTypeFileCapture:
			capture := executeFileCaptureCheck(ctx, session, check, totalCaptured)
			totalCaptured += capture.Size
			payload, err := json.Marshal(capture)
			if err != nil {
				slog.Default().Warn("marshal file capture result", "key", check.Key, "error", err)
				continue
			}
			results = append(results, PostExecutionVerificationResult{
				Key:     check.Key,
				Type:    scoring.PostExecutionCheckTypeFileCapture,
				Payload: payload,
			})

		case scoring.PostExecutionCheckTypeDirectoryListing:
			listing := executeDirectoryListingCheck(ctx, session, check)
			payload, err := json.Marshal(listing)
			if err != nil {
				slog.Default().Warn("marshal directory listing result", "key", check.Key, "error", err)
				continue
			}
			results = append(results, PostExecutionVerificationResult{
				Key:     check.Key,
				Type:    scoring.PostExecutionCheckTypeDirectoryListing,
				Payload: payload,
			})
		}
	}
	return results
}

func executeCodeExecutionChecks(
	ctx context.Context,
	session sandbox.Session,
	checks []codeExecutionCheck,
) []PostExecutionVerificationResult {
	results := make([]PostExecutionVerificationResult, 0, len(checks))
	for _, check := range checks {
		payload, err := json.Marshal(executeCodeExecutionCheck(ctx, session, check))
		if err != nil {
			slog.Default().Warn("marshal code execution result", "validator_key", check.ValidatorKey, "error", err)
			continue
		}
		results = append(results, PostExecutionVerificationResult{
			Key:     check.ValidatorKey,
			Type:    string(scoring.ValidatorTypeCodeExecution),
			Payload: payload,
		})
	}
	return results
}

// executeFileCaptureCheck reads a single file from the sandbox, enforcing per-file
// and total capture size limits. Files that exceed the limit are truncated.
func executeFileCaptureCheck(
	ctx context.Context,
	session sandbox.Session,
	check scoring.PostExecutionCheck,
	totalCapturedSoFar int64,
) scoring.FileCaptureResult {
	result := scoring.FileCaptureResult{
		Key:  check.Key,
		Path: check.Path,
	}

	content, err := session.ReadFile(ctx, check.Path)
	if err != nil {
		// File does not exist or is unreadable.
		result.Exists = false
		return result
	}
	result.Exists = true
	result.Size = int64(len(content))

	maxSize := check.EffectiveMaxSizeBytes()

	// Enforce total capture budget.
	remaining := scoring.DefaultMaxTotalCaptureBytes - totalCapturedSoFar
	if remaining <= 0 {
		result.Content = ""
		result.Truncated = true
		return result
	}
	if maxSize > remaining {
		maxSize = remaining
	}

	if int64(len(content)) > maxSize {
		result.Content = string(content[:maxSize])
		result.Truncated = true
	} else {
		result.Content = string(content)
	}
	return result
}

// executeDirectoryListingCheck lists files in a sandbox directory. When
// check.Recursive is false the prefix is used as-is; the sandbox ListFiles
// implementation controls depth behavior.
func executeDirectoryListingCheck(
	ctx context.Context,
	session sandbox.Session,
	check scoring.PostExecutionCheck,
) scoring.DirectoryListingResult {
	result := scoring.DirectoryListingResult{
		Key:  check.Key,
		Path: check.Path,
	}

	files, err := session.ListFiles(ctx, check.Path)
	if err != nil {
		result.Entries = []scoring.DirectoryEntry{}
		return result
	}

	entries := make([]scoring.DirectoryEntry, 0, len(files))
	for _, f := range files {
		entries = append(entries, scoring.DirectoryEntry{
			Path: f.Path,
			Size: f.Size,
		})
	}
	result.Entries = entries
	return result
}

func executeCodeExecutionCheck(
	ctx context.Context,
	session sandbox.Session,
	check codeExecutionCheck,
) scoring.CodeExecutionResult {
	result := scoring.CodeExecutionResult{
		ValidatorKey:  check.ValidatorKey,
		Target:        check.Target,
		TargetPath:    check.TargetPath,
		TestCommand:   check.Config.TestCommand,
		TimeoutMS:     check.Config.EffectiveTimeoutMS(),
		Scoring:       string(check.Config.Scoring),
		PassThreshold: check.Config.PassThreshold,
	}

	execResult, err := session.Exec(ctx, sandbox.ExecRequest{
		// Intentionally run through the sandbox shell so challenge-pack authors
		// can supply normal test commands (pipelines, env var expansion, `cd`,
		// etc.). This is safe here because the command executes inside the same
		// isolated ephemeral sandbox as the generated code under evaluation.
		Command:          []string{"sh", "-lc", check.Config.TestCommand},
		WorkingDirectory: defaultCodeExecutionWorkingDirectory(check.TargetPath),
		Timeout:          check.Config.EffectiveTimeout(),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result.TimedOut = true
			return result
		}
		result.ExecutionError = err.Error()
		return result
	}

	result.ExitCode = intPtr(execResult.ExitCode)
	result.Stdout = execResult.Stdout
	result.Stderr = execResult.Stderr

	passed, failed, errored, total, ok := scoring.ParseCodeExecutionCounts(execResult.Stdout, execResult.Stderr)
	if ok {
		result.PassedTests = intPtr(passed)
		result.FailedTests = intPtr(failed)
		result.ErrorTests = intPtr(errored)
		result.TotalTests = intPtr(total)
	}

	return result
}

func defaultCodeExecutionWorkingDirectory(targetPath string) string {
	dir := path.Dir(strings.TrimSpace(targetPath))
	if dir == "." || dir == "/" || dir == "" {
		return "/workspace"
	}
	if strings.HasPrefix(dir, "/workspace") {
		return "/workspace"
	}
	return dir
}

func intPtr(value int) *int {
	return &value
}
