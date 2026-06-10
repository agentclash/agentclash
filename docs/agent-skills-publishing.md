# Publishing Agent Skills via `gh skill`

AgentClash's Agent Skills (`web/content/agent-skills`) reach coding agents
through two channels, both generated from that single canonical source:

| Channel | For | How |
| --- | --- | --- |
| `agentclash integration <host> install` | Users who already have the CLI | Embedded snapshot (`cli/internal/skills/snapshot`); see [CLI workflow handoff](cli-workflow-handoff.md). |
| `agentclash skills export` | Offline or air-gapped install | Directory or `.tar.gz` from the same embedded snapshot; see [Use with AI tools](guides/use-with-ai-tools.mdx) in the web docs tree. |
| `gh skill install agentclash/agentclash <skill>` | Any [supported host](#supported-hosts) (incl. ones without the CLI) | Published GitHub release validated against the [agentskills.io](https://agentskills.io) spec. |

`web/content/agent-skills` is the source of truth. Do **not** hand-edit either
generated bundle.

## Build the publishable bundle

```bash
node scripts/build-skills-bundle.mjs
# -> dist/skills-bundle/skills/<name>/SKILL.md  (gitignored build artifact)
```

The script flattens the canonical skills into the `skills/*/SKILL.md` layout
`gh skill` discovers and **self-validates** every skill against the rules
`gh skill publish` enforces — kebab-case `name` ≤ 64 chars, `description`
present and ≤ 1024 chars, folder name equal to the frontmatter `name`, and no
duplicate names. A non-zero exit means the bundle is **not** publish-ready; fix
the offending `SKILL.md` in `web/content/agent-skills` and rebuild.

The root catalog (`web/content/agent-skills/SKILL.md`) is intentionally excluded:
it is a docs-navigation hub, and a root-level `SKILL.md` trips `gh skill`'s
`name="."` rejection ([cli/cli#13552](https://github.com/cli/cli/issues/13552)).

## Prerequisites (one-time, maintainer)

- **GitHub CLI ≥ 2.90.0** (`gh skill` ships in 2.90.0; check `gh --version`).
- On the `agentclash/agentclash` repo: `gh skill publish` interactively adds an
  `agent-skills` topic and cuts a GitHub release. It also validates/warns on tag
  protection, secret scanning, and code scanning — enable those or be ready to
  override. This needs repo-admin rights.
- **Public preview:** `gh skill` and the Agent Skills spec are in public preview
  and may change without notice. Pin the documented `gh` version and date-stamp
  any screenshots/output you keep.

## Validate and publish

```bash
# Validate locally first (no release is cut):
gh skill preview dist/skills-bundle
gh skill publish dist/skills-bundle --dry-run     # add --fix to auto-normalize

# Publish (semver tag). Prefer folding this into the existing v* release flow
# rather than ad hoc tags (see CLAUDE.md), once the cadence is decided:
gh skill publish dist/skills-bundle --tag v0.<minor>.0
```

## Automated publishing (CI)

`.github/workflows/publish-skills.yml` wires this up:

- **On pull requests** that touch `web/content/agent-skills/**` or the bundle
  script, it builds the bundle (`node scripts/build-skills-bundle.mjs`), which
  self-validates the `gh skill` spec — a token-free regression check.
- **Manually** (`workflow_dispatch`), it can publish. `dry_run` defaults to
  **true**, so a plain "Run workflow" only runs `gh skill publish --dry-run`
  and cuts nothing. Set `dry_run=false` for a real publish.

This is intentionally a **separate, manual** workflow — not folded into
`release-cli.yml` — because `gh skill publish` cuts its own GitHub release and
adds the `agent-skills` topic, which would conflict with the GoReleaser release
that the `v*` tag already produces, and needs repo-admin rights.

A real publish requires a repo-admin **`GH_SKILL_TOKEN`** secret (a PAT with
admin on `agentclash/agentclash`); the default `GITHUB_TOKEN` cannot add the
topic or cut the release. The dry-run falls back to `GITHUB_TOKEN`.

## Supported hosts

`gh skill install` writes to the correct directory for **Claude Code, GitHub
Copilot, Cursor, Codex, Gemini CLI, and Antigravity**. Users install a skill
with:

```bash
gh skill install agentclash/agentclash <skill> --agent <host>
```

## Discoverability

`gh skill publish` is repo-based, not a separate marketplace: discovery is via
the `agent-skills` repo topic and `gh skill search`, plus the install-by-name
path above. Surface the install command in the README and onboarding so users
know it exists.

## Notes

- `web/content/agent-skills` remains canonical; rebuild the bundle (and the CLI
  embed snapshot via `make cli-skills-snapshot`) whenever a skill changes.
- Adding `license: MIT` to each `SKILL.md` frontmatter is an optional spec
  enhancement; the repo `LICENSE` already applies.
