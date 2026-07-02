package scoring

// SourceKind discriminates what kind of trace element a validator or metric
// evaluated. The frontend inspector uses this to decide how to deep-link back
// into the replay.
type SourceKind string

const (
	// SourceKindRunEvent is a generic pointer into the run_events stream.
	SourceKindRunEvent SourceKind = "run_event"
	// SourceKindToolCall points at a tool_call / grader verification event that
	// produced a file capture, code execution, or similar artifact.
	SourceKindToolCall SourceKind = "tool_call"
	// SourceKindModelCall points at a model.call.completed event — the usual
	// producer of an agent's final text when no dedicated
	// system.output.finalized event was emitted.
	SourceKindModelCall SourceKind = "model_call"
	// SourceKindFinalOutput points at the dedicated finalized-output event
	// (system.output.finalized). Narrower than the run.completed wrapper.
	SourceKindFinalOutput SourceKind = "final_output"
)

// Source is an optional pointer on ValidatorResult/MetricResult identifying
// the originating run event the result was evaluated against. It is nil when
// the evidence is aggregated across multiple events (token totals, latency
// spans) or sourced from challenge inputs that do not live in the trace.
//
// Sequence is the authoritative pointer — run_events are uniquely addressed
// by (run_agent_id, sequence_number). EventType is a denormalized copy that
// lets the UI display a label without re-fetching the event.
type Source struct {
	Kind      SourceKind `json:"kind"`
	Sequence  *int64     `json:"sequence,omitempty"`
	EventType string     `json:"event_type,omitempty"`
	FieldPath string     `json:"field_path,omitempty"`
}
