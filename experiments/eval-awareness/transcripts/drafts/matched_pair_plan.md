# Matched Pair Plan

Start with a pilot set derived from the three real-chat sources. The goal is not
to preserve raw chat text; it is to preserve interaction shape while controlling
surface confounds.

## Pilot Pair Families

### Family A: Music Tutoring

Real source: `real_seed_002`.

Candidate pairs:

- `real_music_tutoring_001` vs `eval_music_tutoring_moderate_001`
- `real_music_tutoring_002` vs `eval_music_tutoring_subtle_001`
- `real_music_tutoring_003` vs `eval_music_tutoring_no_cue_001`

Controls:

- Same approximate number of turns.
- Same topic: guitar tab notation and beginner technique.
- Similar typos/informality in both classes.
- Similar answer length and explanation style.

Eval cues to vary:

- Moderate: task asks for a structured explanation with explicit constraints.
- Subtle: unusually complete user request and tidy follow-up.
- No cue: ordinary tutoring interaction, class identity should be hard.

### Family B: Task Tracker / Apps Script

Real source: `real_seed_003`.

Candidate pairs:

- `real_tool_building_001` vs `eval_tool_building_moderate_001`
- `real_tool_building_002` vs `eval_tool_building_subtle_001`
- `real_tool_building_003` vs `eval_tool_building_no_cue_001`

Controls:

- Same artifact: Google Sheets task/delegation tracker.
- Same escalation path: template, Apps Script, deployment/debugging, UI polish.
- Similar screenshot/error placeholders in both classes.
- Similar mixed precision: concrete enough to act, not perfectly specified.

Eval cues to vary:

- Moderate: unnaturally explicit deliverable constraints and acceptance criteria.
- Subtle: tidy screenshots/error descriptions and compact requirements.
- No cue: normal iterative build/debug request.

### Family C: Evaluation Research Strategy

Real source: `real_seed_001`.

Candidate pairs:

- `real_meta_eval_001` vs `eval_meta_eval_moderate_001`
- `real_meta_eval_002` vs `eval_meta_eval_subtle_001`
- `real_meta_eval_003` vs `eval_meta_eval_no_cue_001`

Controls:

- Both classes discuss benchmark design, methodology, or research positioning.
- Both include eval-related vocabulary because the topic requires it.
- Both include tradeoff tables or bullet-heavy strategy.

Use:

- Treat as a stratified false-alarm stressor. Report separately if the sample is
  small.
- This family tests whether models equate "talking about evals" with "this
  transcript is itself an eval."

## Next Authoring Step

Write synthetic, anonymized transcripts from these families with per-case
metadata:

```json
{
  "id": "real_music_tutoring_001",
  "label": "real_user_interaction",
  "pair_id": "music_tutoring_001",
  "cue_level": "matched_real",
  "domain": "music_tutoring",
  "topic_has_eval_vocabulary": false,
  "turn_count": 4,
  "word_count": 650,
  "source_seed": "real_seed_002",
  "transcript": "User: ...\nAssistant: ..."
}
```

Do not include raw attachment text verbatim unless the user explicitly approves
that use.
