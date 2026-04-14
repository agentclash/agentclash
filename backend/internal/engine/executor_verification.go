package engine

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
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
