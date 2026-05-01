# codex/issue-468-github-app-repo-picker - Test Contract

## Functional Behavior
- AgentClash models GitHub App installations as organization-owned records, workspace bindings, and cached installation repositories; no access tokens are persisted.
- Workspace users can list bound GitHub installations and repositories from workspace-scoped endpoints only after normal workspace authorization.
- Workspace admins can start a GitHub install flow and receive a GitHub App installation URL carrying workspace return metadata.
- Workspace admins can complete a GitHub installation after GitHub redirects back with `installation_id` and signed `state`; the backend verifies the state, verifies the installation through GitHub App auth, binds it to the workspace organization, and syncs repositories.
- GitHub webhooks verify `X-Hub-Signature-256` before handling installation and repository access changes; unknown/unbound installations are ignored safely until a user binds them.
- GitHub App credentials and short-lived installation tokens are used only in memory for verification and repository sync; no GitHub access tokens are persisted.
- Agent harness create accepts either the existing `repository_url` fallback or structured GitHub metadata: `repository_provider: "github"`, `github_repository_id`, optional `github_installation_id`, and `base_branch`.
- Structured harness create validates that the selected repository belongs to an active installation bound to the workspace, stores denormalized repo metadata on the harness, and defaults `base_branch` from the cached GitHub repository default branch.
- Harness snapshots include the structured repository metadata so executions remain auditable if installation access later changes.
- The UI keeps URL fallback available, shows a GitHub empty state when no installation/repos are connected, and allows choosing a cached GitHub repository/branch when available.

## Unit Tests
- `TestGitHubIntegrationManagerStartInstallationRequiresAdminAction` - viewer/member cannot start install; admin receives a stateful install URL.
- `TestGitHubIntegrationManagerCompleteInstallationVerifiesStateAndSyncsRepositories` - signed setup state is verified, installation metadata is upserted, workspace binding is created, and repos are cached.
- `TestGitHubWebhookHandlerRejectsInvalidSignature` - invalid webhook signatures are rejected before payload handling.
- `TestGitHubIntegrationManagerListsWorkspaceRepositories` - only active repositories from active installations bound to the workspace are listed.
- `TestAgentHarnessManagerCreateValidatesGitHubRepositoryBinding` - unknown/unbound repo is rejected with `github_repo_not_installed`.
- `TestAgentHarnessManagerCreatePersistsGitHubRepositoryMetadata` - provider, repository ID, installation ID, full name, clone URL, and branch are persisted.
- Existing agent harness route tests continue passing with URL fallback payloads.
- Frontend create-dialog tests cover the empty connect state and GitHub repository payload path.

## Integration / Functional Tests
- Backend short test suite passes for `./internal/api/...` and `./internal/repository/...`.
- Web component tests for the agent harness create dialog pass.
- Migrations apply cleanly against the local Postgres stack.

## Smoke Tests
- `curl -fsS http://localhost:8080/healthz` returns success with the local stack running.
- Authenticated workspace API calls remain protected; unauthenticated calls to the new GitHub endpoints return auth errors instead of data.

## E2E Tests
- Local stack starts through `scripts/dev/start-local-stack.sh`; the API server and worker become healthy.
- Full GitHub App OAuth/setup cannot complete locally without production GitHub App credentials, so E2E verification is limited to local API health, auth protection, migrations, and cached-repository behavior through unit tests.

## Manual / cURL Tests
```bash
curl -fsS http://localhost:8080/healthz
# Expected: 200 response from the local API server.

curl -i http://localhost:8080/v1/workspaces/00000000-0000-0000-0000-000000000000/github/repositories
# Expected: 401/403 auth error and no repository data.
```
