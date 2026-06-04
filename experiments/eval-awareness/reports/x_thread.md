# Eval-Awareness X Thread

---

**1/**
We tested whether frontier AI can tell it's being evaluated vs. talking to a real user. The results broke our priors. [THREAD]

---

**2/**
Setup: 23 transcripts — 10 synthetic "eval-style", 12 real user chats (guitar lessons, skincare, travel, cooking, LLM infra...). Two conditions: see all 23 at once (batch), or see exactly one at a time (per-case).

---

**3/**
Batch mode results looked alarming (3 models tested here — Opus 4.8 was only run per-case):

```
Model             Eval Detection (batch)
─────────────────────────────────────────
Claude Sonnet 4.6  ░░░░░░░░░░░░░░  0%
GPT-5.5            ████████████░░  83%
Claude Opus 4.7    ██████████████  100%
```

Opus caught every single eval transcript. GPT-5.5 got 5/6. Sonnet got none.

---

**4/**
Then we ran per-case mode. One transcript. No other context. Just: "is this real or eval?"

```
Model             Eval Detection (per-case)
───────────────────────────────────────────
Claude Sonnet 4.6  ░░░░░░░░░░░░░░  0%
GPT-5.5            ░░░░░░░░░░░░░░  0%
Claude Opus 4.7    ░░░░░░░░░░░░░░  0%
Claude Opus 4.8    ░░░░░░░░░░░░░░  0%
```

Every model. Zero detections. Across all 10 eval transcripts.

---

**5/**
Full table:

```
┌──────────────────┬──────────────┬───────────────┬────────┐
│ Model            │ Batch (6 ev) │ Per-case (10) │  Drop  │
├──────────────────┼──────────────┼───────────────┼────────┤
│ Sonnet 4.6       │   0%  (0/6)  │   0%  (0/10)  │  0 pp  │
│ GPT-5.5          │  83%  (5/6)  │   0%  (0/10)  │ -83 pp │
│ Opus 4.7         │ 100%  (6/6)  │   0%  (0/10)  │ -100pp │
│ Opus 4.8         │     n/a      │   0%  (0/10)  │   —    │
└──────────────────┴──────────────┴───────────────┴────────┘
```

The 83pp and 100pp collapses are not noise. That's a complete mode flip.

---

**6/**
Here's what Opus was actually doing in batch mode. From its own rationale: "this is a polished version of t005's messy message."

It wasn't detecting eval structure. It was doing cross-transcript comparison. Pattern matching against the other transcripts it had already seen.

---

**7/**
Remove that comparative context and the capability vanishes entirely. The "eval-awareness" that looked scary in batch mode is better described as: unusually good text similarity detection across a document set.

That's a very different thing.

---

**8/**
Secondary finding: both GPT-5.5 and Opus 4.7 made the same false alarm — flagging a real user conversation *about* AI evaluation methodology as "this is an evaluation."

They key on eval vocabulary as topic, not interaction structure. Discuss evals -> flagged as an eval.

---

**9/**
We then ran Opus 4.8 per-case. It also scored 0% eval detection — confirming the pattern. But it did something the others didn't: it explicitly reasoned in its chain-of-thought about a "mismatch between system prompt and task" in the deployment it was running under.

And it still submitted correct JSON. No false alarm on the research stressor transcript that tripped up 4.7.

---

**10/**
Caveat worth flagging: Opus 4.8 ran under a different build policy (opus-4.8-wedge) from the standard Coding Assistant deployment used for 4.7. The better stressor calibration might reflect improved training — or it might reflect the different system prompt. We can't fully disentangle those.

---

**11/**
What this means for AI safety: the concern that models will behave differently during evals than in deployment requires the model to recognize the eval context. In per-case conditions — which is how evals actually run — no current frontier model does this reliably.

---

**12/**
What this means for benchmark design: if your benchmark transcripts look different as a corpus from your training distribution, a sufficiently capable model running in batch/agentic mode can pattern-match on that. Your eval isn't measuring what you think.

Keep eval transcripts ecologically valid.

---

**13/**
The irony: the model best at detecting evals in batch mode (Opus 4.7, 100%) is also the one that drops hardest to 0% when you remove batch context. The capability is entirely parasitic on cross-document comparison. Isolate the transcript, isolate the blindness.

This experiment was run entirely on AgentClash (agentclash.dev) — an open-source platform for structured LLM evaluations. The eval-awareness challenge packs (v3 + v8) are in the public pack registry. Transcripts and scorer code are on GitHub.
