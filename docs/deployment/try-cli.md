# Try CLI deployment

The frontend and HTTP proxy stay on Vercel. One long-running Bun process serves
terminal HTTP, browser WebSockets and sandbox gateway calls behind Caddy. E2B
continues to host disposable sandboxes. The main API's Postgres data and Temporal
storage are separate migration work; this service uses Redis-compatible storage
for trial admission and estimated gateway spending.

## Request paths and ownership

| Caller | Path | Destination |
| --- | --- | --- |
| Browser | `/api/try/*` | Vercel proxy, then terminal `/api/*` |
| Browser | `/ws?sessionId=...` | Caddy, then the terminal process |
| E2B CLI | `/gw/<provider>/...` | Caddy, then the metered gateway |
| Host monitor | `/health` / `/health/ready` | Private terminal listener |

Run exactly **one terminal process**. Session, PTY and gateway-token ownership is
in memory. Redis persistence does not make sessions transferable across replicas.
Load-balancer stickiness cannot coordinate browsers and separately originating
E2B requests. Keep the terminal listener private and expose only Caddy.

Session IDs are bearer capabilities for terminal access; gateway tokens are
separate credentials. Do not record query strings, authorization headers, request
bodies, terminal content or environment values in proxy/application logs.

## Runtime configuration

Use the placeholder-only [environment example](../../services/try-cli/.env.example).
Resolve real values through the deployment secret manager into the process
outside the checkout. Do not paste values into images, Compose examples, CI
artifacts or shell arguments. The production command disables automatic `.env`
loading. Use `bun run start` or the Dockerfile's command; `--preserve-symlinks` is
required for the local core package's dependency resolution.

Production startup requires:

- E2B credentials and a working E2B API check. The configured demo templates must
  separately be verified during deployment rehearsal; readiness creates no sandbox.
- `REDIS_URL` (or `VALKEY_URL`) using `rediss://`, with certificate verification.
  Mount `REDIS_TLS_CA_FILE` for a private CA; omit it for a system-trusted CA.
  Verification cannot be disabled. Persist the Redis/Valkey data and use AOF;
  this service cannot enforce durable limits against a disposable cache.
- Explicit HTTPS `TRY_CLI_CORS_ORIGINS`, a random `TRY_CLI_PROXY_SECRET` of at least
  32 bytes and an HTTPS `TRY_CLI_GATEWAY_URL` origin without a path.
- Valid positive numeric limits. Development alone permits mock sandboxes and
  in-memory limits. Provider keys are optional and enable only supported trials.

Redis permissions must cover `PING`, `GET`, `SET`, `DEL`, `EXPIRE`, `INCRBYFLOAT`
and `EVAL` on `trycli:*` (including commands invoked by scripts). Admission uses
`SET NX EX`; daily holds and settlement use Lua. Existing `trycli:trial:used:*`
and `trycli:gw:daily:*` keys keep their meaning. Copy them, including expiries,
when migrating the cache. Never clear them to make a deployment appear healthy.

`/health` is cheap process liveness. `/health/ready` verifies Redis, periodically
checks E2B and reflects drain/cleanup/metering state. Dependency failures reject
new session/reset admission. Readiness is an admission signal: do **not** configure
Caddy to remove the only upstream on this probe during drain, because existing
WebSockets and gateway callbacks must still reach it.

## Forwarded identity and Caddy

The Vercel route creates fresh forwarding headers. It derives user identity from
WorkOS and client IP from Vercel's platform-owned `x-real-ip`, then attaches the
shared secret for **both anonymous and signed-in** requests. The service trusts
`X-Agentclash-Client-IP` and `X-Agentclash-User` only with that secret. Never expose
it as a `NEXT_PUBLIC_*` variable. An arbitrary client `X-Forwarded-For` does not
establish identity. Self-hosting this frontend proxy requires a separately
reviewed platform IP trust configuration.

Caddy overwrites `X-Trycli-Client-IP` using its resolved peer/client address:

```caddyfile
https://terminal.example.com {
    reverse_proxy terminal:3001 {
        header_up X-Trycli-Client-IP {http.request.client_ip}
        stream_close_delay 5m
    }
}
```

Set `TRY_CLI_TRUSTED_PROXY_CIDRS` to the immediate Caddy address(es), using exact
host CIDRs where possible. A dedicated network and firewall must prevent other
containers from reaching the terminal as that trusted peer. Do not trust all
private networks. When adding Cloudflare, review Caddy's global trusted proxies
and client-IP headers against the actual origin path; do not trust arbitrary
incoming Cloudflare/XFF headers. This is part of the infrastructure rehearsal.

Browser WebSockets require an allowlisted `Origin`. Bun sends ping frames and
allows 60 seconds of idle connection time; a responsive browser stays connected.
Caddy tunnels upgrades automatically. Keep proxy streaming timeouts above the
session lifetime, leave response buffering off and allow gateway cancellation
on disconnect. Caddy reloads can close tunnels, so drain before proxy replacement
as well as terminal replacement. See the official [Caddy proxy reference](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
and [Vercel request headers](https://vercel.com/docs/headers/request-headers).

## Trial spending and credential boundaries

Codex, Grok, Kimi and Qwen can use the metered gateway when their provider key is
configured. Signed-in sessions are BYO. Claude Code and opencode are BYO-only;
`OPENCODE_ZEN_API_KEY` is not injected or used. E2B hosting still has costs for BYO
sessions.

Real provider keys stay in the gateway. Sandboxes receive only a short-lived
session token and a gateway URL. The gateway permits supported inference POST
paths only, limits body/output sizes, disables background Responses and uses
fresh upstream headers. Other provider API operations are not exposed.

Session admission and daily estimated spending fail closed. Each in-flight
request reserves budget before contacting the provider. Aborted responses,
missing/invalid usage and abandoned process reservations retain an estimated
charge. Confirmed usage settles the reservation; a settlement failure closes
new gateway admission and fails readiness. Anonymous resets preserve both the
original deadline and the shared budget, including requests still being metered.

Token pricing is approximate, and a request can cost more than its reservation.
These controls are **not an exact provider billing ceiling**. Configure provider
account limits and monitor actual invoices during rehearsal; do not advertise
an exact dollar guarantee. Defaults are a $0.30 session budget, $5 daily estimate,
$0.05 request reservation, 2,048 output tokens and a 120-second gateway deadline.

## Drain and shutdown

Send signals to the Bun process, not a wrapper that fails to forward signals.
For a local development process (replace the PID from your process manager):

```sh
curl --fail http://127.0.0.1:3001/health
curl --fail http://127.0.0.1:3001/health/ready
kill -USR2 "$TERMINAL_PID"
# Liveness remains 200; readiness and session/reset admission become 503.
curl --silent --output /dev/null --write-out '%{http_code}\n' http://127.0.0.1:3001/health/ready
# After existing sessions have ended, terminate the process.
kill -TERM "$TERMINAL_PID"
```

`SIGUSR2` irreversibly drains admission in that process. Existing terminals and
gateway callbacks continue. Wait for the configured session lifetime (at most
20 minutes by default for signed-in sessions, further limited by each demo),
or explicitly choose to end the remaining sessions during a maintenance window.
Do not start a replacement alongside the old process.

`SIGTERM`/`SIGINT` close sockets, stop accepting connections, abort gateway work,
await metering and tracked sandbox cleanup, and finally close Redis. The default
shutdown deadline is 45 seconds (`TRY_CLI_SHUTDOWN_MS`). Give Compose/systemd at
least **60 seconds** before forced termination. SDK requests use bounded timeouts;
failed or timed-out cleanup exits nonzero. After an uncertain E2B create, capacity
remains occupied and readiness fails even after local expiry: the local clock
cannot prove a remote allocation was never created. Reconcile E2B resources tagged
`service=try-cli` and their session metadata privately before replacement. A failed
shutdown must block the deployment job rather than be ignored.

Destroy invalidates access immediately but releases capacity only after confirmed
cleanup. Browser reconnects reuse one PTY; old sockets lose input ownership. No
in-memory sessions or terminals resume across a process replacement.

## Friend-managed Vercel handoff (later migration steps)

Prepare these values privately and send instructions to the frontend owner during
Steps 13–14. Do not deploy the frontend as part of terminal preparation.

```env
TRY_CLI_API_URL=https://terminal.example.com
TRY_CLI_PROXY_SECRET=<same-secret-as-terminal>
NEXT_PUBLIC_TRY_CLI_API_URL=/api/try
NEXT_PUBLIC_TRY_CLI_WS_URL=wss://terminal.example.com
NEXT_PUBLIC_TRY_CLI_PUBLIC_URL=https://frontend.example.com/try
```

The proxy code and terminal trust contract must be deployed together at cutover.
The frontend must be on Vercel (`VERCEL=1` supplied by the platform); missing
production settings return 503. Set all actual browser origins on the terminal
allowlist. After the owner's redeploy, verify anonymous admission, WorkOS sign-in
and BYO tier, browser WebSocket reconnection, expiry/reset and real E2B gateway
reachability. Public DNS, certificates, Cloudflare and live Vercel behavior are
not proven by local tests.

## Isolated validation

From the repository root:

```sh
python3 scripts/deployment/test-terminal.py --evidence-dir /private/path/outside-checkout
```

Requires local Docker, Caddy, OpenSSL, curl and Bun. It generates a test CA and runtime
credentials outside Git, starts one disposable loopback-only TLS Redis container,
and exercises real HTTP/WS through Caddy using synthetic E2B/provider dependencies.
It tests certificate rejection, client/server restart persistence, both tiers,
65-second idle WebSockets, drain and process shutdown, then removes its container
and temporary private keys. It never contacts real E2B/provider services. Use
`--test-path tests/integration/limits.test.ts` for a focused Redis rerun.

For unit checks: `cd services/try-cli && bun install --frozen-lockfile && bun run test`
(integration cases skip without the isolated harness), followed by `bun run typecheck`.
The actual Next route also has a runtime test with a synthetic WorkOS session;
its separate typecheck uses the frozen `web/` dependencies. Live sign-in still
requires the frontend owner's rehearsal.
Local development uses `bun run dev`; mock sessions are allowed without E2B credentials.
E2B template builds are separate cloud operations and require the later approved
rehearsal, not this local validation command.
