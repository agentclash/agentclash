package runevents

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRaceStandingsInjectedEventTypeIsValid(t *testing.T) {
	if !isValidType(EventTypeRaceStandingsInjected) {
		t.Fatalf("EventTypeRaceStandingsInjected must be accepted by isValidType")
	}
}

func TestRaceStandingsTriggerIsValid(t *testing.T) {
	for _, trigger := range []RaceStandingsTrigger{
		RaceStandingsTriggerCadence,
		RaceStandingsTriggerPeerSubmitted,
		RaceStandingsTriggerPeerFailed,
		RaceStandingsTriggerPeerTimedOut,
	} {
		if !trigger.IsValid() {
			t.Fatalf("trigger %q must be valid", trigger)
		}
	}

	if RaceStandingsTrigger("bogus").IsValid() {
		t.Fatalf("unknown trigger must not validate")
	}
}

func TestRaceStandingsInjectedEnvelopeValidates(t *testing.T) {
	payload, err := json.Marshal(RaceStandingsInjectedPayload{
		TokensAdded:       42,
		StandingsSnapshot: "[RACE UPDATE] 3 agents running, 0 submitted.",
		TriggeredBy:       RaceStandingsTriggerCadence,
		SelfStepIndex:     5,
		MinStepGap:        3,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	env := Envelope{
		EventID:       "evt-test",
		SchemaVersion: SchemaVersionV1,
		RunID:         uuid.New(),
		RunAgentID:    uuid.New(),
		EventType:     EventTypeRaceStandingsInjected,
		Source:        SourceNativeEngine,
		OccurredAt:    time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC),
		Payload:       payload,
	}
	if err := env.ValidatePending(); err != nil {
		t.Fatalf("envelope must validate: %v", err)
	}
}
