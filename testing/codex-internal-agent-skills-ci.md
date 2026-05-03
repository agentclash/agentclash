# Codex Internal Agent Skills CI - Test Contract

## Functional Behavior
- Add an internal experimental GitHub Actions workflow that runs only when `web/content/agent-skills/**/SKILL.md` changes or when manually dispatched.
- The workflow must run AgentClash, not just a local script, by creating and running an AgentClash `agent-harness` for each changed skill.
- Each harness must simulate a coding agent that does not know the AgentClash repo and is instructed to rely only on the changed `SKILL.md`.
- The workflow must use hosted production by default and read credentials from GitHub secrets or vars, without printing OpenAI or AgentClash tokens.
- The workflow must support a fixed OpenAI secret name (`OPENAI_API_KEY`) and sync the GitHub `OPENAI_API_KEY` secret into the AgentClash workspace secret before creating harnesses.
- Checks must be deterministic at the gate layer: the LLM writes a structured JSON artifact and a command validator checks schema, skill identity, required sections, CLI-hosted-backend expectations when applicable, and absence of repo-source dependency claims.
- The harness generator must work for any new skill file, not hardcode deployment/runtime/build-specific terms.

## Unit Tests
- Add local tests for the harness-preparation script that verify changed skill detection, prompt generation, harness JSON shape, and validator command shape.
- Add local tests for the validator script that cover passing JSON, missing fields, wrong skill identity, CLI-required hosted backend omissions, and invalid JSON.

## Integration / Functional Tests
- The workflow YAML should be syntactically valid and reference the checked-in scripts.
- The script-generated harness specs should be usable with `agentclash agent-harness create --from-file`.

## Smoke Tests
- Run the new script tests locally.
- Run a dry-run harness preparation command against one existing skill file.

## E2E Tests
N/A - the real AgentClash/OpenAI harness execution requires repository secrets and hosted infrastructure, so the PR should include deterministic dry-run coverage plus the GitHub Action wiring.

## Manual / cURL Tests
- Review `.github/workflows/internal-agent-skills-harness.yml` and confirm it only triggers for agent-skill changes and `workflow_dispatch`.
- Review `scripts/internal-agent-skills/prepare-skill-harnesses.mjs` and `validate-skill-harness-output.mjs` to confirm the pass/fail contract is skill-generic.
