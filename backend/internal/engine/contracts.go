package engine

import "github.com/agentclash/agentclash/runtime/runner"

type StopReason = runner.StopReason

const (
	StopReasonCompleted     = runner.StopReasonCompleted
	StopReasonTimeout       = runner.StopReasonTimeout
	StopReasonStepLimit     = runner.StopReasonStepLimit
	StopReasonToolLimit     = runner.StopReasonToolLimit
	StopReasonProviderError = runner.StopReasonProviderError
	StopReasonSandboxError  = runner.StopReasonSandboxError
	StopReasonObserverError = runner.StopReasonObserverError
)

type Result = runner.Result
type Failure = runner.Failure
type PostExecutionVerificationResult = runner.PostExecutionVerificationResult
type StandingsInjection = runner.StandingsInjection
type Observer = runner.Observer
type NoopObserver = runner.NoopObserver
type SecretsLookup = runner.SecretsLookup
type AssetContent = runner.AssetContent
type AssetLoader = runner.AssetLoader

func NewFailure(stopReason StopReason, message string, cause error) error {
	return runner.NewFailure(stopReason, message, cause)
}

func AsFailure(err error) (Failure, bool) {
	return runner.AsFailure(err)
}
