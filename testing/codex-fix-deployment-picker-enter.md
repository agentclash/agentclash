# codex/fix-deployment-picker-enter - Test Contract

## Functional Behavior
- `agentclash run create` in an interactive TTY must preserve deployment selections made with Space when the user presses Enter to confirm.
- Selecting one or more deployments in the interactive multi-select must satisfy the minimum-selection validator instead of being treated as empty input.
- The interactive picker must still resolve the selected deployment labels back to the correct deployment IDs in request order.
- Non-interactive validation must still require `--challenge-pack-version` and `--deployments` when guided selection is unavailable.
- Explicit `--deployments` flags must still bypass the guided picker entirely.

## Unit Tests
- `TestSurveyPickerMultiSelectPreservesSelectionsOnSubmit` - a `survey.MultiSelect` response containing checked options is accepted and returned as deployment selections.
- Existing picker normalization behavior remains covered so duplicate labels still resolve correctly.

## Integration / Functional Tests
- `TestRunCreateGuidedSelectionPostsResolvedIDs` still passes and proves the guided run-create flow posts the selected deployment IDs.
- `TestRunCreateNonInteractiveRequiresExplicitFlags` still passes.
- `TestRunCreateExplicitFlagsBypassGuidedPrompts` still passes.

## Smoke Tests
- `go test ./cmd -run 'TestSurveyPickerMultiSelectPreservesSelectionsOnSubmit|TestRunCreateGuidedSelectionPostsResolvedIDs|TestRunCreateNonInteractiveRequiresExplicitFlags|TestRunCreateExplicitFlagsBypassGuidedPrompts'`
- Local TTY repro against a mock API:
  press Space on two deployments, press Enter, expect the command to create the run instead of redrawing the prompt with an empty selection error.

## E2E Tests
- N/A - not applicable for this focused CLI bug fix.

## Manual / cURL Tests
```bash
cd cli
go test ./cmd -run 'TestSurveyPickerMultiSelectPreservesSelectionsOnSubmit|TestRunCreateGuidedSelectionPostsResolvedIDs|TestRunCreateNonInteractiveRequiresExplicitFlags|TestRunCreateExplicitFlagsBypassGuidedPrompts' -count=1
```

```bash
AGENTCLASH_API_URL=http://127.0.0.1:18080 AGENTCLASH_WORKSPACE=ws-1 agentclash run create --challenge-pack-version ver-1
# In the deployment picker: press Space on two items, then Enter.
# Expected: the run is created; no "choose at least 1 option(s)" redraw.
```
