# Hosted provider credentials — test contract

## Functional behavior
- Hosted provider connections accept exactly one raw API key or a valid reference to an existing secret in the current workspace vault.
- Invalid, missing, mixed or unavailable credentials fail before persistence or provider calls.
- Workspace credential resolution never falls back outside its vault; existing rows follow the same rules.
- Trusted local CLI credential resolution remains available.

## Unit tests
- Connection creation rejects invalid inputs without writes; raw keys become vault references.
- Reference creation and resolution use only the requested workspace, including missing secrets and unavailable workspaces.
- Model listing, cached listing and testing reject invalid existing references without upstream calls.
- Workspace-scoped runtime resolution rejects non-vault references and validates secret names; ordinary local environment resolution still works.
- API validation uses the same input contract; judge account selection validates workspace credentials.

## Integration and smoke tests
- Run the backend and runtime build, vet and short race suites and configured secret scan.
- Behind maintenance, verify rejected hosted inputs create no records and a vault-backed connection can list models.

## E2E tests
Existing login, native workflow, artifact, SSE and terminal acceptance remain applicable to unchanged flows. No additional paid run or sandbox is needed for this change.

## Manual checks
Use an authenticated workspace owner to submit an invalid reference and verify rejection. Store a key through the normal encrypted secret API, connect its vault reference, and list models. Keep actual deployment evidence outside this repository.

## Build input availability
If a pinned public package is unavailable upstream, a retained public copy may be used only after the existing SHA256 verification. Test successful recovery, changed-byte rejection and symlink rejection. Retained copies keep their original versions and checksum validation. Apply only fixed-version updates required by concrete mandatory scanner findings; scanner gates remain unchanged.
