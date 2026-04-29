# codex/refactor-playground-side-by-side - Test Contract

## Functional Behavior
- Playground detail opens as a side-by-side comparison workspace, not a tabbed prompt/eval/experiment split.
- Two model lanes are visible on the first screen with provider, model, label, temperature, timeout, trace mode, tools, knowledge, and run controls.
- The shared prompt, system prompt, evaluation config, and test cases remain editable from the same workspace.
- Launching the comparison creates a batch of two experiments with each lane's provider, model, label, and request config.
- Completed baseline/candidate results can be selected and compared without leaving the side-by-side view.
- Existing backend behavior is preserved: playground save, test case CRUD, experiment polling, and comparison query params continue to work.

## Unit Tests
- TypeScript compile/lint should catch prop and type regressions in the refactored playground components.
- Existing component tests should remain green where they overlap shared UI primitives.

## Integration / Functional Tests
- `PlaygroundDetailClient` should render with existing server-provided props and not require new backend endpoints.
- Experiment launch payloads should remain compatible with `/v1/playgrounds/{id}/experiments/batch`.
- Comparison query params should continue to drive `/v1/playground-experiments/compare`.

## Smoke Tests
- `cd web && pnpm lint`
- `cd web && pnpm test -- --run`

## E2E Tests
- N/A - no browser automation suite exists for this playground flow in the repo.

## Manual Tests
- Open a playground detail page and verify the first visible layout is two comparison lanes.
- Edit prompt/system/evaluation/test cases from the same page.
- Select two completed experiments and verify per-case outputs are displayed side by side.
- Launch a two-lane comparison and verify the page returns to the comparison workspace with polling active.
