import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { badgeSvg } from "@try-cli/core";
import { registry } from "./registry.ts";
import { sessions } from "./sessions.ts";
import { demoToMeta } from "./types.ts";

const __dirname = dirname(fileURLToPath(import.meta.url));
const PORT = Number(process.env.PORT ?? 3000);
const isProd = process.env.NODE_ENV === "production";
const DIST = join(__dirname, "../dist");

function json(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
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

    if (pathname === "/api/demos") {
      return json(registry.list().map(demoToMeta));
    }

    const demoMatch = pathname.match(/^\/api\/demos\/([a-z0-9-]+)$/);
    if (demoMatch) {
      const demo = registry.get(demoMatch[1]!);
      if (!demo) return json({ error: "Demo not found" }, 404);
      return json(demoToMeta(demo));
    }

    if (pathname === "/api/sessions" && req.method === "POST") {
      try {
        const body = (await req.json()) as { slug?: string };
        const slug = body.slug;
        if (!slug) return json({ error: "slug required" }, 400);
        const demo = registry.get(slug);
        if (!demo) return json({ error: "Demo not found" }, 404);
        const ip = getClientIp(req);
        const session = await sessions.create(slug, demo, ip);
        return json({
          id: session.id,
          slug: session.slug,
          expiresAt: session.expiresAt,
          status: session.status,
          mock: session.mock,
        });
      } catch (err) {
        return json({ error: err instanceof Error ? err.message : String(err) }, 429);
      }
    }

    const sessionMatch = pathname.match(/^\/api\/sessions\/([a-f0-9-]+)$/);
    if (sessionMatch) {
      const session = sessions.get(sessionMatch[1]!);
      if (!session) return json({ error: "Session not found" }, 404);

      if (req.method === "GET") {
        return json({
          id: session.id,
          slug: session.slug,
          expiresAt: session.expiresAt,
          status: session.status,
          error: session.error,
          mock: session.mock,
        });
      }

      if (req.method === "DELETE") {
        await sessions.destroy(session.id);
        return json({ ok: true });
      }

      if (req.method === "POST" && url.searchParams.get("action") === "reset") {
        const newSession = await sessions.reset(session);
        return json({
          id: newSession.id,
          slug: newSession.slug,
          expiresAt: newSession.expiresAt,
          status: newSession.status,
          mock: newSession.mock,
        });
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

console.log(`try-cli server running on http://localhost:${PORT}`);
console.log(`  Demos: ${registry.list().map((d) => d.slug).join(", ")}`);
console.log(`  E2B: ${process.env.E2B_API_KEY ? "enabled" : "mock mode"}`);
console.log(`  Dev frontend: run 'bun run vite' in apps/web for hot reload`);

export { server };
