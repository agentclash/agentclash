# codex/debug-claude-harness-pr - Test Contract

## Functional Behavior
- Claude Agent Harness executions must run Claude Code in a non-root sandbox context so `--permission-mode bypassPermissions` does not fail with Claude's root/sudo guard.
- AgentClash post-agent git collection must still work after Claude's repo ownership changes; root-side git commands must not fail with Git's dubious ownership guard for `/workspace`.
- GitHub PR creation must detect both dirty working-tree changes and agent-created commits ahead of the configured base branch.
- Structured GitHub harnesses must still prepare git authentication, clone private GitHub repos, push an execution branch, and create a draft PR.
- The shared Codex fullstack template must remain unchanged unless explicitly needed for the Claude fix.

## Unit Tests
- Workflow tests cover GitHub askpass preparation without depending on `session.WriteFile` ownership.
- Workflow tests cover agents that commit their own changes before returning; AgentClash must push those commits instead of skipping the PR as `no_changes`.
- Existing Claude runner tests continue to assert `--output-format stream-json`, `--verbose`, `--permission-mode bypassPermissions`, model override, and Anthropic secret mapping.

## Integration / Functional Tests
- `go test -short -race -count=1 ./internal/workflow` from `backend/`.
- `go test -short -race -count=1 ./internal/api ./internal/repository ./internal/workflow` from `backend/` if workflow changes are broader than the askpass preparation.

## Smoke Tests
- Rebuild hosted E2B template `agentclash-claude-fullstack`.
- Start the existing `Atharva-Kanherkar/e2b-go Claude` harness and verify the execution no longer fails with Claude's root/sudo error or root-side `/workspace` safe-directory error.
- After deploying the backend workflow change, rerun the same harness and verify a Claude-created local commit is pushed to an AgentClash branch and opened as a draft PR.

## E2E Tests
- Verify the rerun creates a draft PR when Claude commits locally; only truly empty base diffs should skip PR creation as `no_changes`.

## Manual / cURL Tests
```bash
AGENTCLASH_API_URL=https://api.agentclash.dev go run ./cli agent-harness run 616d9b02-e680-42dd-b947-fdbe73bb5a3e --follow
```
