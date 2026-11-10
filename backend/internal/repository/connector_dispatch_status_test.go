package repository

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/connectors"
)

// The CHECK constraint from migration 00090 also permits "accepted". No
// writer sets it, so it is not a valid Go state.
var allConnectorDispatchStates = []ConnectorDispatchState{
	ConnectorDispatchStateSending,
	ConnectorDispatchStateCompleted,
	ConnectorDispatchStateRejected,
	ConnectorDispatchStateUnknown,
}

func TestConnectorDispatchTransitionsNameEveryLegalMove(t *testing.T) {
	type move struct{ from, to ConnectorDispatchState }
	legal := map[move]string{
		{ConnectorDispatchStateSending, ConnectorDispatchStateCompleted}: "CommitConnectorResponse",
		{ConnectorDispatchStateSending, ConnectorDispatchStateRejected}:  "RejectConnectorDispatch",
		{ConnectorDispatchStateSending, ConnectorDispatchStateUnknown}:   "UnknownConnectorDispatch",
	}
	for _, from := range allConnectorDispatchStates {
		if !from.Valid() {
			t.Fatalf("state %q is not in the transition table", from)
		}
		for _, to := range allConnectorDispatchStates {
			_, want := legal[move{from, to}]
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v", from, to, got, want)
			}
		}
		if got, want := from.Terminal(), from != ConnectorDispatchStateSending; got != want {
			t.Errorf("%s.Terminal() = %v, want %v", from, got, want)
		}
	}
	if len(connectorDispatchTransitions) != len(allConnectorDispatchStates) {
		t.Fatalf("transition table has %d states, want %d", len(connectorDispatchTransitions), len(allConnectorDispatchStates))
	}
	if _, err := ParseConnectorDispatchState("accepted"); err == nil {
		t.Fatal("accepted has no writer and must not parse")
	}
}

func TestConnectorRejectionKeepsOnlyReasonAndStatus(t *testing.T) {
	refused, ok := ConnectorRejectionFor(&connectors.RefusedError{Status: 401})
	if !ok {
		t.Fatal("refusal not classified")
	}
	raw, err := json.Marshal(refused.Rejection)
	if err != nil || string(raw) != `{"reason":"refused","status":401}` {
		t.Fatalf("refused detail=%s err=%v", raw, err)
	}
	if !errors.Is(refused, ErrConnectorRejected) || !errors.Is(refused, connectors.ErrRefused) || !strings.Contains(refused.Error(), "401") {
		t.Fatalf("refused error=%v", refused)
	}

	notSent, ok := ConnectorRejectionFor(&connectors.NotSentError{Err: ErrConnectorUnapproved})
	if !ok {
		t.Fatal("unsent request not classified")
	}
	raw, err = json.Marshal(notSent.Rejection)
	if err != nil || string(raw) != `{"reason":"not_sent"}` {
		t.Fatalf("not sent detail=%s err=%v", raw, err)
	}
	if !errors.Is(notSent, ErrConnectorRejected) || !errors.Is(notSent, ErrConnectorUnapproved) || !strings.Contains(notSent.Error(), ErrConnectorUnapproved.Error()) {
		t.Fatalf("not sent error=%v", notSent)
	}

	if _, ok := ConnectorRejectionFor(errors.New("connector returned a non-success response; execution outcome is unknown")); ok {
		t.Fatal("unknown outcome classified as a rejection")
	}

	// A stored detail gives the same text as the first call.
	stored := &ConnectorRejectedError{Rejection: refused.Rejection}
	if stored.Error() != refused.Error() || !errors.Is(stored, ErrConnectorRejected) {
		t.Fatalf("stored=%q first=%q", stored.Error(), refused.Error())
	}
}
