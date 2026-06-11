# `agentclash-tryout-office` E2B template

The single general-purpose sandbox image the **public agent tryout** runner
boots for every task. It is hardcoded as the default `E2BTemplateID` in
`backend/internal/workflow/activities.go`
(`NormalizePublicAgentTryoutConfig`), so a correctly-built template with this
name works with zero extra configuration.

## What's inside

- **Codex CLI** (`@openai/codex`) — the binary the `codex_e2b` runner invokes.
- Node.js 20, Python 3 + pip.
- Office document toolchain: `pandoc`, `poppler-utils`, LibreOffice (calc /
  writer / impress), and Python libs (`openpyxl`, `python-docx`,
  `python-pptx`, `pypdf`, `reportlab`, `pandas`, `markdown`, `Pillow`, …).

## Build + publish

Requires the E2B CLI (`npm i -g @e2b/cli`) and an authenticated session
(`e2b auth login`). From this directory:

```bash
e2b template build --name agentclash-tryout-office --cpu-count 2 --memory-mb 2048
```

The build creates/updates `e2b.toml` here (template ID is recorded in it).
Rebuild after editing the Dockerfile by re-running the same command.

## Wiring

The worker needs E2B configured:

```bash
SANDBOX_PROVIDER=e2b
E2B_API_KEY=<your key>
E2B_TEMPLATE_ID=<any default>          # used by the workspace harness path
```

The public tryout path defaults to `agentclash-tryout-office` automatically.
To override without a rebuild, set on **both** api-server and worker:

```bash
AGENT_TRYOUT_E2B_TEMPLATE_ID=agentclash-tryout-office
```
