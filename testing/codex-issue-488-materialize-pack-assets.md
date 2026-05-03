# codex/issue-488-materialize-pack-assets - Test Contract

## Functional Behavior
- Native sandbox preparation materializes every artifact-backed `version.assets[]` entry at its declared `path` before the agent runs.
- Native sandbox preparation materializes every artifact-backed `challenges[].assets[]` entry from the challenge-pack manifest at its declared `path`.
- Native sandbox preparation materializes every artifact-backed asset on the selected input set's cases at its declared `path`.
- Asset bytes are loaded from artifact storage by `artifact_id`, scoped to the run workspace, and uploaded unchanged into the sandbox.
- Assets without `artifact_id` keep existing behavior and are not fetched from artifact storage.
- If a run references an artifact-backed asset but the worker has no asset loader, sandbox preparation fails closed with `sandbox_error`.
- If artifact metadata belongs to a different workspace or the object cannot be opened, sandbox preparation fails before the agent step starts.

## Unit Tests
- `backend/internal/engine`: test that artifact-backed version, challenge, and input-case assets are uploaded to their declared sandbox paths.
- `backend/internal/engine`: test that inline/non-artifact assets do not require an asset loader.
- `backend/internal/engine`: test that artifact-backed assets without a loader fail with `StopReasonSandboxError`.
- `backend/internal/worker`: test that the worker artifact asset loader reads bytes from storage and rejects cross-workspace artifact IDs.
- `backend/internal/worker`: test worker config loads artifact storage defaults and env overrides.

## Integration / Functional Tests
- `backend/cmd/worker` must compile with artifact storage initialization and native invoker wiring.
- `backend/internal/worker` must compile with `NewNativeModelInvoker...WithAssetLoader(...)` propagation into `engine.NativeExecutor`.

## Smoke Tests
- `go test ./internal/engine ./internal/worker ./cmd/worker` from `backend/`.

## E2E Tests
- N/A - this is a backend staging path covered by unit tests and compile checks; a real E2E would require external sandbox and artifact services.

## Manual / cURL Tests
- N/A - no HTTP API behavior changes; publish-time validation already accepted `version.assets[].artifact_id`.
