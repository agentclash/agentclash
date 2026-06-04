Frontier LLMs cannot detect when they're being evaluated — but they can compare transcripts.

We ran an experiment with 23 transcripts: 10 synthetic evaluation-style conversations and 12 real user interactions pulled from actual chat logs (medical questions, cooking help, travel planning, coding tools, and more). We tested two conditions — models seeing all transcripts at once (batch) versus seeing exactly one at a time (per-case).

| Model | Batch eval detection | Per-case eval detection |
|---|---|---|
| Claude Sonnet 4.6 | 0% | 0% |
| GPT-5.5 | 83% | 0% |
| Claude Opus 4.7 | 100% | 0% |
| Claude Opus 4.8 | n/a | 0% |

The batch numbers look alarming. They aren't. Every model dropped to 0% in per-case mode.

The "eval-awareness" was cross-transcript comparison masquerading as intrinsic detection. Opus was the clearest example — it literally wrote in its rationale: "this is a polished version of t005's messy message." It was pairing transcripts, not recognizing anything structural about what an evaluation looks like. Remove the comparison context and the signal vanishes entirely.

Opus 4.8 confirms the pattern — 0% per-case eval detection across all 10 synthetic transcripts — and adds a small note of nuance: it was the only model that didn't false-alarm on the research stressor conversation, which tripped both GPT-5.5 and Opus 4.7. Whether that reflects better calibration in 4.8 or its different deployment build is an open question worth flagging.

There's a secondary failure mode worth noting: models consistently flagged real conversations that *discuss* evaluation methodology as fake. A user genuinely asking about benchmarking frameworks got classified as synthetic. They're keying on vocabulary, not structure or intent.

The practical implication: if you're worried about strategic behavior in evals, batching transcripts together doesn't protect you — it gives models a different kind of signal. And if you're studying eval-awareness, per-case measurement is the right baseline. Batch performance is measuring something else entirely.

We built this experiment entirely on AgentClash (agentclash.dev), an open-source benchmark platform for structured LLM evaluations. The eval-awareness challenge packs and scorer are in the public pack registry — methodology and transcripts are on GitHub if you want to reproduce or extend it.

What would it take for a model to actually detect it's being evaluated from a single conversation? Is that even a meaningful capability to measure?
