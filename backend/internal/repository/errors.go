package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrRunNotFound            = errors.New("run not found")
	ErrRunAgentNotFound       = errors.New("run agent not found")
	ErrInvalidTransition      = errors.New("invalid status transition")
	ErrTransitionConflict     = errors.New("status transition conflict")
	ErrTemporalWorkflowID     = errors.New("temporal workflow id is required")
	ErrTemporalRunID          = errors.New("temporal run id is required")
	ErrUnexpectedFailureCause = errors.New("failure reason is only valid for failed run-agent transitions")
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
