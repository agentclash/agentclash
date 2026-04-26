# CLI Phase 1 Handoff

## What shipped

Phase 1 shipped an additive, workflow-first CLI layer on top of the existing resource-oriented command tree.

The new happy path is:

1. `agentclash auth login`
2. `agentclash link`
3. `agentclash challenge-pack init`
4. `agentclash challenge-pack validate`
5. `agentclash challenge-pack publish`
6. `agentclash eval start --follow`
7. `agentclash baseline set`
8. `agentclash eval scorecard`

The old `workspace`, `run`, and `compare` surfaces still exist and remain the advanced path.

## Commands Added Or Changed

Added:

- `agentclash link`
- `agentclash eval start`
- `agentclash eval scorecard`
- `agentclash baseline set`
- `agentclash baseline show`
- `agentclash baseline clear`
- `agentclash doctor`
- `agentclash challenge-pack init`

Changed:

- `agentclash run create`
  - added `--scope`
  - added `--suite`
  - added `--case`
  - local validation for `suite_only` and race-context cadence
- `agentclash run scorecard`
  - rendering now goes through shared scorecard helpers
- root help, `workspace use`, and `run create`/`run scorecard` help now point users toward the workflow-first path

## Config And State Added

User config now supports workspace-scoped baseline bookmarks in `~/.config/agentclash/config.yaml`.

Shape:

- `baseline_bookmarks.<workspace-id>.run_id`
- `baseline_bookmarks.<workspace-id>.run_agent_id`
- optional UX metadata: `run_name`, `run_agent_label`, `set_at`

This is intentionally local-only in Phase 1. There is no backend bookmark object.

## Main Files Changed

CLI:

- `cli/cmd/root.go`
- `cli/cmd/run.go`
- `cli/cmd/run_create_interactive.go`
- `cli/cmd/challenge_pack.go`
- `cli/cmd/workspace.go`
- `cli/cmd/link.go`
- `cli/cmd/eval.go`
- `cli/cmd/baseline.go`
- `cli/cmd/doctor.go`
- `cli/cmd/eval_resolve.go`
- `cli/cmd/run_create_helpers.go`
- `cli/cmd/scorecard_helpers.go`

Config:

- `cli/internal/config/config.go`
- `cli/internal/config/manager.go`

Tests:

- `cli/cmd/cmd_test.go`
- `cli/cmd/contract_alignment_test.go`
- `cli/cmd/link_test.go`
- `cli/cmd/eval_test.go`
- `cli/cmd/baseline_test.go`
- `cli/cmd/doctor_test.go`
- `cli/cmd/challenge_pack_init_test.go`
- `cli/internal/config/config_test.go`
- `cli/internal/config/manager_test.go`

Docs:

- `README.md`
- `npm/cli/README.md`
- `web/content/docs/getting-started/quickstart.mdx`
- `web/content/docs/guides/write-a-challenge-pack.mdx`
- `web/content/docs/contributing/testing.mdx`
- `testing/codex-cli-phase1-agent-first.md`

Support / smoke:

- `testing/cli-e2e-suite.sh`

## Tests And Self-Checks Added

New coverage includes:

- root help assertions for the workflow-first path
- `link` selector + interactive default-workspace save
- `baseline set/show/clear`
- `challenge-pack init`
- `eval start` selector resolution and request-shape assertions
- `eval scorecard --json` workflow envelope assertions
- ambiguous multi-agent handling for `eval scorecard`
- config persistence for workspace-scoped baseline bookmarks
- contract alignment for `run create` regression selector fields
- shell smoke checks for new command discovery/help in `testing/cli-e2e-suite.sh`

## Known Limitations

- Phase 1 is CLI-only. There is still no official Claude/Codex plugin, MCP server, or skills bundle.
- `baseline` bookmarks are local user config, not server-side state.
- `eval scorecard` still needs `--agent` or a TTY when a run has multiple run agents.
- `link` saves default workspace/org only. It does not replace repo-local `.agentclash.yaml`.
- `run create` remains the advanced ID-centric surface. The intent-first UX lives in `eval start`.
- `eval start` resolves suite names, but `--case` still expects case IDs.

## Deferred To Phase 2

- official Claude/Codex integration: plugin, MCP, skills, slash commands
- more intent-first workflow commands such as `quickstart`, `compare latest`, and `replay triage`
- update/discovery lifecycle commands for agent assets
- richer auth profiles and debug/diagnostic visibility
- server-side or shareable baseline bookmarks
- deeper workflow around regression cases by friendly selectors instead of raw IDs

## Suggested Next Entry Points

If another agent picks this up, start here:

- `cli/cmd/eval.go`
- `cli/cmd/doctor.go`
- `cli/cmd/eval_resolve.go`
- `cli/internal/config/config.go`
- `web/content/docs/getting-started/quickstart.mdx`

Likely next commands to add in Phase 2:

- `agentclash quickstart`
- `agentclash compare latest`
- `agentclash replay triage`
- `agentclash update`

## Verification Commands And Results

Succeeded:

- `cd cli && /tmp/agentclash-go/go/bin/go build ./...`
- `cd cli && /tmp/agentclash-go/go/bin/go vet ./...`
- `cd cli && /tmp/agentclash-go/go/bin/go test -short ./...`
- `cd cli && /tmp/agentclash-go/go/bin/go test -short -race -count=1 ./...`
- `bash -n testing/cli-e2e-suite.sh`
- `cd web && $HOME/Library/pnpm/.tools/pnpm/9.15.4_tmp_864_0/bin/pnpm build`

Repo-health note:

- `cd web && pnpm install --frozen-lockfile` failed because `web/pnpm-lock.yaml` was stale relative to `web/package.json` and missing `swr`.
- For verification, `cd web && $HOME/Library/pnpm/.tools/pnpm/9.15.4_tmp_864_0/bin/pnpm install --no-frozen-lockfile` succeeded.
- After the lockfile update, `cd web && $HOME/Library/pnpm/.tools/pnpm/9.15.4_tmp_864_0/bin/pnpm install --frozen-lockfile` also succeeded.
- The resulting `pnpm build` succeeded.
- The web build still emitted existing Next.js/Edge warnings around WorkOS imports plus webpack cache warnings, but it completed successfully.
