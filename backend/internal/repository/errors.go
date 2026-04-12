package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrRunNotFound                   = errors.New("run not found")
	ErrRunAgentNotFound              = errors.New("run agent not found")
	ErrRunAgentReplayNotFound        = errors.New("run agent replay not found")
	ErrRunAgentScorecardNotFound     = errors.New("run agent scorecard not found")
	ErrRunScorecardNotFound          = errors.New("run scorecard not found")
	ErrRunComparisonNotFound         = errors.New("run comparison not found")
	ErrEvaluationSpecNotFound        = errors.New("evaluation spec not found")
	ErrChallengePackVersionNotFound  = errors.New("challenge pack version not found")
	ErrChallengeInputSetNotFound     = errors.New("challenge input set not found")
	ErrChallengePackVersionExists    = errors.New("challenge pack version already exists")
	ErrChallengePackMetadataConflict = errors.New("challenge pack metadata conflicts with existing pack")
	ErrInvalidTransition             = errors.New("invalid status transition")
	ErrTransitionConflict            = errors.New("status transition conflict")
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
	ErrRunAgentLabelRequired         = errors.New("run agent label is required")
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
