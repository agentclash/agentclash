# codex/guided-build-authoring — Test Contract

## Functional Behavior
- Creating a new build version should no longer drop users straight into a blank JSON-only draft. The flow should offer starter templates with plain-English descriptions and create the version from the selected template.
- Editing a draft build version should expose a guided authoring experience for non-expert users while keeping the same underlying product layer and API payloads.
- The guided experience should let users edit the highest-value agent settings in plain language, at minimum:
  - agent kind
  - role / mission / instructions
  - success criteria
  - output contract choice
  - a small set of execution preferences that map back into the existing JSON specs
- The editor must preserve an advanced JSON mode so experienced users can still inspect and directly edit the raw spec objects.
- Switching between guided mode and JSON mode must keep user edits in sync rather than forcing users to choose one forever.
- Existing versions that already contain valid JSON specs should load into the new editor without losing their current data.
- Validation and mark-ready behavior must keep working against the same backend endpoints.

## Unit Tests
- `CreateVersionButton` dialog tests:
  - opening the flow shows template choices before creation
  - creating from a starter template sends seeded spec JSON instead of only empty defaults
  - creating from “blank” remains possible
- `VersionEditor` tests:
  - guided fields hydrate from an existing version payload
  - editing guided fields updates the saved PATCH payload correctly
  - guided output mode changes generate the expected `output_schema`
  - JSON mode still renders and saves raw spec edits
  - switching modes preserves edits

## Integration / Functional Tests
- The builds UI should still create a version, navigate to the version page, save a draft, validate it, and mark it ready without backend changes.
- The resulting PATCH payload should remain compatible with the current `/v1/agent-build-versions/{id}` API contract.

## Smoke Tests
- `pnpm vitest run` for the touched build authoring tests passes.
- `pnpm exec tsc --noEmit` passes for the web app.
- `pnpm exec eslint` passes for the touched files.

## E2E Tests
- N/A — not applicable for this change in this session.

## Manual / cURL Tests
- Manual UI flow:
  1. Open a workspace build page.
  2. Click `New Version`.
  3. Confirm a template picker appears with guided starter options.
  4. Choose a template and verify the created version lands on the version page with the guided editor visible by default.
  5. Edit mission/instructions/output choice in guided mode, save, refresh, and confirm the values persist.
  6. Switch to JSON mode and confirm the underlying spec objects reflect the guided selections.
  7. Validate and mark the version ready.
