package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
)

type StopReason string

const (
	StopReasonCompleted     StopReason = "completed"
	StopReasonTimeout       StopReason = "timeout"
	StopReasonStepLimit     StopReason = "step_limit"
	StopReasonToolLimit     StopReason = "tool_limit"
	StopReasonProviderError StopReason = "provider_error"
	StopReasonSandboxError  StopReason = "sandbox_error"
	StopReasonObserverError StopReason = "observer_error"
)

type Result struct {
	FinalOutput   string
	StopReason    StopReason
	StepCount     int
	ToolCallCount int
	Usage         provider.Usage
}

type Failure struct {
	StopReason StopReason
	Message    string
	Cause      error
}

func (f Failure) Error() string {
	if strings.TrimSpace(f.Message) != "" {
		return f.Message
	}
	return fmt.Sprintf("runtime runner stopped: %s", f.StopReason)
}

func (f Failure) Unwrap() error {
	return f.Cause
}

func NewFailure(stopReason StopReason, message string, cause error) error {
	return Failure{
		StopReason: stopReason,
		Message:    message,
		Cause:      cause,
	}
}

func AsFailure(err error) (Failure, bool) {
	var failure Failure
	if !errors.As(err, &failure) {
		return Failure{}, false
	}
	return failure, true
}

// PostExecutionVerificationResult holds captured file or directory state from
// the sandbox, emitted as a grader.verification.* event before sandbox teardown.
type PostExecutionVerificationResult struct {
	Key     string          `json:"key"`
	Type    string          `json:"type"` // "file_capture", "directory_listing", or "code_execution"
	Payload json.RawMessage `json:"payload"`
}

// StandingsInjection describes a race-context newswire message inserted
// into the agent's context at a step boundary. Observers that persist
// events should record this as `race.standings.injected` with the same
// payload shape (see runevents.RaceStandingsInjectedPayload).
type StandingsInjection struct {
	StepIndex         int
	TokensAdded       int
	StandingsSnapshot string
	TriggeredBy       runevents.RaceStandingsTrigger
	MinStepGap        int
}

type Observer interface {
	OnStepStart(ctx context.Context, step int) error
	OnProviderCall(ctx context.Context, request provider.Request) error
	OnProviderOutput(ctx context.Context, request provider.Request, delta provider.StreamDelta) error
	OnProviderResponse(ctx context.Context, response provider.Response) error
	OnToolExecution(ctx context.Context, record ToolExecutionRecord) error
	OnStepEnd(ctx context.Context, step int) error
	OnPostExecutionVerification(ctx context.Context, results []PostExecutionVerificationResult) error
	OnStandingsInjected(ctx context.Context, injection StandingsInjection) error
	OnRunComplete(ctx context.Context, result Result) error
	OnRunFailure(ctx context.Context, err error) error
}

type NoopObserver struct{}

func (NoopObserver) OnStepStart(context.Context, int) error                 { return nil }
func (NoopObserver) OnProviderCall(context.Context, provider.Request) error { return nil }
func (NoopObserver) OnProviderOutput(context.Context, provider.Request, provider.StreamDelta) error {
	return nil
}
func (NoopObserver) OnProviderResponse(context.Context, provider.Response) error { return nil }
func (NoopObserver) OnToolExecution(context.Context, ToolExecutionRecord) error  { return nil }
func (NoopObserver) OnStepEnd(context.Context, int) error                        { return nil }
func (NoopObserver) OnPostExecutionVerification(context.Context, []PostExecutionVerificationResult) error {
	return nil
}
func (NoopObserver) OnStandingsInjected(context.Context, StandingsInjection) error { return nil }
func (NoopObserver) OnRunComplete(context.Context, Result) error                   { return nil }
func (NoopObserver) OnRunFailure(context.Context, error) error                     { return nil }

// SecretsLookup resolves ${secrets.X} references at run-start by returning
// the plaintext secret map for a workspace. Hosted and local runtimes can
// provide different backing stores while sharing this contract.
type SecretsLookup interface {
	LoadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)
}

type AssetContent struct {
	Content     []byte
	ContentType string
}

// AssetLoader resolves artifact-backed challenge-pack assets before the
// sandbox starts executing.
type AssetLoader interface {
	LoadAsset(ctx context.Context, workspaceID uuid.UUID, artifactID uuid.UUID) (AssetContent, error)
}
