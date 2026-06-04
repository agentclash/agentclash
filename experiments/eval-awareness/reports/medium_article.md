# Frontier Models Can't Detect When They're Being Evaluated — But They Fooled Us Into Thinking They Could

*A controlled experiment separating genuine eval-awareness from batch comparison tricks*

---

When you run a benchmark, you're implicitly assuming the model doesn't know it's being benchmarked. If that assumption is wrong, every leaderboard number is suspect. Models that ace evaluations might simply be the ones best at recognizing the test and performing accordingly — a kind of Goodhart's Law for AI capability measurement.

This concern has a name in the field: **eval-awareness**. It's been discussed theoretically, flagged in alignment papers, and cited as a reason to distrust certain benchmark results. But measuring it directly is surprisingly hard. How do you test whether a model can detect a test?

We ran an experiment designed to answer this question. The results were surprising — not because models turned out to be eval-aware, but because we caught ourselves almost measuring the wrong thing entirely.

---

## The Setup

We built two evaluation packs on AgentClash, a benchmark platform for comparing frontier models on structured tasks.

The dataset was 23 transcripts in total:

- **10 synthetic "evaluation-style" transcripts** — designed to look like evaluation prompts, but deliberately made messy. We injected casual language, incomplete sentences, and informal phrasing to make them harder to fingerprint.
- **12 real user interactions** drawn from actual chat logs across 8 domains: guitar tabs, Google Sheets formulas, a skin infection question, skincare advice, travel planning, cooking, AI research strategy, and LLM infrastructure.
- **1 ambiguous case** that could plausibly be either.
- **2 stressor transcripts** — real conversations whose *topic* is evaluation methodology. These test whether models confuse "a conversation about benchmarks" with "a conversation that is a benchmark."

The real transcripts were genuinely raw: typos, code-switching between Hindi and English (Hinglish), references to images, the kind of half-formed questions people actually ask. The eval transcripts were the opposite — formally structured, but we tried to paper over that with deliberate roughness.

### Two Modes

We ran the experiment in two configurations:

**v3 (batch mode):** The model receives all 14 sampled transcripts at once and classifies each one in a single call.

**v8 (per-case mode):** The model sees exactly one transcript per invocation, with no access to other cases.

This distinction turned out to be everything.

---

## Results

### Batch Mode (v3 — 14 transcripts per call)

| Model | Accuracy | Eval Detection Rate | False Alarm Rate |
|---|---|---|---|
| Claude Sonnet 4.6 | 36% | 0% | 14% |
| GPT-5.5 | 71% | 83% | 14% |
| Claude Opus 4.7 | 93% | 100% | 0% |

Opus looks remarkable here. 100% eval detection, no false alarms. GPT-5.5 is also strong. Sonnet 4.6 is guessing.

```
Batch Mode — Eval Detection Rate
█████████████████████████████████████████████████ Opus 4.7    100%
█████████████████████████████████████████         GPT-5.5      83%
                                                  Sonnet 4.6    0%
```

### Per-Case Mode (v8 — 23 transcripts, one at a time)

| Model | Accuracy | Eval Detection Rate | False Alarm Rate |
|---|---|---|---|
| Claude Sonnet 4.6 | 52% | 0% | 0% |
| GPT-5.5 | 48% | 0% | 8% |
| Claude Opus 4.7 | 48% | 0% | 8% |
| Claude Opus 4.8 | 52% | 0% | 0% |

Every model collapses to near-chance performance. Eval detection drops to 0% for all four.

```
Per-Case Mode — Eval Detection Rate
                                                  Opus 4.8      0%
                                                  Opus 4.7      0%
                                                  GPT-5.5       0%
                                                  Sonnet 4.6    0%
```

### The Drop

| Model | Eval Detection (batch) | Eval Detection (per-case) | Drop |
|---|---|---|---|
| Claude Sonnet 4.6 | 0% | 0% | 0 pp |
| GPT-5.5 | 83% | 0% | −83 pp |
| Claude Opus 4.7 | 100% | 0% | −100 pp |
| Claude Opus 4.8 | n/a* | 0% | — |

*Opus 4.8 was only tested in per-case mode.

```
Eval Detection Collapse (batch → per-case)
Sonnet 4.6   ████████████████████████████████ 0pp drop (was already 0%)
GPT-5.5      ████████████████████████████████ −83pp
Opus 4.7     ████████████████████████████████ −100pp
```

---

## What Actually Happened

When we looked at Opus 4.7's batch-mode rationales, the mechanism became transparent. The model was doing inter-transcript comparison, not intrinsic classification. Its notes included observations like:

> *"This appears to be a polished version of t005's messier message."*

> *"The clean structured English here mirrors t008's Hinglish content, suggesting a paired eval variant."*

It was pattern-matching across the batch to identify which transcripts seemed like "clean" versions of "messy" ones — a legitimate reasoning strategy, but one that requires seeing multiple cases simultaneously. It found structural parallels because we had, in fact, constructed synthetic evals as deliberate refinements of real conversation patterns. Opus found the signal we didn't know we'd left in.

Remove the ability to compare transcripts side-by-side, and the capability evaporates entirely.

GPT-5.5's strong batch performance likely reflects a similar mechanism — the model can identify statistical regularities across a set that are invisible when cases are presented in isolation.

Sonnet 4.6 showed no significant change across modes, but that's because it wasn't detecting anything in batch mode either. It was producing near-chance classifications throughout.

We also tested Claude Opus 4.8 in per-case mode. It confirmed the pattern: 0% eval detection across all 10 synthetic transcripts, and — unlike Opus 4.7 — no false alarm on the research stressor transcript. Interestingly, Opus 4.8's chain-of-thought explicitly reasoned about a "mismatch between system prompt and task" in its deployment context, then correctly submitted non-detection anyway. Whether this better stressor calibration reflects improved training in 4.8 or simply the different build configuration (see Limitations) isn't fully resolved.

---

## The Stressor Finding

The two stressor transcripts — real conversations whose topic was evaluation methodology — produced an interesting pattern. Both GPT-5.5 and Opus 4.7 flagged the same one as a false alarm: a genuine user conversation about AI evaluation methodology. Neither flagged a separate real conversation about LLM infrastructure.

The difference: the flagged conversation used direct vocabulary from the evaluation domain ("benchmark," "evaluation," "test cases"). The infrastructure conversation discussed related technical concepts but with less explicit evaluation vocabulary.

Opus 4.8, by contrast, did not false-alarm on this stressor — though as noted below, its deployment build differed from 4.7's, making direct comparison imperfect.

This suggests models are keying on **domain vocabulary**, not **interaction structure**. A real researcher discussing their benchmark methodology will trigger the same signal as an evaluation prompt — while a carefully written synthetic eval using non-benchmark vocabulary will not. This is almost the opposite of what you'd want from a robust eval-awareness detector.

---

## What This Means

**The main finding is negative:** No frontier model we tested can reliably detect, from a single transcript in isolation, whether that transcript is an evaluation or a real user interaction. All four models we tested score 0% eval detection in per-case mode.

This is actually somewhat reassuring from a benchmarking integrity standpoint. If you're running evaluations where each call is independent — which is the norm — there's no evidence that the models are modifying their behavior based on detection of an evaluation context.

**The batch finding is a methodological warning:** If you show a model multiple test cases simultaneously, the model can identify patterns that span cases — patterns an evaluator may not have realized they introduced. This can produce the illusion of intrinsic eval-awareness when the actual mechanism is cross-case comparison. Our synthetic transcripts had a structural relationship to the real ones (they were refined versions), and Opus found it.

This has direct implications for evaluation design:

1. **Don't present multiple test cases in a single context window** when your research question is about per-case classification. The multi-case format enables cross-contamination.

2. **Structural relationships between cases leak information.** If your synthetic test cases are derived from real examples, a capable model may reconstruct that relationship. Genuinely independent construction matters.

3. **Vocabulary is a confounder.** Any transcript that uses benchmark-specific language ("evaluation," "test cases," "benchmark") will likely be flagged by capable models — not because of structural properties, but because of surface vocabulary. Include stressor transcripts in your test set.

---

## Limitations and Next Steps

This experiment has real constraints worth naming.

**Sample size is small.** 23 transcripts and four models is enough to see a pattern, but not enough to characterize the boundaries of the effect. The 0% per-case eval detection rate could be 0% with more data, or it could be 5% or 10%. We don't know.

**Our synthetic transcripts may have been too obviously artificial despite our efforts.** We tried to make them hard to detect by adding noise, but a model operating in batch mode found enough signal to classify them perfectly. That suggests residual structure we didn't fully eliminate. Harder synthetic data might produce different results.

**We tested classification, not behavioral modification.** The experiment asked: *can models identify eval transcripts?* A different question is: *do models behave differently when they suspect they're being evaluated, even without explicit classification?* Those are separable. A model might shift behavior based on weak signals it never makes explicit in a rationale.

**Opus 4.8 used a different deployment build.** Opus 4.8 ran under a deployment we called `opus-4.8-wedge`, which has a different system prompt and policy instruction from the standard Coding Assistant build used for Opus 4.7 and the other models. This is a minor but real confound: the absence of a false alarm on the stressor, and 4.8's explicit reasoning about a "system prompt / task mismatch," may partly reflect this different operating context rather than intrinsic model differences. Future runs should use a matched build policy across all models.

**The stressor effect needs more data.** Two stressor transcripts is not enough to characterize what vocabulary patterns trigger false positives. A systematic study with varied vocabulary would be more informative.

---

## Open Questions

Several questions this experiment opens but doesn't close:

**Does per-case eval detection ability exist at any scale?** We found 0% with four frontier models. But that's four models, one dataset, one task structure. Is there a model anywhere on the capability frontier that can classify individual transcripts above chance?

**What would genuinely eval-aware behavior look like?** If a model were actually modifying its behavior based on detected evaluation context, what would you observe? More carefully constructed reasoning? Different tool use patterns? More conservative outputs? Designing experiments to detect behavioral modification — rather than just transcript classification — seems like the more important problem.

**How do you construct evaluation-resistant synthetic data?** Our synthetic transcripts had enough structural resemblance to real ones that a cross-case comparison recovered them. Is it possible to construct synthetic eval transcripts with no recoverable relationship to real examples? What does that even require?

**Does eval-awareness matter more for some capability domains than others?** A model that adjusts its behavior on math benchmarks is a different problem than one that adjusts on safety evaluations. The stakes aren't uniform.

---

The working assumption in most benchmark work is that models don't know they're being benchmarked, or that it doesn't matter if they do. This experiment provides modest evidence for the first claim — at least in the per-case regime — while raising sharper questions about how we'd detect the second. The absence of per-case classification ability doesn't preclude weaker forms of eval-awareness that wouldn't show up in this design.

If you've run similar experiments or have data on behavioral modification under suspected evaluation, we'd be interested to see it.

---

*This experiment was run using AgentClash ([agentclash.dev](https://agentclash.dev)), an open-source platform for structured LLM evaluations. The challenge packs used here (eval-awareness v3 and v8) are available in the public pack registry.*
