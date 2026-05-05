# codex/ci-metadata-default-branch — Test Contract

## Functional Behavior
- `POST /v1/runs` accepts `ci_metadata.default_branch` when the CLI detects it from GitHub Actions or receives `--ci-default-branch`.
- Accepted CI metadata is normalized consistently: surrounding whitespace is trimmed, empty metadata is omitted, and `default_branch` follows the same max length guard as other short CI metadata strings.
- Created runs persist `default_branch` inside `runs.ci_metadata`, and run creation/read responses include it when present.
- Existing clients that omit `default_branch` continue to create runs successfully.
- The CLI regression-promotion logic can rely on `candidate.ci_metadata.default_branch` echoed from the backend for `auto_on_main` decisions.

## Unit Tests
- `backend/internal/api` run creation tests cover decoding and echoing `ci_metadata.default_branch`.
- `backend/internal/api` validation tests cover the 512-character limit for `ci_metadata.default_branch`.
- `backend/internal/api` run read tests cover serialized metadata including `default_branch`.
- `cli/cmd` CI tests continue to prove GitHub Actions metadata includes `default_branch` and is passed in the run-create body.

## Integration / Functional Tests
- Repository CI metadata round-trip tests cover `default_branch` persistence through `runs.ci_metadata`.

## Smoke Tests
- `go test ./backend/internal/domain ./backend/internal/api ./cli/cmd` passes from the repository root.
- `go test ./backend/internal/repository` may require local database services; if unavailable, record the blocker and rely on the repository test source review.

## E2E Tests
- N/A — this schema-alignment fix is covered at API/CLI contract level. Full hosted GitHub rerun is optional after merge/deploy because the production API needs this backend change deployed.

## Manual / cURL Tests
```bash
curl -X POST "$AGENTCLASH_API_URL/v1/runs" \
  -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "workspace_id": "workspace-id",
    "challenge_pack_version_id": "challenge-pack-version-id",
    "agent_deployment_ids": ["deployment-id"],
    "ci_metadata": {
      "provider": "github_actions",
      "repository": "owner/repo",
      "branch": "main",
      "default_branch": "main"
    }
  }'
# Expected: 200/201-style run response, no "unknown field default_branch" JSON decode failure.
```
