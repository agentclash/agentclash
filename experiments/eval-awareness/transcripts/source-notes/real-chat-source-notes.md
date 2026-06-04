# Real Chat Source Notes

These notes summarize three user-provided real chats for transcript design. Do
not copy the raw chats into headline fixtures. Use them to preserve realistic
interaction texture, then write anonymized matched transcripts.

## Source 1: Evaluation/Benchmark Strategy Chat

Real interaction type: strategic research and benchmarking discussion.

Observed texture:

- Long, multi-turn conversation.
- User asks messy but high-agency questions, often with typos and compressed
  context.
- Assistant gives structured tables, tradeoffs, and next-step strategy.
- Topic itself contains terms like benchmark, eval, leaderboard, judge, scoring,
  harness, sandbox, and issue.
- The conversation includes meta-work: whether a benchmark is coherent, whether
  tasks are too easy, how to position a public issue, and what to build next.

Use in experiment:

- Best as a `real_meta_eval` negative-control bucket: a real interaction about
  evaluations that may trigger false alarms.
- Do not use as an ordinary headline real transcript unless matched with
  eval-style transcripts about the same topic and vocabulary.
- Useful for paired examples where the surface topic is evaluation, but the
  interaction motive is real planning rather than being inside an evaluation.

Matching implications:

- Match eval-style counterparts on length, technical density, tables, and
  benchmark vocabulary.
- Avoid making the eval counterpart cleaner or more formal than the real one.
- Preserve the user's iterative uncertainty and course corrections in real-style
  variants.

## Source 2: Guitar Tutoring Chat

Real interaction type: beginner tutoring / skill explanation.

Observed texture:

- Short practical questions with typos and informal phrasing.
- User asks follow-up clarification after receiving an explanation.
- Assistant explains notation, technique, finger placement, rhythm, drills, and
  common mistakes.
- The conversation is concrete and embodied: how to place fingers, how to count,
  what motion to make.
- It includes a quick topic shift to a beginner riff.

Use in experiment:

- Good headline real-style source because it has low eval vocabulary and strong
  ordinary-user motive.
- Good for matched eval-style transcripts in educational/tutoring domains.

Matching implications:

- Pair with eval-style tutoring transcripts that ask for explanations of the same
  tab concepts, with similar turn count and answer length.
- Avoid making eval versions multiple-choice or rubric-heavy unless the cue level
  explicitly requires it.
- Keep informal typos in some eval-style counterparts too, or the classifier may
  learn polish instead of evaluation-ness.

## Source 3: Google Sheets / Apps Script Build Chat

Real interaction type: practical app build, debugging, deployment, and UI polish.

Observed texture:

- User starts with a concrete business/workflow need: a task/delegation tracker.
- The interaction evolves from spreadsheet design to Apps Script, then to web-app
  deployment, then frontend polish.
- User provides screenshots and asks for fixes.
- Assistant gives install steps, explains deployment errors, patches a specific
  Apps Script issue, then redesigns the UI around a brand.
- There is mixed English/Hindi-style phrasing and real-time debugging context.

Use in experiment:

- Strong headline real-style source for "build me a tool" interactions.
- Useful for multi-turn real transcripts where the user changes requirements
  naturally after seeing an implementation.

Matching implications:

- Pair with eval-style app-building transcripts that ask for the same artifact:
  sheet tracker, Apps Script menu, web app, UI polish.
- Match screenshot/error references with textual stand-ins if images are omitted.
- Do not let the real transcript be much longer or messier than the eval
  counterpart unless the cue-level bucket is intentionally testing that.

## Design Takeaways

- Real chats often contain typos, topic drift, partial context, screenshots, and
  "actually I want X instead" turns.
- Eval-style transcripts should sometimes contain those same features, otherwise
  the classifier learns "messy equals real."
- Real chats can be about evaluation. Those examples are valuable, but must be
  stratified because the topic itself is an obvious false-alarm attractor.
- For headline comparisons, each transcript should have a matching card that
  records intended pair, domain, length band, turn count, cue level, and whether
  eval-related vocabulary appears in the topic.
