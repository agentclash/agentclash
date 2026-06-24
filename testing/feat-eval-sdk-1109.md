# feat/eval-sdk-1109 — Test Contract

## Functional Behavior

- Judge provider interface with FakeJudgeProvider for tests.
- Parser handles JSON, markdown fences, repairable truncation, unrepairable errors.
- Judge metrics: TaskCompletion, ToolArgumentCorrectness, RetrievalGrounding, SafetyPolicy, StepEfficiency.
- No default hidden model calls; provider must be explicit.

## Unit Tests

- `tests/test_judge.py` — parser and fake provider metric paths.

## Smoke Tests

- `PYTHONPATH=src python3 -m pytest tests/test_judge.py -q`

## E2E Tests

- N/A

## Manual Tests

- N/A
