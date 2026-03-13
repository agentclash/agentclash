package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrRunNotFound                  = errors.New("run not found")
	ErrRunAgentNotFound             = errors.New("run agent not found")
	ErrChallengePackVersionNotFound = errors.New("challenge pack version not found")
	ErrChallengeInputSetNotFound    = errors.New("challenge input set not found")
	ErrInvalidTransition            = errors.New("invalid status transition")
	ErrTransitionConflict           = errors.New("status transition conflict")
	ErrTemporalWorkflowID           = errors.New("temporal workflow id is required")
	ErrTemporalRunID                = errors.New("temporal run id is required")
	ErrTemporalIDConflict           = errors.New("run already has different temporal ids")
	ErrUnexpectedFailureCause       = errors.New("failure reason is only valid for failed run-agent transitions")
	ErrRunNameRequired              = errors.New("run name is required")
	ErrRunParticipantsRequired      = errors.New("run must have at least one participant")
	ErrInvalidExecutionMode         = errors.New("invalid execution mode")
	ErrRunAgentLabelRequired        = errors.New("run agent label is required")
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

func quotedString(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", *value)
}
