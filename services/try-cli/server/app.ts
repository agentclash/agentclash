import { badgeSvg, type Demo } from "@try-cli/core";
import type { Config } from "./config.ts";
import type { Limits } from "./limits.ts";
import { SessionManager, AdmissionError, type TerminalSession } from "./sessions.ts";
import { Gateway } from "./gateway.ts";
import { requestIdentity } from "./trust.ts";
import { demoToMeta } from "./types.ts";
import { json, readBody, BodyError } from "./http.ts";

export interface ApplicationDeps {
  config: Config;
  limits: Limits;
  sessions: SessionManager;
  gateway: Gateway;
  sandboxHealth(): Promise<void>;
  registry: { list(): Demo[]; get(slug: string): Demo | undefined };
}

export function startService(deps: ApplicationDeps, listen = { hostname: "0.0.0.0", port: deps.config.port }) {
  const { config, sessions, gateway, limits, registry } = deps;
  const terminating = new AbortController();
  let draining = false;
  let healthUntil = 0;
  let sandboxCheck: Promise<void> | undefined;
  async function ready() {
    if (draining || !sessions.healthy || !gateway.healthy) throw new Error("Not ready");
    await limits.health();
    if (Date.now() >= healthUntil) {
      sandboxCheck ??= deps.sandboxHealth().then(() => { healthUntil = Date.now() + 10000; }).finally(() => { sandboxCheck = undefined; });
      await sandboxCheck;
    }
    if (draining) throw new Error("Not ready");
  }
  const info = (s: TerminalSession) => ({
    id: s.id, slug: s.slug, expiresAt: s.expiresAt, status: s.status, error: s.error,
    mock: s.mock, tier: s.tier, trial: s.trialWired,
    budgetUsd: s.budget.limit, spentUsd: Math.round(s.budget.spent * 1000) / 1000,
  });
  const server = Bun.serve<{ sessionId: string }>({
    ...listen, maxRequestBodySize: config.maxRequestBytes, idleTimeout: 30,
    async fetch(req, server) {
      const origin = req.headers.get("origin");
      const cors = (res: Response) => {
        res.headers.set("vary", "Origin");
        if (origin && config.origins.includes(origin)) {
          res.headers.set("access-control-allow-origin", origin);
          res.headers.set("access-control-allow-methods", "GET, POST, DELETE, OPTIONS");
          res.headers.set("access-control-allow-headers", "Content-Type");
        }
        return res;
      };
      const url = new URL(req.url);
      const path = url.pathname;
      try {
        if (path === "/health" && req.method === "GET") return json({ ok: true });
        if (path === "/health/ready" && req.method === "GET") {
          try { await ready(); return json({ ready: true }); }
          catch { return json({ ready: false }, 503); }
        }
        if (terminating.signal.aborted) return json({ error: "shutting_down" }, 503);
        if (path.startsWith("/gw/")) {
          // Inference streams have their own bounded lifetime and backpressure.
          server.timeout(req, 0);
          return await gateway.handle(req);
        }
        if (origin && !config.origins.includes(origin)) return json({ error: "origin_forbidden" }, 403);
        if (req.method === "OPTIONS") return cors(new Response(null, { status: 204 }));
        if (path === "/ws" && req.method === "GET") {
          // Browser terminal only. Requiring Origin also prevents a missing
          // browser Origin from silently bypassing the allowlist.
          if (!origin || !config.origins.includes(origin)) return json({ error: "origin_forbidden" }, 403);
          const id = url.searchParams.get("sessionId");
          const session = id ? sessions.get(id) : undefined;
          if (!session || Date.now() >= session.expiresAt || session.status === "error") return json({ error: "session_unavailable" }, 404);
          if (server.upgrade(req, { data: { sessionId: session.id } })) return;
          return json({ error: "upgrade_failed" }, 400);
        }
        if (path === "/api/demos" && req.method === "GET") return cors(json(registry.list().map(demoToMeta)));
        const demoMatch = path.match(/^\/api\/demos\/([a-z0-9-]+)$/);
        if (demoMatch && req.method === "GET") {
          const demo = registry.get(demoMatch[1]!);
          return cors(demo ? json(demoToMeta(demo)) : json({ error: "demo_not_found" }, 404));
        }
        if (path === "/api/sessions" && req.method === "POST") {
          try { await ready(); } catch { return cors(json({ error: "admission_unavailable" }, 503)); }
          const body = JSON.parse(await readBody(req, 4096, AbortSignal.any([req.signal, terminating.signal, AbortSignal.timeout(10000)])));
          if (!body || typeof body.slug !== "string") throw new BodyError(400);
          const demo = registry.get(body.slug);
          if (!demo) return cors(json({ error: "demo_not_found" }, 404));
          const identity = requestIdentity(req, server.requestIP(req)?.address, config);
          if (!identity.ip) return cors(json({ error: "client_identity_unavailable" }, 503));
          sessions.assertAdmission(identity.ip);
          if (identity.tier === "anonymous" && !await limits.claimTrial(identity.ip)) {
            return cors(json({ error: "free_trial_used", trialUsed: true, message: "Sign in to continue with your own credentials." }, 403));
          }
          // create rechecks drain/capacity after the durable admission await.
          return cors(json(info(await sessions.create(body.slug, demo, identity.ip, identity.tier))));
        }
        const match = path.match(/^\/api\/sessions\/([a-f0-9-]+)$/);
        if (match) {
          const session = sessions.get(match[1]!);
          if (!session) return cors(json({ error: "session_not_found" }, 404));
          if (req.method === "GET") return cors(json(info(session)));
          if (req.method === "DELETE") { await sessions.destroy(session.id); return cors(json({ ok: true })); }
          if (req.method === "POST" && url.searchParams.get("action") === "reset") {
            try { await ready(); } catch { return cors(json({ error: "admission_unavailable" }, 503)); }
            return cors(json(info(await sessions.reset(session))));
          }
        }
        const badge = path.match(/^\/badge\/([a-z0-9-]+)\.svg$/);
        if (badge && req.method === "GET") {
          const demo = registry.get(badge[1]!);
          return new Response(badgeSvg(demo ? `Try ${demo.name}` : "Try in Terminal"), { headers: { "content-type": "image/svg+xml", "cache-control": "public, max-age=3600" } });
        }
        return cors(json({ error: "not_found" }, 404));
      } catch (err) {
        if (err instanceof BodyError) return cors(json({ error: err.message }, err.status));
        if (err instanceof SyntaxError) return cors(json({ error: "invalid_json" }, 400));
        if (err instanceof AdmissionError) return cors(json({ error: err.message }, ["draining", "cleanup_required"].includes(err.message) ? 503 : 429));
        // External errors may contain keys, command output or sandbox IDs.
        return cors(json({ error: "dependency_unavailable" }, 503));
      }
    },
    websocket: {
      idleTimeout: 60, sendPings: true, maxPayloadLength: 65536,
      open(ws) {
        const session = sessions.get(ws.data.sessionId);
        if (!session) { ws.close(4004, "Session unavailable"); return; }
        void sessions.attachPty(session, ws).catch(() => {
          ws.close(1011, "Terminal unavailable");
          void sessions.destroy(session.id).catch(() => {});
        });
      },
      message(ws, message) {
        const session = sessions.get(ws.data.sessionId);
        if (session) sessions.sendInput(session, ws, typeof message === "string" ? new TextEncoder().encode(message) : new Uint8Array(message));
      },
      close(ws) {
        const session = sessions.get(ws.data.sessionId);
        if (session) sessions.detachPty(session, ws);
      },
    },
    error() { return json({ error: "internal_error" }, 500); },
  });
  const drain = () => { draining = true; sessions.drain(); };
  let shutdown: Promise<void> | undefined;
  const close = () => shutdown ??= (async () => {
    drain(); terminating.abort();
    // Stop accepting connections; existing HTTP/WS clients finish below.
    void server.stop(false);
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      await Promise.race([
        (async () => {
          const results = await Promise.allSettled([gateway.close(), sessions.close()]);
          if (results.some(r => r.status === "rejected")) throw new Error("Terminal cleanup incomplete");
        })(),
        new Promise<never>((_, reject) => { timer = setTimeout(() => reject(new Error("Terminal shutdown timed out")), config.shutdownMs); }),
      ]);
    } finally {
      clearTimeout(timer);
      await server.stop(true);
      limits.close();
    }
  })();
  return { server, ready, drain, close };
}
