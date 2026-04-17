# codex/issue-268-text-gen-metrics — Test Contract

## Functional Behavior
- Add three new scoring validators: `bleu_score`, `rouge_score`, and `chrf_score`.
- Each validator returns a graduated normalized score in the 0..1 range and a pass/fail verdict based on a configurable threshold.
- `bleu_score` computes clipped n-gram precision with brevity penalty and supports one or more reference texts.
- `rouge_score` supports `rouge-1`, `rouge-2`, and `rouge-l`, with scores computed from token overlap or longest-common-subsequence behavior as configured.
- `chrf_score` computes a character n-gram F-score and works on Unicode text without corrupting multi-byte characters.
- Exact matches produce scores at or near 1.0 for all three validators.
- Predictions with no meaningful overlap produce scores near 0.0.
- Short predictions are penalized by BLEU brevity handling instead of receiving an inflated precision-only score.
- Unknown config fields are rejected at spec-load time, matching the existing strict validator-config behavior.
- The new validator types are accepted by spec loading and validation, and continue using the existing evidence resolution flow (`target` plus `expected_from`).

## Unit Tests
- `TestValidateBLEUScore` covers exact match, no overlap, brevity penalty, threshold handling, and multi-reference behavior.
- `TestValidateROUGEScore` covers `rouge-1`, `rouge-2`, and `rouge-l` exact match and low-overlap cases.
- `TestValidateChrFScore` covers exact match, low-overlap behavior, and Unicode text.
- `TestParseBLEUScoreConfig` rejects invalid JSON, invalid smoothing values, invalid `max_ngram`, and mixed reference config shapes if unsupported.
- `TestParseROUGEScoreConfig` rejects invalid JSON, unsupported Rouge variants, and invalid threshold/beta settings.
- `TestParseChrFScoreConfig` rejects invalid JSON and invalid character n-gram / beta settings.
- `TestLoadEvaluationSpecAcceptsGenerationMetricValidators` ensures the three new validator types are accepted in manifests.
- `TestValidateEvaluationSpecRejectsInvalidGenerationMetricConfig` ensures config validation reports useful errors.

## Integration / Functional Tests
- `TestEvaluateValidators`-style engine coverage verifies the new validator types flow through `applyValidator` and surface normalized scores in validator results.
- A spec-loading test confirms deterministic evaluation specs can declare the new validators without breaking scorecard normalization or validator targeting.

## Smoke Tests
- `go test ./internal/scoring`
- `go test ./internal/challengepack ./internal/engine`
- Confirm no unrelated validator regressions in the scoring package test suite.

## E2E Tests
- N/A — this issue is backend scoring infrastructure and does not introduce a browser or end-user flow.

## Manual / cURL Tests
- N/A — verification is through Go unit/spec tests rather than a dedicated HTTP endpoint for these validators.
