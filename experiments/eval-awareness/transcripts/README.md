# Transcript Fixtures

This directory holds transcript-design materials for the evaluation-awareness
experiment.

Current files:

- `source-notes/real-chat-source-notes.md`: anonymized notes from user-provided
  real chats.
- `raw-user-provided/`: raw chat logs explicitly provided as source material for
  auditability.
- `drafts/real_seed_catalog.jsonl`: metadata catalog for the three real-chat
  seeds.
- `drafts/matched_pair_plan.md`: pilot pairing plan.
- `drafts/pilot_transcripts.jsonl`: first synthetic pilot set.

## Pilot Status

`drafts/pilot_transcripts.jsonl` contains 9 matched pairs, 18 transcripts total:

- 9 `real_user_interaction`
- 9 `evaluation`
- 3 domains: music tutoring, business tool building, evaluation research strategy

The real-style transcripts are synthetic/anonymized from user-provided real-chat
seeds. The raw source logs are also committed under `raw-user-provided/` for
auditability. The eval-style transcripts are synthetic counterparts matched on
topic, turn count, and interaction shape.

The local JSONL fixture intentionally includes hidden labels, internal IDs,
domains, cue levels, and source notes for offline scoring. The generated
AgentClash pack must not expose those fields to the model. The current pack
builder emits only neutral public IDs (`t001` through `t018`) and transcript
text.

Do not treat this pilot as the headline dataset yet. It is for smoke-testing the
classifier prompt, parser, and scoring workflow, and for checking whether the
matching strategy creates obvious artifacts.

## Important Caveat

The meta-evaluation family intentionally discusses benchmarks and evals while
still containing real-style examples. Report that family separately in early
pilots because it is designed to measure false alarms caused by topic vocabulary.
