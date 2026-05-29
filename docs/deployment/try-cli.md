# Try CLI deployment

Try CLI splits across **Vercel** (Next.js UI + API proxy) and a **long-running Bun service** (WebSocket + E2B PTY). The main AgentClash API (Go) stays on **Railway** with Postgres and Redis — Try CLI does not use that database.

## Architecture

| Component | Host | Notes |
| --- | --- | --- |
| Web UI `/try` | Vercel | `agentclash/web` |
| HTTP proxy `/api/try/*` | Vercel | Forwards to Try CLI service |
| WebSocket `/ws` | Railway or Fly | Must be long-lived; set `NEXT_PUBLIC_TRY_CLI_WS_URL` |
| E2B sandboxes | E2B cloud | `E2B_API_KEY` on Try CLI service only |

## Vercel (frontend)

Project: AgentClash `web/`

```env
TRY_CLI_API_URL=https://<try-cli-service-host>
NEXT_PUBLIC_TRY_CLI_API_URL=/api/try
NEXT_PUBLIC_TRY_CLI_WS_URL=wss://<try-cli-service-host>
NEXT_PUBLIC_TRY_CLI_PUBLIC_URL=https://www.agentclash.dev/try
```

Deploy via existing Vercel Git integration on `main` after merge.

Optional DNS: `try.agentclash.dev` → Vercel (rewrite to `/try` is in `next.config.ts`).

## Free-trial gateway (anonymous AI demos)

Anonymous visitors can try the AI coding agents with **no login and no API key**
for a few minutes. This is served by a metered gateway built into the Try CLI
service: the real provider keys live only on the service, each session gets a
short-lived spend-capped proxy token, and the sandbox CLIs point their base URL
at `/gw/<provider>`. Signed-in users instead bring their own credentials and get
a longer session (so they cost us nothing).

Set on the **Railway Try CLI service**:

```env
# Provider keys (server-side only — never sent to the sandbox)
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
XAI_API_KEY=xai-...            # enables the Grok free trial
OPENROUTER_API_KEY=sk-or-v1-... # enables Kimi K2 (kimi-cli) + Qwen3-Coder (qwen-code)
OPENCODE_ZEN_API_KEY=sk-...      # enables the opencode demo on opencode Zen models
# Where the sandbox CLIs reach the gateway (this service's own public URL)
TRY_CLI_GATEWAY_URL=https://try-cli-production.up.railway.app
# Durable daily spend ceiling (THE backstop) — reference the project's Redis
REDIS_URL=${{Redis.REDIS_URL}}
# Shared secret so the Vercel proxy can grant the signed-in (BYO) tier
TRY_CLI_PROXY_SECRET=<random-string>
# Caps (optional; shown with defaults)
GW_DAILY_CEILING_USD=5
GW_SESSION_BUDGET_USD=0.30
GW_ANON_MINUTES=7
GW_AUTH_MINUTES=20
GW_MAX_OUTPUT_TOKENS=2048
```

On **Vercel** (web), set the matching `TRY_CLI_PROXY_SECRET` so the proxy attaches
the signed-in user id. Without it, everyone is treated as anonymous (safe default).

> The daily ceiling is Redis-backed so it survives restarts and multiple
> instances — it's the hard cap on what an abuser can spend. The per-session
> budget is the per-visitor comfort cap. If Redis is unreachable the gateway
> fails **closed** (reports the ceiling as reached).

## E2B template (prerequisite)

Demos boot from a shared E2B template (`agentclash-trycli`) with every CLI
pre-installed, so sandboxes start in ~1–2s instead of installing on each visit.
Build it (runs in E2B's cloud — no local Docker) before the first deploy and
again whenever the tool set changes:

```bash
cd services/try-cli
E2B_API_KEY=... bun run scripts/build-template.ts
```

The same `E2B_API_KEY` must be set on the Try CLI service so it can launch the
template at runtime (`Sandbox.create("agentclash-trycli", …)`).

## Try CLI service (Railway recommended)

Deploy `services/try-cli/` as a **separate Railway service** in the same project as the backend:

1. New service → deploy from repo, root directory `services/try-cli`
2. Start command: `bun run start`
3. Public networking on port `3001`
4. Secrets:
   - `E2B_API_KEY`
   - `TRY_CLI_CORS_ORIGINS=https://www.agentclash.dev,https://agentclash.dev,https://try.agentclash.dev`
   - `PORT=3001`

Generate domain e.g. `try-cli-production.up.railway.app` and set Vercel `TRY_CLI_API_URL` / `NEXT_PUBLIC_TRY_CLI_WS_URL` to that host.

### Dockerfile (alternative)

`services/try-cli/Dockerfile` + `fly.toml` are included for Fly.io if preferred over Railway.

## Local dev

```bash
# Terminal 1
cd services/try-cli && E2B_API_KEY=... bun install && bun run dev

# Terminal 2
cd web && TRY_CLI_API_URL=http://localhost:3001 \
  NEXT_PUBLIC_TRY_CLI_WS_URL=ws://localhost:3001 \
  pnpm dev
```

Open http://localhost:3000/try

## Maintainer CLI

```bash
npx @agentclash/try-cli init
npx @agentclash/try-cli publish
```
