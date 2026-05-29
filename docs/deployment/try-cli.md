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
