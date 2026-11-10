package repository

import "fmt"

// ConnectorDispatchState is the state of one connector_dispatches row. One
// row records one http_json call for one canonical case.
type ConnectorDispatchState string

const (
	// ConnectorDispatchStateSending is the first state. ClaimConnectorDispatch
	// writes it before the network call.
	ConnectorDispatchStateSending ConnectorDispatchState = "sending"
	// ConnectorDispatchStateCompleted means that the remote returned a
	// valid response, and the output and the evidence are stored.
	ConnectorDispatchStateCompleted ConnectorDispatchState = "completed"
	// ConnectorDispatchStateRejected means that the remote did not run the
	// case: the request was not sent, or the remote refused it. The
	// response column holds only the reason and the HTTP status.
	ConnectorDispatchStateRejected ConnectorDispatchState = "rejected"
	// ConnectorDispatchStateUnknown means that AgentClash cannot tell what
	// the remote did. The call is never sent again.
	ConnectorDispatchStateUnknown ConnectorDispatchState = "unknown"
	// The DB CHECK also permits "accepted". No writer sets it, and it is not
	// in the transition table: a synchronous call has no accept phase.
)

// connectorDispatchTransitions names each state change that a writer can
// make.
//
// Writers:
//   - CommitConnectorResponse: sending -> completed.
//   - RejectConnectorDispatch: sending -> rejected.
//   - UnknownConnectorDispatch: sending -> unknown.
var connectorDispatchTransitions = map[ConnectorDispatchState]map[ConnectorDispatchState]struct{}{
	ConnectorDispatchStateSending: {
		ConnectorDispatchStateCompleted: {},
		ConnectorDispatchStateRejected:  {},
		ConnectorDispatchStateUnknown:   {},
	},
	ConnectorDispatchStateCompleted: {},
	ConnectorDispatchStateRejected:  {},
	ConnectorDispatchStateUnknown:   {},
}

func ParseConnectorDispatchState(raw string) (ConnectorDispatchState, error) {
	state := ConnectorDispatchState(raw)
	if !state.Valid() {
		return "", fmt.Errorf("invalid connector dispatch state: %q", raw)
	}
	return state, nil
}

func (s ConnectorDispatchState) Valid() bool {
	_, ok := connectorDispatchTransitions[s]
	return ok
}

func (s ConnectorDispatchState) CanTransitionTo(next ConnectorDispatchState) bool {
	nextStates, ok := connectorDispatchTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStates[next]
	return ok
}

// Terminal reports whether the state is final.
func (s ConnectorDispatchState) Terminal() bool {
	nextStates, ok := connectorDispatchTransitions[s]
	return ok && len(nextStates) == 0
}
