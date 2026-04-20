package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrRunNotFound                   = errors.New("run not found")
	ErrEvalSessionNotFound           = errors.New("eval session not found")
	ErrRunAgentNotFound              = errors.New("run agent not found")
	ErrRunAgentReplayNotFound        = errors.New("run agent replay not found")
	ErrRunAgentScorecardNotFound     = errors.New("run agent scorecard not found")
	ErrRunScorecardNotFound          = errors.New("run scorecard not found")
	ErrRunComparisonNotFound         = errors.New("run comparison not found")
	ErrRegressionSuiteNotFound       = errors.New("regression suite not found")
	ErrRegressionCaseNotFound        = errors.New("regression case not found")
	ErrRegressionSuiteNameConflict   = errors.New("regression suite name already exists in workspace")
	ErrEvaluationSpecNotFound        = errors.New("evaluation spec not found")
	ErrChallengePackVersionNotFound  = errors.New("challenge pack version not found")
	ErrChallengeInputSetNotFound     = errors.New("challenge input set not found")
	ErrChallengePackVersionExists    = errors.New("challenge pack version already exists")
	ErrChallengePackMetadataConflict = errors.New("challenge pack metadata conflicts with existing pack")
	ErrInvalidTransition             = errors.New("invalid status transition")
	ErrIllegalSessionTransition      = errors.New("illegal eval session transition")
	ErrTransitionConflict            = errors.New("status transition conflict")
	ErrAttachmentConflict            = errors.New("attachment conflict")
	ErrRunAlreadyAttachedToSession   = errors.New("run already attached to an eval session")
	ErrTemporalWorkflowID            = errors.New("temporal workflow id is required")
	ErrTemporalRunID                 = errors.New("temporal run id is required")
	ErrTemporalIDConflict            = errors.New("run already has different temporal ids")
	ErrFrozenExecutionContext        = errors.New("run agent execution context is invalid")
	ErrHostedRunExecutionNotFound    = errors.New("hosted run execution not found")
	ErrArtifactNotFound              = errors.New("artifact not found")
	ErrUnexpectedFailureCause        = errors.New("failure reason is only valid for failed run-agent transitions")
	ErrRunNameRequired               = errors.New("run name is required")
	ErrRunParticipantsRequired       = errors.New("run must have at least one participant")
	ErrInvalidExecutionMode          = errors.New("invalid execution mode")
	ErrEvalSessionRepetitionsInvalid = errors.New("eval session repetitions must be >= 1")
	ErrEvalSessionSchemaVersion      = errors.New("eval session schema version must be >= 1")
	ErrRunAgentLabelRequired         = errors.New("run agent label is required")
	ErrCLITokenNotFound              = errors.New("cli token not found")
	ErrDeviceCodeNotFound            = errors.New("device code not found")
	ErrDeviceCodeExpired             = errors.New("device code expired")
	ErrWorkspaceSecretNotFound       = errors.New("workspace secret not found")
	ErrSecretsCipherUnset            = errors.New("secrets cipher is not configured")
	ErrInvalidSecretKey              = errors.New("secret key must match [A-Za-z_][A-Za-z0-9_]* and be 1..128 characters")
)

type InvalidTransitionError struct {
	Entity string
	From   string
	To     string
}

func (e InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid %s status transition: %s -> %s", e.Entity, e.From, e.To)
}

func (e InvalidTransitionError) Is(target error) bool {
	return target == ErrInvalidTransition
}

type TransitionConflictError struct {
	Entity   string
	ID       uuid.UUID
	Expected string
}

func (e TransitionConflictError) Error() string {
	return fmt.Sprintf("%s %s changed before the transition could be applied from %s", e.Entity, e.ID, e.Expected)
}

func (e TransitionConflictError) Is(target error) bool {
	return target == ErrTransitionConflict
}

type AttachmentConflictError struct {
	Entity string
	ID     uuid.UUID
}

func (e AttachmentConflictError) Error() string {
	return fmt.Sprintf("%s %s changed before the attachment could be applied", e.Entity, e.ID)
}

func (e AttachmentConflictError) Is(target error) bool {
	return target == ErrAttachmentConflict
}

type IllegalSessionTransitionError struct {
	From string
	To   string
}

func (e IllegalSessionTransitionError) Error() string {
	return fmt.Sprintf("illegal eval session transition: %s -> %s", e.From, e.To)
}

func (e IllegalSessionTransitionError) Is(target error) bool {
	return target == ErrIllegalSessionTransition
}

type RunAlreadyAttachedToSessionError struct {
	RunID                  uuid.UUID
	ExistingEvalSessionID  uuid.UUID
	RequestedEvalSessionID uuid.UUID
}

func (e RunAlreadyAttachedToSessionError) Error() string {
	return fmt.Sprintf(
		"run %s is already attached to eval session %s and cannot be reattached to %s",
		e.RunID,
		e.ExistingEvalSessionID,
		e.RequestedEvalSessionID,
	)
}

func (e RunAlreadyAttachedToSessionError) Is(target error) bool {
	return target == ErrRunAlreadyAttachedToSession
}

type TemporalIDConflictError struct {
	RunID                uuid.UUID
	ExistingWorkflowID   *string
	ExistingTemporalRun  *string
	RequestedWorkflowID  string
	RequestedTemporalRun string
}

func (e TemporalIDConflictError) Error() string {
	return fmt.Sprintf(
		"run %s already has temporal ids workflow=%s run=%s; cannot replace with workflow=%q run=%q",
		e.RunID,
		quotedString(e.ExistingWorkflowID),
		quotedString(e.ExistingTemporalRun),
		e.RequestedWorkflowID,
		e.RequestedTemporalRun,
	)
}

func (e TemporalIDConflictError) Is(target error) bool {
	return target == ErrTemporalIDConflict
}

type FrozenExecutionContextError struct {
	RunAgentID uuid.UUID
	Reason     string
}

func (e FrozenExecutionContextError) Error() string {
	return fmt.Sprintf("run agent %s has an invalid frozen execution context: %s", e.RunAgentID, e.Reason)
}

func (e FrozenExecutionContextError) Is(target error) bool {
	return target == ErrFrozenExecutionContext
}

func quotedString(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", *value)
}
