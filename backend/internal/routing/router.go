package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrNoModelsAvailable     = errors.New("no models available for routing")
	ErrUnsupportedPolicyKind = errors.New("routing policy kind is not yet supported")
	ErrAllModelsFailed       = errors.New("all models in fallback chain failed")
)

// Policy represents a routing policy loaded from the database.
type Policy struct {
	ID         uuid.UUID
	PolicyKind string          // "single_model", "fallback", "budget_aware", "latency_aware"
	Config     json.RawMessage
}

// ModelTarget represents a selectable provider+model combination.
type ModelTarget struct {
	ProviderKey         string
	ProviderAccountID   string
	CredentialReference string
	Model               string
}

// Selector picks a model target from the available targets based on the policy.
type Selector interface {
	Select(ctx context.Context, policy Policy, available []ModelTarget) (ModelTarget, error)
}

// NewSelector returns the appropriate Selector for the given policy kind.
func NewSelector(policyKind string) (Selector, error) {
	switch policyKind {
	case "single_model":
		return SingleModelSelector{}, nil
	case "fallback":
		return FallbackSelector{}, nil
	case "budget_aware", "latency_aware":
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedPolicyKind, policyKind)
	default:
		return nil, fmt.Errorf("unknown routing policy kind: %s", policyKind)
	}
}
