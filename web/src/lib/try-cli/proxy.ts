import { isIP } from "node:net";

interface ProxyOptions {
  serviceUrl?: string;
  secret?: string;
  production: boolean;
  vercel: boolean;
  userId(): Promise<string | undefined>;
  fetch?: (url: string, init: RequestInit) => Promise<Response>;
}

/** Server-only HTTP proxy; browser WebSockets connect to the terminal host. */
export function createTryCliProxy(options: ProxyOptions) {
  return async (req: Request, segments: string[]): Promise<Response> => {
    try {
      const service = new URL(options.serviceUrl ?? "http://localhost:3001");
      if (service.username || service.password || service.search || service.hash || service.pathname !== "/" ||
        !["http:", "https:"].includes(service.protocol) || segments.some(s => !/^[a-z0-9-]+$/.test(s)) ||
        (options.production && (!options.serviceUrl || service.protocol !== "https:" || !options.vercel || !options.secret || Buffer.byteLength(options.secret) < 32))) {
        return unavailable();
      }
      // Vercel overwrites this platform header. Never infer trust from a header
      // on an arbitrary self-hosted Next.js server or forward a client XFF list.
      const ip = options.vercel ? req.headers.get("x-real-ip") : "127.0.0.1";
      if (!ip || !isIP(ip)) return unavailable();
      const headers = new Headers();
      const contentType = req.headers.get("content-type");
      if (contentType) headers.set("content-type", contentType);
      if (options.secret) {
        headers.set("x-agentclash-proxy-secret", options.secret);
        headers.set("x-agentclash-client-ip", ip);
        const user = await options.userId().catch(() => undefined);
        if (user) headers.set("x-agentclash-user", user);
      }
      const signal = AbortSignal.any([req.signal, AbortSignal.timeout(15000)]);
      let body: string | undefined;
      if (req.method !== "GET" && req.method !== "HEAD") {
        if (Number(req.headers.get("content-length")) > 4096) return Response.json({ error: "request_too_large" }, { status: 413 });
        const reader = req.body?.getReader();
        const chunks: Uint8Array[] = [];
        let size = 0;
        const cancel = () => { void reader?.cancel().catch(() => {}); };
        signal.addEventListener("abort", cancel, { once: true });
        try {
          while (reader) {
            signal.throwIfAborted();
            const { value, done } = await reader.read();
            if (done) break;
            size += value.byteLength;
            if (size > 4096) { cancel(); return Response.json({ error: "request_too_large" }, { status: 413 }); }
            chunks.push(value);
          }
          body = Buffer.concat(chunks).toString("utf8");
        } finally { signal.removeEventListener("abort", cancel); reader?.releaseLock(); }
      }
      signal.throwIfAborted();
      const url = new URL(req.url);
      const response = await (options.fetch ?? fetch)(`${service.origin}/api/${segments.join("/")}${url.search}`, {
        method: req.method, headers, body, cache: "no-store", signal, redirect: "error",
      });
      const responseHeaders = new Headers({ "cache-control": "no-store" });
      for (const name of ["content-type", "retry-after"]) {
        const value = response.headers.get(name);
        if (value) responseHeaders.set(name, value);
      }
      return new Response(response.body, { status: response.status, headers: responseHeaders });
    } catch { return unavailable(); }
  };
}

function unavailable() {
  return Response.json({ error: "terminal_proxy_unavailable" }, { status: 503, headers: { "cache-control": "no-store" } });
}
