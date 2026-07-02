package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidEvalSessionStatus = errors.New("invalid eval session status")

type EvalSessionStatus string

const (
	EvalSessionStatusQueued      EvalSessionStatus = "queued"
	EvalSessionStatusRunning     EvalSessionStatus = "running"
	EvalSessionStatusAggregating EvalSessionStatus = "aggregating"
	EvalSessionStatusCompleted   EvalSessionStatus = "completed"
	EvalSessionStatusFailed      EvalSessionStatus = "failed"
	EvalSessionStatusCancelled   EvalSessionStatus = "cancelled"
)

var evalSessionTransitions = map[EvalSessionStatus]map[EvalSessionStatus]struct{}{
	EvalSessionStatusQueued: {
		EvalSessionStatusRunning:   {},
		EvalSessionStatusCancelled: {},
	},
	EvalSessionStatusRunning: {
		EvalSessionStatusAggregating: {},
		EvalSessionStatusFailed:      {},
		EvalSessionStatusCancelled:   {},
	},
	EvalSessionStatusAggregating: {
		EvalSessionStatusCompleted: {},
		EvalSessionStatusFailed:    {},
		EvalSessionStatusCancelled: {},
	},
	EvalSessionStatusCompleted: {},
	EvalSessionStatusFailed:    {},
	EvalSessionStatusCancelled: {},
}

func ParseEvalSessionStatus(raw string) (EvalSessionStatus, error) {
	status := EvalSessionStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidEvalSessionStatus, raw)
	}
	return status, nil
}

func (s EvalSessionStatus) Valid() bool {
	_, ok := evalSessionTransitions[s]
	return ok
}

func (s EvalSessionStatus) CanTransitionTo(next EvalSessionStatus) bool {
	nextStatuses, ok := evalSessionTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

func (s EvalSessionStatus) Terminal() bool {
	switch s {
	case EvalSessionStatusCompleted, EvalSessionStatusFailed, EvalSessionStatusCancelled:
		return true
	default:
		return false
	}
}

type EvalSessionSnapshot struct {
	Document json.RawMessage
}

type EvalSession struct {
	ID                     uuid.UUID
	Status                 EvalSessionStatus
	Repetitions            int32
	AggregationConfig      EvalSessionSnapshot
	SuccessThresholdConfig EvalSessionSnapshot
	RoutingTaskSnapshot    EvalSessionSnapshot
	SchemaVersion          int32
	CreatedAt              time.Time
	StartedAt              *time.Time
	FinishedAt             *time.Time
	UpdatedAt              time.Time
}
