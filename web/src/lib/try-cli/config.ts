/** Base URL for Try CLI HTTP API (sessions, demos). */
export function getTryCliApiBase(): string {
  if (typeof window !== "undefined") {
    return process.env.NEXT_PUBLIC_TRY_CLI_API_URL ?? "/api/try";
  }
  return (
    process.env.TRY_CLI_API_URL ??
    process.env.NEXT_PUBLIC_TRY_CLI_API_URL ??
    "http://localhost:3001"
  );
}

/** WebSocket base for PTY streams (must be a long-running service, not Vercel serverless). */
export function getTryCliWsBase(): string {
  const ws =
    process.env.NEXT_PUBLIC_TRY_CLI_WS_URL ??
    (typeof window !== "undefined" ? "ws://localhost:3001" : "ws://localhost:3001");
  return ws.replace(/\/$/, "");
}

export function tryCliPublicOrigin(): string {
  return process.env.NEXT_PUBLIC_TRY_CLI_PUBLIC_URL ?? "https://www.agentclash.dev/try";
}

export function tryCliBadgeUrl(slug: string): string {
  const base = getTryCliApiBase();
  const origin =
    base.startsWith("http") ? base : tryCliPublicOrigin().replace(/\/try$/, "");
  return `${origin.replace(/\/api\/try$/, "")}/api/try/badge/${slug}.svg`;
}
