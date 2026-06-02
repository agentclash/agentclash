---
name: agentclash-skill-catalog
description: Use when creating, reviewing, or updating AgentClash agent-skill folders so the catalog taxonomy, frontmatter, generated docs, markdown exports, and llms.txt surfaces stay consistent.
metadata:
  agentclash.role: catalog
  agentclash.version: "1"
  agentclash.requires_cli: "false"
---

# AgentClash Skill Catalog

## Purpose
Define the folder taxonomy and publishing contract for AgentClash Agent Skills. Use this skill before adding or changing any `web/content/agent-skills/**/SKILL.md` file.

## Use When
- A user asks to add a new AgentClash skill.
- A user asks to update the skill catalog, taxonomy, metadata, generated docs, markdown exports, or `llms.txt` coverage.
- A reviewer needs to check whether a skill can be copied into Codex, Claude Code, Cursor, or another coding-agent workflow.

## Do Not Use When
- The task is only to run an eval, read a scorecard, configure the CLI, or author a challenge pack.
- The task changes product docs outside the Agent Skills catalog and does not affect skill discovery.

## Inputs Needed
- Feature area and intended user workflow.
- The upstream skill dependencies that must be read first.
- Source-backed command names, field names, examples, and failure modes.
- Whether the workflow targets hosted production, local development, or a self-hosted backend.

## Canonical Folder Taxonomy
The canonical source is always a `SKILL.md` file under `web/content/agent-skills`.

```text
web/content/agent-skills/SKILL.md
web/content/agent-skills/<top-level-skill>/SKILL.md
web/content/agent-skills/agent-build-skills/<skill>/SKILL.md
web/content/agent-skills/challenge-pack-skills/<skill>/SKILL.md
```

Use top-level folders for cross-cutting workflows such as CLI setup, eval running, scorecard reading, regression, and CI gates. Use `agent-build-skills/` for agent build specs, runtime resources, deployments, providers, secrets, and model aliases. Use `challenge-pack-skills/` for challenge pack planning, YAML authoring, inputs, tools, artifacts, scoring, judges, validation, and publish workflows.

## Required Frontmatter
Every skill must start with YAML frontmatter that the docs generator can parse with `gray-matter`.

```yaml
---
name: agentclash-example-skill
description: Use when the trigger is specific enough that an agent can choose this skill before reading the body.
metadata:
  agentclash.role: example
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---
```

Field rules:
- `name`: stable kebab-case skill identifier, usually matching the folder name.
- `description`: trigger-oriented sentence that starts with "Use when" and names the workflow, not a generic summary.
- `metadata.agentclash.role`: short role label shown in the catalog list.
- `metadata.agentclash.version`: string version for the skill contract.
- `metadata.agentclash.requires_cli`: string `"true"` or `"false"` so installers and reviewers can spot CLI-dependent skills.

## Required Body Sections
Each skill should be useful without reading the AgentClash source code. Include these sections unless the section is not applicable and explicitly say why.

- `Purpose`: one paragraph describing the workflow outcome.
- `Use When`: concrete triggers that should load the skill.
- `Do Not Use When`: nearby workflows that should choose a different skill.
- `Inputs Needed`: information the agent should collect before acting.
- `Environment`: backend defaults, credentials, workspace, and local/self-hosted differences.
- `Procedure`: ordered operating steps.
- `Commands`: copyable commands with placeholders.
- `Expected Output`: what success looks like.
- `Failure Modes`: common errors and recovery steps.
- `Safety Notes`: secrets, destructive actions, cost, publish, or production cautions.
- `Report Back Format`: concise format the agent should use when done.
- `Related Docs`: `/docs-md/...` links that support the workflow.

## Generated Docs Contract
The web docs generator discovers skill files from `web/content/agent-skills/**/SKILL.md`.

- `/docs/agent-skills` and `/docs-md/agent-skills` render the catalog index and this catalog contract.
- `/docs/agent-skills/<skill>` and `/docs-md/agent-skills/<skill>` render individual top-level skills.
- `/docs/agent-skills/<category>/<skill>` and `/docs-md/agent-skills/<category>/<skill>` render nested category skills.
- `/llms.txt` includes the Agent Skills entry and every discovered skill page.
- `/llms-full.txt` includes the Agent Skills catalog, category pages, and full skill bodies.

When adding a new category, update the docs navigation and category map in `web/src/lib/docs.ts` so the category page, markdown path, and bundle order are explicit.

## Hosted Backend Examples
Use hosted production by default:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth login --device
agentclash workspace list
agentclash workspace use <workspace-id>
```

Only use local or self-hosted URLs when the skill is explicitly about local development or deployment:

```bash
agentclash --api-url http://localhost:8080 doctor
```

## Authoring Procedure
1. Pick the folder from the taxonomy.
2. Read the related upstream skills in dependency order.
3. Verify command names, config names, YAML fields, and API behavior from source-backed docs or code.
4. Write trigger-oriented frontmatter and the required body sections.
5. Prefer production hosted examples unless the workflow is local or self-hosted.
6. Add or update docs-generation tests when a new path, category, or export behavior is introduced.
7. Run the docs tests and lint before opening a PR.

## Validation Commands
```bash
cd web
npm test -- docs.test.ts
npm run lint
```

For CLI packaging-related skill changes, also validate from `cli/`:

```bash
go build ./...
go test -short -race -count=1 ./...
```

## Failure Modes
- Missing frontmatter: the generated page may have an empty name or description.
- Vague description: agents may not discover the skill at the right time.
- New category without generator updates: `/docs-md/agent-skills/<category>` may not exist.
- Examples that default to localhost: users may accidentally run against the wrong backend.
- Claims not tied to current docs or code: downstream skills will repeat incorrect fields or commands.

## Safety Notes
- Do not include tokens, workspace secrets, or customer data in examples.
- Do not tell agents to publish, delete, or mutate production resources without an explicit confirmation step.
- Prefer read-only discovery commands before write commands.
- Keep skill instructions portable; avoid relying on a single agent product unless the section is explicitly install-target guidance.

## Dependency Order
Read related skills in this order so downstream workflows do not redefine upstream concepts:

1. `agentclash-skill-catalog`
2. `agentclash-hub`
3. `agentclash-cli-setup`
4. `agentclash-quickstart`
5. `agentclash-runtime-resources-setup`
6. `agentclash-agent-build-author`
7. `agentclash-agent-deployment-setup`
8. `agentclash-challenge-pack-planner`
9. `agentclash-challenge-pack-yaml-author`
10. `agentclash-challenge-pack-input-sets`
11. `agentclash-challenge-pack-tools-sandbox`
12. `agentclash-challenge-pack-artifacts`
13. `agentclash-challenge-pack-scoring-validators`
14. `agentclash-challenge-pack-llm-judges`
15. `agentclash-challenge-pack-validation-publish`
16. `agentclash-eval-runner`
17. `agentclash-scorecard-reader`
18. `agentclash-compare-and-triage`
19. `agentclash-regression-flywheel`
20. `agentclash-ci-release-gate`

## Review Checklist
- The folder path matches the taxonomy.
- Frontmatter includes `name`, trigger-oriented `description`, and the three `metadata.agentclash.*` fields.
- Commands and examples use `https://api.agentclash.dev` unless local or self-hosted behavior is explicit.
- The body includes inputs, exact fields or commands, expected outputs, failure modes, safety notes, and report-back format.
- Related docs use `/docs-md/...` links.
- Generated docs, markdown exports, `llms.txt`, and `llms-full.txt` include the updated content.

## Report Back Format
```text
Skill: <name>
Path: web/content/agent-skills/<path>/SKILL.md
Category: <core | agent-build-skills | challenge-pack-skills>
Docs: /docs-md/agent-skills/<path>
Validation: <commands run and result>
Notes: <source-backed caveats or follow-ups>
```

## Related Docs
- `/docs-md/agent-skills`
- `/docs-md/guides/use-with-ai-tools`
- `/docs-md/reference/cli`
- `/docs-md/reference/config`
