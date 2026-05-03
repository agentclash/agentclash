# codex/issue-489-single-challenge-input-sets - Test Contract

## Functional Behavior
- `ValidateBundle` rejects an `input_sets[].cases[]` collection that references more than one distinct `challenge_key`.
- The validation error is attached to the offending `input_sets[n].cases[m].challenge_key` field and explains that an input set must reference a single challenge.
- Multiple cases for the same challenge in one input set remain valid.
- Existing validation for missing, unknown, or duplicate case keys remains unchanged.

## Unit Tests
- `TestValidateBundleRejectsInputSetCasesAcrossMultipleChallenges` - mixed challenge keys in one input set return a validation error.
- Existing challenge pack validation tests continue to pass.

## Integration / Functional Tests
- `cd backend && go test ./internal/challengepack`.

## Smoke Tests
- `cd backend && go test ./internal/challengepack -run TestValidateBundleRejectsInputSetCasesAcrossMultipleChallenges -count=1`.

## E2E Tests
N/A - this is a publish/validate-time schema guard covered by package tests.

## Manual / cURL Tests
N/A - no API route changes; `agentclash challenge-pack validate` will surface the same `ValidateBundle` error through existing API/CLI paths.
