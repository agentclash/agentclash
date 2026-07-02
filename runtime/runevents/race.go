package runevents

// RaceStandingsTrigger identifies why a standings injection fired. Emitters in
// the native executor pick one value per event; post-hoc analyses use this
// field to distinguish "scheduled cadence" injections from "peer state change"
// injections, which behave differently in research settings.
type RaceStandingsTrigger string

const (
	RaceStandingsTriggerCadence       RaceStandingsTrigger = "cadence"
	RaceStandingsTriggerPeerSubmitted RaceStandingsTrigger = "peer_submitted"
	RaceStandingsTriggerPeerFailed    RaceStandingsTrigger = "peer_failed"
	RaceStandingsTriggerPeerTimedOut  RaceStandingsTrigger = "peer_timed_out"
)

// IsValid reports whether the trigger is one of the declared values.
func (t RaceStandingsTrigger) IsValid() bool {
	switch t {
	case RaceStandingsTriggerCadence,
		RaceStandingsTriggerPeerSubmitted,
		RaceStandingsTriggerPeerFailed,
		RaceStandingsTriggerPeerTimedOut:
		return true
	default:
		return false
	}
}

// RaceStandingsInjectedPayload is the canonical shape emitted with a
// `race.standings.injected` event. The executor writes this to
// Envelope.Payload (JSON) each time it appends a standings newswire message
// to an agent's context.
//
// TokensAdded is prompt-side only (injection is a user message).
// StandingsSnapshot is the verbatim newswire string passed to the model — it
// must stay in the event so replays and research reproduce exactly what the
// agent saw.
type RaceStandingsInjectedPayload struct {
	TokensAdded       int                  `json:"tokens_added"`
	StandingsSnapshot string               `json:"standings_snapshot"`
	TriggeredBy       RaceStandingsTrigger `json:"triggered_by"`
	// SelfStepIndex is the agent's step number at the moment of injection
	// (the same step at whose start the injection was evaluated).
	SelfStepIndex int `json:"self_step_index"`
	// MinStepGap is the cadence threshold active for this run at the moment
	// of injection. Preserved on every event so a replayer can explain the
	// cadence without loading the run row.
	MinStepGap int `json:"min_step_gap"`
}
