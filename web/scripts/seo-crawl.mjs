#!/usr/bin/env node
// Anonymous production crawl for SEO invariants.
// Usage: node scripts/seo-crawl.mjs [baseUrl]
// Expects a running server (e.g. `next start`) at baseUrl. Checks every
// sitemap URL for a direct 200, canonical agreement, and indexability, then
// verifies key entrypoints are reachable from the homepage within 3 clicks
// using only links present in the initial HTML.
const base = (process.argv[2] || "http://127.0.0.1:3000").replace(/\/$/, "");
const KEY_ENTRYPOINTS = [
  "/features",
  "/use-cases",
  "/industries",
  "/glossary",
  "/benchmarks",
  "/benchmarks/methodology",
  "/security",
  "/publications",
  "/pricing",
  "/docs",
];

const failures = [];
const fail = (url, message) => failures.push(`${url}: ${message}`);

async function fetchManual(url) {
  return fetch(url, { redirect: "manual", headers: { "user-agent": "seo-crawl" } });
}

function attr(html, pattern) {
  const match = html.match(pattern);
  return match ? match[1] : null;
}

function internalLinks(html, pageUrl) {
  const links = new Set();
  for (const match of html.matchAll(/<a[^>]+href="([^"]+)"/g)) {
    const href = match[1];
    if (href.startsWith("/") && !href.startsWith("//")) {
      links.add(href.split("#")[0].split("?")[0] || "/");
    } else if (href.startsWith(base)) {
      links.add(new URL(href).pathname);
    } else if (href.startsWith("https://www.agentclash.dev")) {
      links.add(new URL(href).pathname);
    }
  }
  void pageUrl;
  return [...links];
}

const sitemapRes = await fetchManual(`${base}/sitemap.xml`);
if (sitemapRes.status !== 200) {
  console.error(`sitemap.xml returned ${sitemapRes.status}`);
  process.exit(1);
}
const sitemapXml = await sitemapRes.text();
const locs = [...sitemapXml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]);
const paths = locs.map((loc) => {
  try {
    return new URL(loc).pathname;
  } catch {
    fail(loc, "unparseable sitemap URL");
    return null;
  }
}).filter(Boolean);
if (new Set(paths).size !== paths.length) fail("sitemap", "duplicate URLs");

const linkGraph = new Map();
for (const pathname of paths) {
  const url = `${base}${pathname}`;
  const res = await fetchManual(url);
  if (res.status >= 300 && res.status < 400) {
    fail(pathname, `redirects (${res.status}) instead of serving 200 directly`);
    continue;
  }
  if (res.status !== 200) {
    fail(pathname, `expected 200, got ${res.status}`);
    continue;
  }
  const html = await res.text();
  const canonical = attr(html, /<link[^>]+rel="canonical"[^>]+href="([^"]+)"/) ||
    attr(html, /<link[^>]+href="([^"]+)"[^>]+rel="canonical"/);
  if (canonical) {
    const canonicalPath = canonical.startsWith("http") ? new URL(canonical).pathname : canonical;
    const expected = pathname === "/" ? "/" : pathname;
    if (canonicalPath !== expected && canonical !== "https://www.agentclash.dev" && !(expected === "/" && canonicalPath === "/")) {
      fail(pathname, `canonical ${canonical} does not match ${expected}`);
    }
  } else {
    fail(pathname, "missing canonical link");
  }
  if (/noindex/i.test(attr(html, /<meta[^>]+name="robots"[^>]+content="([^"]+)"/) || "") ||
      /noindex/i.test(res.headers.get("x-robots-tag") || "")) {
    fail(pathname, "sitemap URL is noindexed");
  }
  if (!/<title>[^<]+<\/title>/.test(html)) fail(pathname, "missing title");
  if (!/<h1[\s>]/.test(html)) fail(pathname, "missing h1");
  linkGraph.set(pathname, internalLinks(html, url));
}

// Internal destinations linked from crawled pages must resolve.
for (const [from, links] of linkGraph) {
  for (const href of links) {
    if (href.startsWith("/api/") || href.startsWith("/auth/") || href.startsWith("/_next/")) continue;
    if (!linkGraph.has(href) && !paths.includes(href)) {
      const res = await fetchManual(`${base}${href}`);
      if (res.status !== 200) fail(from, `links to ${href} which returned ${res.status}`);
    }
  }
}

// Homepage reachability within 3 clicks over initial-HTML links.
const depth = new Map([["/", 0]]);
const queue = ["/"];
while (queue.length) {
  const current = queue.shift();
  const d = depth.get(current);
  if (d >= 3) continue;
  for (const next of linkGraph.get(current) || []) {
    if (!depth.has(next)) {
      depth.set(next, d + 1);
      queue.push(next);
    }
  }
}
for (const entry of KEY_ENTRYPOINTS) {
  if (!depth.has(entry)) fail(entry, "not reachable from homepage within 3 clicks");
  else if (depth.get(entry) > 3) fail(entry, `reachable only at depth ${depth.get(entry)}`);
}

console.log(`Crawled ${paths.length} sitemap URLs from ${base}`);
if (failures.length) {
  console.error(`\n${failures.length} failure(s):`);
  for (const line of failures.slice(0, 50)) console.error(`- ${line}`);
  process.exit(1);
}
console.log("SEO crawl passed: 200s, canonicals, indexability, internal links, homepage depth.");
