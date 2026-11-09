#!/usr/bin/env node
import { JSDOM } from "jsdom";
import { pathToFileURL } from "node:url";

const PUBLIC_ORIGIN = "https://www.agentclash.dev";
const KEY_ENTRYPOINTS = [
  "/features", "/use-cases", "/industries", "/glossary", "/benchmarks",
  "/benchmarks/methodology", "/security", "/publications", "/pricing", "/docs",
  "/platform/agent-evaluation", "/platform/datasmith", "/agent-opportunity", "/tryouts", "/try",
];
// These are auth/private apps or machine representations, never discovery nodes.
const EXCLUDED_PREFIXES = [
  "/api", "/auth", "/dashboard", "/github", "/ingest", "/invites", "/onboard",
  "/orgs", "/share", "/workspaces", "/_next", "/md", "/docs-md", "/schemas", "/vibe-evals",
];
const EXCLUDED_PATHS = ["/resources/eval-checklist/thank-you"];

/** Crawl the same initial HTML and HTTP responses available to an anonymous crawler. */
export async function runSeoCrawl({
  baseUrl = "http://127.0.0.1:3000", canonicalOrigin = PUBLIC_ORIGIN,
  keyEntrypoints = KEY_ENTRYPOINTS, samplePaths = [],
} = {}) {
  const base = new URL(baseUrl).origin;
  const origin = new URL(canonicalOrigin).origin;
  const failures = [];
  const fail = (url, message) => failures.push(`${url}: ${message}`);
  const request = (pathname) => fetch(`${base}${pathname}`, {
    redirect: "manual", headers: { "user-agent": "seo-crawl" }, signal: AbortSignal.timeout(30_000),
  });
  const excluded = (url) => url.search ||
    EXCLUDED_PREFIXES.some((prefix) => url.pathname === prefix || url.pathname.startsWith(`${prefix}/`)) ||
    [...EXCLUDED_PATHS, ...samplePaths].includes(url.pathname) ||
    /\.(?:xml|ya?ml|json|txt|md|pdf|png|jpe?g|svg|ico|webp|zip|gz)$/i.test(url.pathname);

  async function sitemap(pathname, allowEmpty) {
    try {
      const response = await request(pathname);
      if (response.status !== 200) throw new Error(`expected 200, got ${response.status}`);
      const dom = new JSDOM(await response.text(), { contentType: "application/xml" });
      try {
        const root = dom.window.document.documentElement;
        if (root.localName !== "urlset" || root.namespaceURI !== "http://www.sitemaps.org/schemas/sitemap/0.9") throw new Error("invalid sitemap urlset");
        const paths = [];
        for (const entry of root.children) {
          if (entry.localName !== "url" || entry.namespaceURI !== root.namespaceURI) continue;
          const locations = [...entry.children].filter((node) => node.localName === "loc" && node.namespaceURI === root.namespaceURI);
          if (locations.length !== 1 || !locations[0].textContent.trim()) throw new Error("missing or empty sitemap location");
          const url = new URL(locations[0].textContent.trim());
          if (url.origin !== origin || url.search || url.hash || url.username || url.password || excluded(url)) throw new Error(`invalid sitemap location ${url.href}`);
          paths.push(url.pathname);
        }
        if (!allowEmpty && !paths.length) throw new Error("empty sitemap");
        if (new Set(paths).size !== paths.length) throw new Error("duplicate sitemap URLs");
        return paths;
      } finally { dom.window.close(); }
    } catch (error) { fail(pathname, error.message); return []; }
  }

  const paths = await sitemap("/sitemap.xml", false);
  const publicationPaths = await sitemap("/publications/sitemap.xml", true);
  const graph = new Map();
  const queue = [...new Set(["/", ...paths, ...publicationPaths])];
  const scheduled = new Set(queue);
  for (let index = 0; index < queue.length; index++) {
    const pathname = queue[index];
    try {
      const response = await request(pathname);
      if (response.status !== 200) throw new Error(`expected direct 200, got ${response.status}`);
      if (!/^text\/html(?:;|$)/i.test(response.headers.get("content-type") || "")) throw new Error("expected HTML content type");
      const dom = new JSDOM(await response.text()); // no scripts or subresource loading
      try {
        const document = dom.window.document;
        const canonicals = document.querySelectorAll('link[rel~="canonical"]');
        const expected = new URL(pathname, origin).href;
        if (canonicals.length !== 1) fail(pathname, "expected one canonical link");
        else {
          const raw = canonicals[0].getAttribute("href");
          let canonical;
          try { canonical = new URL(raw).href; } catch { /* relative/malformed canonical */ }
          if (canonical !== expected) fail(pathname, `canonical ${raw} does not match ${expected}`);
        }
        const directives = [response.headers.get("x-robots-tag") || ""];
        for (const meta of document.querySelectorAll("meta[name]")) {
          if (["robots", "googlebot"].includes(meta.getAttribute("name").toLowerCase())) directives.push(meta.getAttribute("content") || "");
        }
        if (directives.some((value) => /\b(?:noindex|none)\b/i.test(value))) fail(pathname, "public URL is noindexed");
        if (!document.querySelector("head > title")?.textContent.trim()) fail(pathname, "missing title");
        if (![...document.querySelectorAll("meta[name]")].some((meta) => meta.getAttribute("name").toLowerCase() === "description" && meta.getAttribute("content")?.trim())) fail(pathname, "missing description");
        if (!document.querySelector("h1")?.textContent.trim()) fail(pathname, "missing h1");
        const links = new Set();
        for (const anchor of document.querySelectorAll("a[href]")) {
          let url;
          try { url = new URL(anchor.getAttribute("href"), `${base}${pathname}`); } catch { continue; }
          if (![base, origin].includes(url.origin) || excluded(url)) continue;
          links.add(url.pathname);
          if (!scheduled.has(url.pathname)) { scheduled.add(url.pathname); queue.push(url.pathname); }
        }
        graph.set(pathname, links);
      } finally { dom.window.close(); }
    } catch (error) { fail(pathname, error.message); }
  }

  const depth = new Map([["/", 0]]);
  const reachable = ["/"];
  for (let index = 0; index < reachable.length; index++) {
    const current = reachable[index];
    for (const next of graph.get(current) || []) {
      if (!depth.has(next)) { depth.set(next, depth.get(current) + 1); reachable.push(next); }
    }
  }
  for (const pathname of paths) if (!depth.has(pathname)) fail(pathname, "not reachable from homepage initial HTML");
  for (const pathname of keyEntrypoints) {
    if (!depth.has(pathname) || depth.get(pathname) > 3) fail(pathname, "not reachable from homepage within 3 clicks");
  }
  return { failures, paths, publicationPaths, depth };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = await runSeoCrawl({ baseUrl: process.argv[2] });
  console.log(`Crawled ${result.paths.length} static and ${result.publicationPaths.length} publication sitemap URLs`);
  for (const failure of result.failures) console.error(failure);
  if (result.failures.length) process.exitCode = 1;
  else console.log("SEO crawl passed: direct 200s, metadata, public links, full reachability and entrypoint depth.");
}
