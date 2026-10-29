import { describe, expect, it } from "vitest";
import { AI_CRAWLERS } from "@/app/robots";
import {
  agentRequestLog,
  classifyAgentUserAgent,
  classifyRouteKind,
  isMarkdownNegotiablePath,
  parseRepresentationPreference,
  requiresAuthkitMiddleware,
  prefersMarkdown,
  representationLinkHeader,
  requestedRepresentation,
  shouldLogAgentRequest,
  shouldNegotiateMarkdown,
  withoutInternalNegotiationHeaders,
} from "./public-http";

describe("public representation negotiation", () => {
  it.each([
    [null, false],
    ["*/*", false],
    ["text/*", false],
    ["text/html", false],
    ["text/markdown", true],
    ["text/markdown;q=0", false],
    ["text/html, text/markdown", false],
    ["text/html;q=0.4, text/markdown;q=0.8", true],
    ["text/html;q=0.8, text/markdown;q=0.4", false],
    ["text/markdown;q=bogus", false],
    ["application/json, text/markdown;q=0.6", true],
  ])("handles Accept %s", (accept, expected) => {
    expect(prefersMarkdown(accept)).toBe(expected);
  });

  it("keeps the highest exact quality for duplicate ranges", () => {
    expect(
      parseRepresentationPreference(
        "text/markdown;q=0.2, text/markdown;q=0.9, text/html;q=0.5",
      ),
    ).toEqual({ markdown: 0.9, html: 0.5 });
  });

  it("negotiates public GET and HEAD only", () => {
    const base = {
      enabled: true,
      pathname: "/pricing",
      accept: "text/markdown",
    };
    expect(shouldNegotiateMarkdown({ ...base, method: "GET" })).toBe(true);
    expect(shouldNegotiateMarkdown({ ...base, method: "HEAD" })).toBe(true);
    expect(shouldNegotiateMarkdown({ ...base, method: "POST" })).toBe(false);
    expect(shouldNegotiateMarkdown({ ...base, enabled: false, method: "GET" })).toBe(false);
  });

  it.each([
    "/workspaces/ws_123",
    "/orgs/example",
    "/auth/login",
    "/api/indexnow",
    "/ingest/capture",
    "/share/capability-token",
    "/md/pricing",
    "/docs-md/getting-started/quickstart",
    "/openapi.yaml",
  ])("excludes %s", (pathname) => {
    expect(isMarkdownNegotiablePath(pathname)).toBe(false);
  });

  it.each([
    "/",
    "/pricing",
    "/security",
    "/benchmarks/methodology",
    "/docs/getting-started/quickstart",
    "/blog/example",
    "/compare/agentclash-vs-langsmith",
    "/publications/pub_123",
  ])("allows %s", (pathname) => {
    expect(isMarkdownNegotiablePath(pathname)).toBe(true);
  });

  it.each([
    "/",
    "/docs",
    "/docs/getting-started/first-eval",
    "/blog",
    "/compare",
    "/publications",
    "/pricing",
    "/dashboard",
    "/workspaces/ws_123",
  ])("requires AuthKit on HTML path %s", (pathname) => {
    expect(requiresAuthkitMiddleware(pathname)).toBe(true);
  });

  it.each([
    "/md",
    "/md/pricing",
    "/docs-md/getting-started/quickstart",
    "/llms.txt",
    "/openapi.yaml",
  ])("allows AuthKit bypass on machine path %s", (pathname) => {
    expect(requiresAuthkitMiddleware(pathname)).toBe(false);
  });

  it("advertises canonical and Markdown URLs", () => {
    expect(
      representationLinkHeader("/pricing", "https://www.agentclash.dev"),
    ).toBe(
      '<https://www.agentclash.dev/pricing>; rel="canonical", <https://www.agentclash.dev/md/pricing>; rel="alternate"; type="text/markdown"',
    );
  });

  it("strips client-spoofable negotiation-only headers", () => {
    const sanitized = withoutInternalNegotiationHeaders(
      new Headers({
        Accept: "text/markdown",
        "X-AgentClash-Negotiated-Markdown": "1",
        "X-AgentClash-Canonical-Path": "/pricing",
      }),
    );
    expect(sanitized.get("accept")).toBe("text/markdown");
    expect(sanitized.has("x-agentclash-negotiated-markdown")).toBe(false);
    expect(sanitized.has("x-agentclash-canonical-path")).toBe(false);
  });
});

describe("agent request logging", () => {
  it("classifies named AI crawler families", () => {
    expect(classifyAgentUserAgent("Mozilla/5.0 compatible; GPTBot/1.2")).toBe(
      "GPTBot",
    );
    expect(classifyAgentUserAgent("Claude-SearchBot/1.0")).toBe(
      "Claude-SearchBot",
    );
    expect(classifyAgentUserAgent("Mozilla/5.0")).toBeNull();
  });

  it("classifies every crawler explicitly welcomed by robots", () => {
    for (const crawler of AI_CRAWLERS) {
      expect(classifyAgentUserAgent(`${crawler}/1.0`), crawler).toBe(crawler);
    }
  });

  it("builds an allowlisted payload without URL or header secrets", () => {
    const log = agentRequestLog({
      pathname: "/docs",
      method: "GET",
      accept: "text/markdown",
      userAgent: "GPTBot",
      requestId: "iad1::abc",
      servedRepresentation: "markdown",
    });
    expect(log).toEqual({
      level: "info",
      event: "agent_readable_request",
      path: "/docs",
      method: "GET",
      agent_family: "GPTBot",
      accept_class: "markdown",
      requested_representation: "markdown",
      served_representation: "markdown",
      route_kind: "public_content",
      request_id: "iad1::abc",
    });
    expect(JSON.stringify(log)).not.toContain("authorization");
    expect(JSON.stringify(log)).not.toContain("cookie");
    expect(JSON.stringify(log)).not.toContain("?");
  });

  it("redacts capability tokens from logged share paths", () => {
    const log = agentRequestLog({
      pathname: "/share/secret-capability-token",
      method: "GET",
      accept: "text/html",
      userAgent: "GPTBot",
      requestId: null,
      servedRepresentation: "html",
    });
    expect(log.path).toBe("/share/{token}");
    expect(JSON.stringify(log)).not.toContain("secret-capability-token");
  });

  it("separates publication pages from the publication sitemap contract", () => {
    expect(classifyRouteKind("/publications/123")).toBe("publication");
    expect(classifyRouteKind("/publications/sitemap.xml")).toBe("contract");
    expect(requestedRepresentation("/md/pricing", "*/*")).toBe("markdown");
    expect(requestedRepresentation("/docs-md", "*/*")).toBe("markdown");
    expect(requestedRepresentation("/openapi.yaml", "*/*")).toBe("machine");
  });

  it("targets guessed paths from known agents for 404 analysis", () => {
    expect(
      shouldLogAgentRequest({
        pathname: "/guessed-agent-docs",
        accept: "*/*",
        userAgent: "PerplexityBot/1.0",
      }),
    ).toBe(true);
  });

  it("logs explicit Markdown ranges even when their quality disables negotiation", () => {
    expect(
      shouldLogAgentRequest({
        pathname: "/pricing",
        accept: "text/html, text/markdown;q=0",
        userAgent: "Mozilla/5.0",
      }),
    ).toBe(true);
  });
});
