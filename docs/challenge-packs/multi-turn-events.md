# Multi-turn run events and transcript

Challenge-pack multi-turn evals emit sequence-numbered conversation events alongside the existing native multi-step tool loop. Scoring, replay, and human workflows reconstruct turns from these events.

## Event types

| Type | Purpose |
|------|---------|
| `turn.user.message` | Scripted, LLM, or human user message for a turn |
| `turn.user.simulated` | LLM simulator metadata (model, token usage) |
| `turn.assistant.message` | Final assistant text summary for the turn |
| `turn.completed` | Turn boundary; includes `mismatch` when expects fail |
| `turn.awaiting_human` | H3 blocked state waiting for operator input |
| `turn.state.captured` | Optional snapshot reference |
| `conversation.completed` | Outer loop terminal marker |

## Summary metadata

Multi-turn events reuse envelope `summary` fields:

- `turn_index` — zero-based outer turn counter
- `phase_id` — active `user_simulator` phase
- `actor` — `scripted`, `llm`, or `human`
- `mismatch` — explicit bool on `turn.completed` for native multi-turn runs

Voice adapter `turn.completed` events remain valid without `phase_id` / `actor`.

## Transcript helper

`runevents.TranscriptFromEvents([]Envelope)` sorts by `sequence_number` and groups user + assistant content per `turn_index`. Emitters land in sub-issue #843; this issue defines types, validation, and reconstruction only.

## Validation

- `Envelope.ValidatePending()` — generic envelope checks (all event types)
- `ValidateMultiTurnEvent()` — stricter checks for native multi-turn emitters
