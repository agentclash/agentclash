package routing

import "context"

// SingleModelSelector returns the first available model target.
type SingleModelSelector struct{}

// Select returns the first model target from the available list.
func (SingleModelSelector) Select(_ context.Context, _ Policy, available []ModelTarget) (ModelTarget, error) {
	if len(available) == 0 {
		return ModelTarget{}, ErrNoModelsAvailable
	}
	return available[0], nil
}
