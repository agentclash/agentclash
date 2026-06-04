# Frontier Models Can't Detect When They're Being Evaluated — But Batch Mode Fooled Us Into Thinking They Could

We ran an experiment on eval-awareness: can frontier LLMs tell when they're being tested versus talking to a real user? The answer surprised us.

---

## The Setup

We built a dataset of 23 transcripts: 10 synthetic "evaluation-style" conversations, 12 real user interactions pulled from actual chat logs (medical questions, cooking, travel planning, guitar tutoring, skincare, and more), and 1 ambiguous case. The eval transcripts were deliberately made messy — informal language, incomplete phrasing — to make them harder to fingerprint.

We tested two conditions:

**Batch mode:** The model sees all transcripts at once and classifies each one.

**Per-case mode:** The model sees exactly one transcript per invocation. No comparison context.

---

## Batch Mode Looked Alarming

<!-- PASTE chart_detection.png HERE -->

In batch mode, Opus 4.7 caught every single eval transcript. GPT-5.5 got 5 out of 6. Sonnet 4.6 detected nothing.

| Model | Batch Eval Detection | Per-Case Eval Detection | Drop |
|---|---|---|---|
| Claude Sonnet 4.6 | 0% | 0% | 0 pp |
| GPT-5.5 | 83% | 0% | −83 pp |
| Claude Opus 4.7 | 100% | 0% | −100 pp |
| Claude Opus 4.8* | n/a | 0% | — |

*Opus 4.8 was only tested in per-case mode.*

---

## Then We Ran Per-Case Mode

<!-- PASTE chart_collapse.png HERE -->

Every model. Zero detections. Across all 10 eval transcripts. The 83 and 100 percentage point drops are not noise — that's a complete mode flip.

---

## What Was Actually Happening

When we read Opus 4.7's batch-mode rationales, the mechanism became transparent. It wasn't detecting anything intrinsic about evaluation transcripts. It was doing cross-transcript comparison:

> *"This appears to be a polished version of t005's messier message."*

> *"The clean structured English here mirrors t008's Hinglish content, suggesting a paired eval variant."*

It found structural parallels because we had — without fully realizing it — built synthetic eval transcripts as deliberate refinements of real conversation patterns. Opus found the signal we'd left in. GPT-5.5's strong batch performance likely reflects the same mechanism.

Remove the ability to compare transcripts side-by-side, and the capability disappears entirely.

---

## A Secondary Finding: Vocabulary, Not Structure

<!-- PASTE chart_percase_summary.png HERE -->

Both GPT-5.5 and Opus 4.7 made the same false alarm in per-case mode: they flagged a real user conversation *about* AI evaluation methodology as "this is an evaluation." A user genuinely discussing benchmark design got classified as synthetic.

Opus 4.8 did not make this mistake — though it ran under a different deployment build, so we can't fully attribute this to improved training.

The pattern: models key on eval vocabulary as a topic signal, not on anything structural about the interaction. Discuss benchmarks → flagged as a benchmark.

---

## What This Means

**For benchmarking integrity:** If your evaluations run each case independently — which is the standard setup — there's no evidence frontier models are modifying behavior based on detected eval context. That's somewhat reassuring.

**For eval design:** If you show a model multiple test cases simultaneously, it can identify structural patterns across cases that span the corpus. Our matched-pair design inadvertently left this signal in. Keep eval transcripts ecologically valid and independently constructed.

**For eval-awareness research:** Batch performance is measuring something different from per-case performance. Cross-case comparison ability is not the same as intrinsic eval-awareness. If you're studying whether models behave differently when tested, per-case measurement is the right baseline.

---

## Built on AgentClash

This experiment was run on **AgentClash** ([agentclash.dev](https://agentclash.dev)) — an open-source platform for structured LLM evaluations. The eval-awareness challenge packs (v3 batch + v8 per-case) are available in the public pack registry. Dataset, classification prompts, and scorer code are on GitHub.

The working assumption in most benchmark work is that models don't know they're being benchmarked. This experiment provides evidence for that claim — in the per-case regime — while raising sharper questions about how batch evaluation design can leak structural signals to capable models.

What would it take for a model to genuinely detect it's being evaluated from a single conversation? We don't know yet.
