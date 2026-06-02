# feat/skills-export — Test Contract

## Functional Behavior

`agentclash skills export` writes the embedded skills snapshot to a directory
or `.tar.gz` archive for offline install (issue #922 P2).

- Default `--format dir` writes `<skill>/SKILL.md` under `--dir`.
- With `--host`, writes host-relative paths (e.g. `.claude/skills/<skill>/SKILL.md`).
- `--format tar.gz` packages the same tree into a single archive.

## Unit Tests

```bash
cd cli && go test -short -count=1 ./internal/skills/... -run Export
cd cli && go test -short -count=1 ./cmd/... -run SkillsExport
go test ./cmd -run TestSchemaJSONMatchesGoldenSnapshot -update  # when command tree changes
```

## Smoke Tests

```bash
agentclash skills export --dir /tmp/ac-skills --format dir
agentclash skills export --dir /tmp/ac-skills-claude.tar.gz --host claude --format tar.gz
tar -tzf /tmp/ac-skills-claude.tar.gz | head
```

## Manual Tests

Extract tarball on a machine without network; copy into agent skills dir; run
`agentclash integration <host> doctor`.
