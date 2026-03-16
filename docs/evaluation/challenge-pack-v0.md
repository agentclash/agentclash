# Challenge Pack v0

This document defines the v0 evaluation contract for Step 6a.10.

## Purpose

The v0 contract gives each runnable challenge-pack version a versioned `evaluation_spec`
that later scoring code can load, persist, and reference from scorecards.

This step does not execute validators.
It defines the schema, loading path, validation rules, and versioning rules.

## Authoring format

For v0, the source of truth lives in `challenge_pack_versions.manifest`.

The manifest may contain a top-level `evaluation_spec` block like this:

```json
{
  "schema_version": 1,
  "tool_policy": {
    "allowed_tool_kinds": ["file", "shell"]
  },
  "evaluation_spec": {
    "name": "coding-fix-v0",
    "version_number": 1,
    "judge_mode": "deterministic",
    "validators": [
      {
        "key": "exact_output_match",
        "type": "exact_match",
        "target": "final_output",
        "expected_from": "challenge_input"
      }
    ],
    "metrics": [
      {
        "key": "total_latency_ms",
        "type": "numeric",
        "collector": "run_total_latency_ms",
        "unit": "ms"
      }
    ],
    "scorecard": {
      "dimensions": ["correctness", "latency"]
    }
  }
}
```

## Field summary

- `evaluation_spec.name`: stable logical name for the scoring contract
- `evaluation_spec.version_number`: positive integer version
- `evaluation_spec.judge_mode`: one of `deterministic`, `llm_judge`, `hybrid`
- `validators`: declared checks that later validator execution can implement
- `metrics`: declared collectors that later metric execution can implement
- `scorecard.dimensions`: score groups later surfaced to users

## Loader behavior

The loader:

1. extracts `evaluation_spec` from the manifest
2. unmarshals it into typed Go structs
3. validates required fields and enum values
4. rejects duplicate validator or metric keys
5. normalizes the spec before persistence

Invalid specs fail early with field-specific errors.

## Versioning and immutability

- A runnable `ChallengePackVersion` remains immutable.
- `evaluation_spec.version_number` must be greater than zero.
- Changing validator behavior, metric declarations, or scorecard dimensions requires a new `version_number`.
- Scorecards must persist the exact `evaluation_spec_id` they used.

This preserves comparability while still allowing the benchmark to evolve through new versions.

## Deferred to later issues

- `#46`: validator execution and metric collectors
- `#47`: scorecard generation
- `#48`-`#50`: comparison and release gates
- `#82`: failure-to-eval flywheel
