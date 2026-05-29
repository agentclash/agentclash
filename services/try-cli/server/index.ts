import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { badgeSvg } from "@try-cli/core";
import { registry } from "./registry.ts";
import { sessions } from "./sessions.ts";
import { demoToMeta } from "./types.ts";
import { handleGatewayRequest } from "./gateway.ts";
import { createDailyLedger } from "./daily-ledger.ts";

const __dirname = dirname(fileURLToPath(import.meta.url));
const PORT = Number(process.env.PORT ?? 3001);
const isProd = process.env.NODE_ENV === "production";
const DIST = join(__dirname, "../dist");

const gatewayDeps = { sessions, daily: createDailyLedger() };
// Shared secret the Vercel proxy adds when forwarding an authenticated user, so
// the public service can't be tricked into granting the (BYO) authed tier.
const PROXY_SECRET = process.env.TRY_CLI_PROXY_SECRET;

function sessionTier(req: Request): "anonymous" | "authenticated" {
  const user = req.headers.get("x-agentclash-user");
  if (!user) return "anonymous";
  if (PROXY_SECRET && req.headers.get("x-agentclash-proxy-secret") !== PROXY_SECRET) {
    return "anonymous"; // header present but unsigned — don't trust it
  }
  return "authenticated";
}

const CORS_ORIGINS = (process.env.TRY_CLI_CORS_ORIGINS ?? "http://localhost:3000,https://www.agentclash.dev,https://agentclash.dev,https://try.agentclash.dev")
  .split(",")
  .map((o) => o.trim())
  .filter(Boolean);

function corsHeaders(req: Request): Record<string, string> {
  const origin = req.headers.get("Origin") ?? "";
  if (origin && CORS_ORIGINS.some((o) => o === origin || o === "*")) {
    return {
      "Access-Control-Allow-Origin": origin,
      "Access-Control-Allow-Methods": "GET, POST, DELETE, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
    };
  }
  if (CORS_ORIGINS.includes("*")) {
    return { "Access-Control-Allow-Origin": "*" };
  }
  return {};
}

function json(data: unknown, status = 200, req?: Request) {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      "Content-Type": "application/json",
      ...(req ? corsHeaders(req) : {}),
    },
  });
}

function getClientIp(req: Request): string {
  return req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ?? "local";
}

async function serveStatic(pathname: string): Promise<Response | null> {
  const file = Bun.file(join(DIST, pathname === "/" ? "index.html" : pathname.replace(/^\//, "")));
  if (await file.exists()) {
    const ext = pathname.split(".").pop();
    const types: Record<string, string> = {
      html: "text/html",
      js: "application/javascript",
      css: "text/css",
      svg: "image/svg+xml",
      png: "image/png",
      ico: "image/x-icon",
    };
    return new Response(file, {
      headers: { "Content-Type": types[ext ?? ""] ?? "application/octet-stream" },
    });
  }
  const spa = Bun.file(join(DIST, "index.html"));
  if (await spa.exists()) return new Response(spa, { headers: { "Content-Type": "text/html" } });
  return null;
}

const server = Bun.serve<{ sessionId: string }>({
  port: PORT,
  async fetch(req, server) {
    const url = new URL(req.url);
    const { pathname } = url;

    if (req.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: corsHeaders(req) });
    }

    // Metered LLM gateway for the anonymous free trial. Called by the sandbox
    // CLIs (not the browser), so no CORS handling needed.
    if (pathname.startsWith("/gw/")) {
      return handleGatewayRequest(req, url, gatewayDeps);
    }

    if (pathname.startsWith("/ws")) {
      const sessionId = url.searchParams.get("sessionId");
      if (!sessionId) return new Response("Missing sessionId", { status: 400 });
      const session = sessions.get(sessionId);
      if (!session) return new Response("Session not found", { status: 404 });
      if (session.status === "expired") return new Response("Session expired", { status: 410 });

      const upgraded = server.upgrade(req, { data: { sessionId } });
      if (!upgraded) return new Response("WebSocket upgrade failed", { status: 500 });
      return undefined as unknown as Response;
    }

    if (pathname === "/api/demos" || pathname === "/health") {
      if (pathname === "/health") return json({ ok: true }, 200, req);
      return json(registry.list().map(demoToMeta), 200, req);
    }

    const demoMatch = pathname.match(/^\/api\/demos\/([a-z0-9-]+)$/);
    if (demoMatch) {
      const demo = registry.get(demoMatch[1]!);
      if (!demo) return json({ error: "Demo not found" }, 404, req);
      return json(demoToMeta(demo), 200, req);
    }

    if (pathname === "/api/sessions" && req.method === "POST") {
      try {
        const body = (await req.json()) as { slug?: string };
        const slug = body.slug;
        if (!slug) return json({ error: "slug required" }, 400, req);
        const demo = registry.get(slug);
        if (!demo) return json({ error: "Demo not found" }, 404, req);
        const ip = getClientIp(req);
        const session = await sessions.create(slug, demo, ip, sessionTier(req));
        return json({
          id: session.id,
          slug: session.slug,
          expiresAt: session.expiresAt,
          status: session.status,
          mock: session.mock,
          tier: session.tier,
        }, 200, req);
      } catch (err) {
        return json({ error: err instanceof Error ? err.message : String(err) }, 429, req);
      }
    }

    const sessionMatch = pathname.match(/^\/api\/sessions\/([a-f0-9-]+)$/);
    if (sessionMatch) {
      const session = sessions.get(sessionMatch[1]!);
      if (!session) return json({ error: "Session not found" }, 404, req);

      if (req.method === "GET") {
        return json({
          id: session.id,
          slug: session.slug,
          expiresAt: session.expiresAt,
          status: session.status,
          error: session.error,
          mock: session.mock,
          tier: session.tier,
          trial: session.trialWired,
          budgetUsd: session.gatewayBudgetUsd,
          spentUsd: Math.round(session.gatewaySpentUsd * 1000) / 1000,
        }, 200, req);
      }

      if (req.method === "DELETE") {
        await sessions.destroy(session.id);
        return json({ ok: true }, 200, req);
      }

      if (req.method === "POST" && url.searchParams.get("action") === "reset") {
        const newSession = await sessions.reset(session);
        return json({
          id: newSession.id,
          slug: newSession.slug,
          expiresAt: newSession.expiresAt,
          status: newSession.status,
          mock: newSession.mock,
          tier: newSession.tier,
        }, 200, req);
      }
    }

    const badgeMatch = pathname.match(/^\/badge\/([a-z0-9-]+)\.svg$/);
    if (badgeMatch) {
      const demo = registry.get(badgeMatch[1]!);
      const label = demo ? `Try ${demo.name}` : "Try in Terminal";
      return new Response(badgeSvg(label), {
        headers: {
          "Content-Type": "image/svg+xml",
          "Cache-Control": "public, max-age=3600",
        },
      });
    }

    if (isProd) {
      const staticRes = await serveStatic(pathname);
      if (staticRes) return staticRes;
    }

    return new Response("Not found", { status: 404 });
  },
  websocket: {
    open(ws) {
      const session = sessions.get(ws.data.sessionId);
      if (!session) {
        ws.close(4004, "Session not found");
        return;
      }
      sessions.attachPty(session, ws).catch((err) => {
        console.error("PTY attach failed:", err);
        ws.close(1011, "PTY attach failed");
      });
    },
    message(ws, message) {
      const session = sessions.get(ws.data.sessionId);
      if (!session) return;
      const data = typeof message === "string" ? new TextEncoder().encode(message) : new Uint8Array(message);
      sessions.sendInput(session, ws, data);
    },
    close(ws) {
      const session = sessions.get(ws.data.sessionId);
      if (session) session.ws = null;
    },
  },
});

console.log(`AgentClash Try CLI service on http://localhost:${PORT}`);
console.log(`  Demos: ${registry.list().map((d) => d.slug).join(", ")}`);
console.log(`  E2B: ${process.env.E2B_API_KEY ? "enabled" : "mock mode"}`);
console.log(`  Dev frontend: run 'pnpm dev' in web/ (Next.js) for hot reload`);

export { server };
