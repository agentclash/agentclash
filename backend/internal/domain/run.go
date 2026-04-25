package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidRunStatus        = errors.New("invalid run status")
	ErrInvalidRunAgentStatus   = errors.New("invalid run agent status")
	ErrInvalidOfficialPackMode = errors.New("invalid official pack mode")
)

type RunStatus string

const (
	RunStatusDraft        RunStatus = "draft"
	RunStatusQueued       RunStatus = "queued"
	RunStatusProvisioning RunStatus = "provisioning"
	RunStatusRunning      RunStatus = "running"
	RunStatusScoring      RunStatus = "scoring"
	RunStatusCompleted    RunStatus = "completed"
	RunStatusFailed       RunStatus = "failed"
	RunStatusCancelled    RunStatus = "cancelled"
)

var runTransitions = map[RunStatus]map[RunStatus]struct{}{
	RunStatusDraft: {
		RunStatusQueued: {},
	},
	RunStatusQueued: {
		RunStatusProvisioning: {},
		RunStatusFailed:       {},
		RunStatusCancelled:    {},
	},
	RunStatusProvisioning: {
		RunStatusRunning:   {},
		RunStatusFailed:    {},
		RunStatusCancelled: {},
	},
	RunStatusRunning: {
		RunStatusScoring:   {},
		RunStatusFailed:    {},
		RunStatusCancelled: {},
	},
	RunStatusScoring: {
		RunStatusCompleted: {},
		RunStatusFailed:    {},
		RunStatusCancelled: {},
	},
	RunStatusCompleted: {},
	RunStatusFailed:    {},
	RunStatusCancelled: {},
}

func ParseRunStatus(raw string) (RunStatus, error) {
	status := RunStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRunStatus, raw)
	}
	return status, nil
}

func (s RunStatus) Valid() bool {
	_, ok := runTransitions[s]
	return ok
}

func (s RunStatus) CanTransitionTo(next RunStatus) bool {
	nextStatuses, ok := runTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

type OfficialPackMode string

const (
	OfficialPackModeFull      OfficialPackMode = "full"
	OfficialPackModeSuiteOnly OfficialPackMode = "suite_only"
)

func ParseOfficialPackMode(raw string) (OfficialPackMode, error) {
	mode := OfficialPackMode(raw)
	if !mode.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidOfficialPackMode, raw)
	}
	return mode, nil
}

func (m OfficialPackMode) Valid() bool {
	switch m {
	case OfficialPackModeFull, OfficialPackModeSuiteOnly:
		return true
	default:
		return false
	}
}

type RunAgentStatus string

const (
	RunAgentStatusQueued     RunAgentStatus = "queued"
	RunAgentStatusReady      RunAgentStatus = "ready"
	RunAgentStatusExecuting  RunAgentStatus = "executing"
	RunAgentStatusEvaluating RunAgentStatus = "evaluating"
	RunAgentStatusCompleted  RunAgentStatus = "completed"
	RunAgentStatusFailed     RunAgentStatus = "failed"
)

var runAgentTransitions = map[RunAgentStatus]map[RunAgentStatus]struct{}{
	RunAgentStatusQueued: {
		RunAgentStatusReady:  {},
		RunAgentStatusFailed: {},
	},
	RunAgentStatusReady: {
		RunAgentStatusExecuting: {},
		RunAgentStatusFailed:    {},
	},
	RunAgentStatusExecuting: {
		RunAgentStatusEvaluating: {},
		RunAgentStatusFailed:     {},
	},
	RunAgentStatusEvaluating: {
		RunAgentStatusCompleted: {},
		RunAgentStatusFailed:    {},
	},
	RunAgentStatusCompleted: {},
	RunAgentStatusFailed:    {},
}

func ParseRunAgentStatus(raw string) (RunAgentStatus, error) {
	status := RunAgentStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRunAgentStatus, raw)
	}
	return status, nil
}

func (s RunAgentStatus) Valid() bool {
	_, ok := runAgentTransitions[s]
	return ok
}

func (s RunAgentStatus) CanTransitionTo(next RunAgentStatus) bool {
	nextStatuses, ok := runAgentTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

type Run struct {
	ID                     uuid.UUID
	OrganizationID         uuid.UUID
	WorkspaceID            uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeInputSetID    *uuid.UUID
	EvalSessionID          *uuid.UUID
	OfficialPackMode       OfficialPackMode
	CreatedByUserID        *uuid.UUID
	Name                   string
	Status                 RunStatus
	ExecutionMode          string
	TemporalWorkflowID     *string
	TemporalRunID          *string
	ExecutionPlan          json.RawMessage
	QueuedAt               *time.Time
	StartedAt              *time.Time
	FinishedAt             *time.Time
	CancelledAt            *time.Time
	FailedAt               *time.Time
	// RaceContext opts the run into live peer-standings injection during
	// execution. Nil or false means the run behaves identically to pre-#400
	// main: no standings are injected, no race.standings.injected events
	// are emitted, and billable-token accounting is unchanged.
	RaceContext bool
	// RaceContextMinStepGap overrides the default minimum steps between
	// standings injections. Nil = backend default. Valid range [1, 10].
	RaceContextMinStepGap *int32
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type RunAgent struct {
	ID                        uuid.UUID
	OrganizationID            uuid.UUID
	WorkspaceID               uuid.UUID
	RunID                     uuid.UUID
	AgentDeploymentID         uuid.UUID
	AgentDeploymentSnapshotID uuid.UUID
	LaneIndex                 int32
	Label                     string
	Status                    RunAgentStatus
	QueuedAt                  *time.Time
	StartedAt                 *time.Time
	FinishedAt                *time.Time
	FailureReason             *string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type RunStatusHistory struct {
	ID              uuid.UUID
	RunID           uuid.UUID
	FromStatus      *RunStatus
	ToStatus        RunStatus
	Reason          *string
	ChangedByUserID *uuid.UUID
	ChangedAt       time.Time
}

type RunAgentStatusHistory struct {
	ID         uuid.UUID
	RunAgentID uuid.UUID
	FromStatus *RunAgentStatus
	ToStatus   RunAgentStatus
	Reason     *string
	ChangedAt  time.Time
}
