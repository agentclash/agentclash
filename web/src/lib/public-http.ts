export const NEGOTIATED_MARKDOWN_HEADER = "x-agentclash-negotiated-markdown";
export const CANONICAL_PATH_HEADER = "x-agentclash-canonical-path";

export function withoutInternalNegotiationHeaders(headers: Headers): Headers {
  const sanitized = new Headers(headers);
  sanitized.delete(NEGOTIATED_MARKDOWN_HEADER);
  sanitized.delete(CANONICAL_PATH_HEADER);
  return sanitized;
}

const PUBLIC_EXACT_PATHS = new Set([
  "/",
  "/agent-evals",
  "/agent-evaluation-framework",
  "/agent-opportunity",
  "/agent-reliability-benchmark",
  "/agent-trajectory-evaluation",
  "/agentic-self-instruct",
  "/ai-agent-benchmark",
  "/ai-agent-testing",
  "/benchmarks",
  "/blog",
  "/changelog",
  "/ci-cd-agent-evaluation",
  "/compare",
  "/docs",
  "/enterprise",
  "/features",
  "/glossary",
  "/industries",
  "/llm-agent-evaluation",
  "/open-source-ai-agent-evaluation",
  "/platform/agent-evaluation",
  "/platform/agent-regression-testing",
  "/platform/datasmith",
  "/pricing",
  "/publications",
  "/resources/eval-checklist",
  "/services",
  "/synthetic-data-generation-agents",
  "/team",
  "/trace-to-dataset",
  "/try",
  "/tryouts",
  "/vibe-evals",
  "/use-cases",
  "/why",
]);

const PUBLIC_ROUTE_PREFIXES = [
  "/benchmarks/",
  "/blog/",
  "/changelog/",
  "/compare/",
  "/docs/",
  "/features/",
  "/glossary/",
  "/industries/",
  "/publications/",
  "/try/",
  "/use-cases/",
] as const;

const PRIVATE_PREFIXES = [
  "/_next",
  "/api",
  "/auth",
  "/dashboard",
  "/github",
  "/ingest",
  "/invites",
  "/onboard",
  "/orgs",
  "/share",
  "/workspaces",
] as const;

const MACHINE_PATH_PREFIXES = [
  "/cli-schema.json",
  "/docs-md",
  "/llms-full.txt",
  "/llms.txt",
  "/md",
  "/openapi.yaml",
  "/publications/sitemap.xml",
  "/schemas",
] as const;

const AI_USER_AGENTS = [
  ["gptbot", "GPTBot"],
  ["oai-searchbot", "OAI-SearchBot"],
  ["chatgpt-user", "ChatGPT-User"],
  ["claudebot", "ClaudeBot"],
  ["claude-user", "Claude-User"],
  ["claude-searchbot", "Claude-SearchBot"],
  ["perplexitybot", "PerplexityBot"],
  ["perplexity-user", "Perplexity-User"],
  ["google-extended", "Google-Extended"],
  ["ccbot", "CCBot"],
  ["bytespider", "Bytespider"],
  ["meta-externalagent", "meta-externalagent"],
  ["applebot-extended", "Applebot-Extended"],
  ["amazonbot", "Amazonbot"],
] as const;

type MediaPreference = {
  markdown: number | null;
  html: number | null;
};

function parseQuality(parameters: string[]): number {
  const qParameter = parameters.find((parameter) => parameter.trim().toLowerCase().startsWith("q="));
  if (!qParameter) return 1;
  const parsed = Number(qParameter.split("=", 2)[1]);
  if (!Number.isFinite(parsed) || parsed < 0 || parsed > 1) return 0;
  return parsed;
}

export function parseRepresentationPreference(accept: string | null): MediaPreference {
  const preference: MediaPreference = { markdown: null, html: null };
  if (!accept) return preference;

  for (const range of accept.split(",")) {
    const [rawType, ...parameters] = range.split(";");
    const mediaType = rawType.trim().toLowerCase();
    if (mediaType !== "text/markdown" && mediaType !== "text/html") continue;
    const quality = parseQuality(parameters);
    if (mediaType === "text/markdown") {
      preference.markdown = Math.max(preference.markdown ?? 0, quality);
    } else {
      preference.html = Math.max(preference.html ?? 0, quality);
    }
  }
  return preference;
}

export function prefersMarkdown(accept: string | null): boolean {
  const { markdown, html } = parseRepresentationPreference(accept);
  if (markdown === null || markdown <= 0) return false;
  return html === null || markdown > html;
}

function matchesPrefix(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

export function isPrivateOrMachinePath(pathname: string): boolean {
  return (
    PRIVATE_PREFIXES.some((prefix) => matchesPrefix(pathname, prefix)) ||
    MACHINE_PATH_PREFIXES.some((prefix) => matchesPrefix(pathname, prefix))
  );
}

export function isMarkdownNegotiablePath(pathname: string): boolean {
  if (isPrivateOrMachinePath(pathname)) return false;
  return (
    PUBLIC_EXACT_PATHS.has(pathname) ||
    PUBLIC_ROUTE_PREFIXES.some((prefix) => pathname.startsWith(prefix))
  );
}

/**
 * Public HTML pages call `withAuth()` (homepage + MarketingHeader). AuthKit
 * middleware must run on those requests or Server Components throw.
 * Machine/markdown endpoints never call `withAuth` and may skip it.
 */
export function requiresAuthkitMiddleware(pathname: string): boolean {
  return !MACHINE_PATH_PREFIXES.some((prefix) => matchesPrefix(pathname, prefix));
}

export function shouldNegotiateMarkdown(args: {
  enabled: boolean;
  method: string;
  pathname: string;
  accept: string | null;
}): boolean {
  return (
    args.enabled &&
    (args.method === "GET" || args.method === "HEAD") &&
    isMarkdownNegotiablePath(args.pathname) &&
    prefersMarkdown(args.accept)
  );
}

export function markdownPathForCanonical(pathname: string): string {
  return pathname === "/" ? "/md" : `/md${pathname}`;
}

export function representationLinkHeader(pathname: string, origin: string): string {
  const canonical = pathname === "/" ? origin : `${origin}${pathname}`;
  const markdown = `${origin}${markdownPathForCanonical(pathname)}`;
  return `<${canonical}>; rel="canonical", <${markdown}>; rel="alternate"; type="text/markdown"`;
}

export function classifyAgentUserAgent(userAgent: string | null): string | null {
  if (!userAgent) return null;
  const normalized = userAgent.toLowerCase();
  return AI_USER_AGENTS.find(([needle]) => normalized.includes(needle))?.[1] ?? null;
}

export function classifyAccept(accept: string | null): "markdown" | "html" | "generic" | "missing" {
  if (!accept) return "missing";
  if (accept.toLowerCase().includes("text/markdown")) return "markdown";
  if (accept.toLowerCase().includes("text/html")) return "html";
  return "generic";
}

export function normalizeLoggedPath(pathname: string): string {
  if (pathname.startsWith("/share/")) return "/share/{token}";
  return pathname.split("?", 1)[0] || "/";
}

export function classifyRouteKind(pathname: string): "publication" | "markdown" | "contract" | "public_content" {
  if (
    pathname === "/md" ||
    pathname.startsWith("/md/") ||
    pathname === "/docs-md" ||
    pathname.startsWith("/docs-md/")
  ) {
    return "markdown";
  }
  if (MACHINE_PATH_PREFIXES.some((prefix) => matchesPrefix(pathname, prefix))) return "contract";
  if (pathname === "/publications" || pathname.startsWith("/publications/")) return "publication";
  return "public_content";
}

export function requestedRepresentation(
  pathname: string,
  accept: string | null,
): "html" | "markdown" | "machine" {
  const routeKind = classifyRouteKind(pathname);
  if (routeKind === "markdown") return "markdown";
  if (routeKind === "contract") return "machine";
  return prefersMarkdown(accept) ? "markdown" : "html";
}

export function shouldLogAgentRequest(args: {
  pathname: string;
  accept: string | null;
  userAgent: string | null;
}): boolean {
  return Boolean(
    classifyAgentUserAgent(args.userAgent) ||
      classifyAccept(args.accept) === "markdown" ||
      MACHINE_PATH_PREFIXES.some((prefix) => matchesPrefix(args.pathname, prefix)) ||
      matchesPrefix(args.pathname, "/publications"),
  );
}

export function agentRequestLog(args: {
  pathname: string;
  method: string;
  accept: string | null;
  userAgent: string | null;
  requestId: string | null;
  servedRepresentation: "html" | "markdown" | "machine";
}) {
  return {
    level: "info",
    event: "agent_readable_request",
    path: normalizeLoggedPath(args.pathname),
    method: args.method,
    agent_family: classifyAgentUserAgent(args.userAgent) ?? "unclassified",
    accept_class: classifyAccept(args.accept),
    requested_representation: requestedRepresentation(args.pathname, args.accept),
    served_representation: args.servedRepresentation,
    route_kind: classifyRouteKind(args.pathname),
    request_id: args.requestId,
  } as const;
}
