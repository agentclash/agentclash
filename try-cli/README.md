# AgentClash Try CLI

Interactive README demos for developer tools **and AI coding agents** — an AgentClash platform primitive. Run Claude Code, Codex, OpenCode, Grok, bun, uv, ruff, biome, or ripgrep in a disposable cloud terminal, no install.

**Live:** [agentclash.dev/try](https://www.agentclash.dev/try)

## Monorepo layout

```
try-cli/
  packages/core/    # .trycli.yml schema, badge SVG
  packages/cli/     # npx @agentclash/try-cli
  demos/            # Curated demo configs (AI agents + dev tools)

services/try-cli/         # Bun + E2B PTY WebSocket service (deploy to Railway)
services/try-cli/scripts/ # build-template.ts — builds the prebaked E2B template
web/src/app/try/          # Next.js UI (deploy to Vercel)
```

All demo tools are pre-installed in a shared E2B template (`agentclash-trycli`)
so sandboxes boot ready. Rebuild it after changing tools:

```bash
cd services/try-cli && E2B_API_KEY=... bun run scripts/build-template.ts
```

## Local development

```bash
# Terminal 1 — Try CLI service (port 3001)
cd services/try-cli
E2B_API_KEY=... bun install && bun run dev

# Terminal 2 — AgentClash web (port 3000)
cd web
pnpm install
TRY_CLI_API_URL=http://localhost:3001 \
NEXT_PUBLIC_TRY_CLI_WS_URL=ws://localhost:3001 \
pnpm dev
```

Open http://localhost:3000/try

Without `E2B_API_KEY`, the service runs in **mock terminal mode** for UI development.

## Deploy

### Vercel (web)

Set environment variables on the AgentClash web project:

```
TRY_CLI_API_URL=https://try-api.agentclash.dev
NEXT_PUBLIC_TRY_CLI_API_URL=/api/try
NEXT_PUBLIC_TRY_CLI_WS_URL=wss://try-api.agentclash.dev
NEXT_PUBLIC_TRY_CLI_PUBLIC_URL=https://www.agentclash.dev/try
```

Optional subdomain rewrite: `try.agentclash.dev` → `/try` (see `web/vercel.json`).

### Fly.io (Try CLI service)

```bash
cd services/try-cli
fly secrets set E2B_API_KEY=...
fly deploy --config fly.toml
```

Map `try-api.agentclash.dev` to the Fly app.

## CLI

```bash
npx @agentclash/try-cli init
npx @agentclash/try-cli validate
npx @agentclash/try-cli publish
```

## License

MIT — part of AgentClash
