// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { runSeoCrawl } from "../../scripts/seo-crawl.mjs";

const origin = "https://www.agentclash.dev";
const baseUrl = "http://127.0.0.1:3000";
const sitemap = (paths: string[]) => `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">${paths.map((path) => `<url><loc>${origin}${path}</loc><image:image><image:loc>https://images.example/asset.png</image:loc></image:image></url>`).join("")}</urlset>`;
const html = (path: string, links: string[] = []) => `<html><head><title>Page</title><meta name="description" content="Description"><link rel="canonical" href="${origin}${path}"></head><body><h1>Page</h1>${links.map((href) => `<a href='${href}'>Next</a>`).join("")}</body></html>`;
function fixture() {
  const responses = new Map([
    ["/sitemap.xml", { body: sitemap(["/", "/child", "/leaf", "/deep", "/last"]), type: "application/xml" }],
    ["/publications/sitemap.xml", { body: sitemap(["/publications/fixture"]), type: "application/xml" }],
    ["/", { body: html("/", ["child", "/auth/login", "/share/private", "/tryouts?tryout=private", "https://www.agentclash.dev.attacker.test/missing"]), type: "text/html" }],
    ["/child", { body: html("/child", ["leaf"]), type: "text/html" }],
    ["/leaf", { body: html("/leaf", ["/deep"]), type: "text/html" }],
    ["/deep", { body: html("/deep", ["/last"]), type: "text/html" }],
    ["/last", { body: html("/last"), type: "text/html" }],
    ["/publications/fixture", { body: html("/publications/fixture"), type: "text/html" }],
  ]);
  const headers = new Map<string, Record<string, string>>();
  vi.stubGlobal("fetch", vi.fn(async (input: string, init: RequestInit) => {
    expect(init.redirect).toBe("manual");
    expect(new URL(input).origin).toBe(baseUrl);
    const path = new URL(input).pathname;
    const response = responses.get(path);
    return new Response(response?.body ?? "Missing", { status: response ? 200 : 404, headers: { "content-type": response?.type ?? "text/plain", ...headers.get(path) } });
  }));
  return { responses, headers, crawl: () => runSeoCrawl({ baseUrl, keyEntrypoints: ["/child"] }) };
}
afterEach(() => vi.unstubAllGlobals());

describe("production crawler HTTP contracts", () => {
  it("follows relative initial anchors beyond three clicks, checks publications, and ignores image locations/private queries", async () => {
    const { crawl } = fixture();
    const result = await crawl();
    expect(result.failures).toEqual([]);
    expect(result.depth.get("/last")).toBe(4);
    expect(result.publicationPaths).toEqual(["/publications/fixture"]);
  });

  it.each([
    ["orphan leaf", "/child", html("/child"), "not reachable from homepage"],
    ["missing relative link", "/child", html("/child", ["leaf", "missing"]), "/missing: expected direct 200, got 404"],
    ["homepage canonical on child", "/child", html("/child", ["leaf"]).replace(`${origin}/child`, origin), "canonical"],
    ["wrong canonical origin", "/child", html("/child", ["leaf"]).replace(origin, "https://wrong.example"), "canonical"],
    ["canonical query", "/child", html("/child", ["leaf"]).replace(`${origin}/child`, `${origin}/child?preview=1`), "canonical"],
    ["canonical fragment", "/child", html("/child", ["leaf"]).replace(`${origin}/child`, `${origin}/child#part`), "canonical"],
    ["empty sitemap", "/sitemap.xml", sitemap([]), "empty sitemap"],
    ["empty location", "/sitemap.xml", sitemap(["/"]).replace(`${origin}/`, ""), "empty sitemap location"],
    ["malformed sitemap", "/sitemap.xml", "<urlset>", "/sitemap.xml:"],
    ["wrong sitemap origin", "/sitemap.xml", sitemap(["/"]).replace(origin, "https://wrong.example"), "invalid sitemap location"],
    ["conflicting googlebot", "/child", html("/child", ["leaf"]).replace("</head>", '<meta name="robots" content="index"><meta name="googlebot" content="noindex"></head>'), "noindexed"],
    ["conflicting robots", "/child", html("/child", ["leaf"]).replace("</head>", '<meta name="robots" content="index"><meta name="robots" content="none"></head>'), "noindexed"],
  ])("rejects %s", async (_name, path, body, error) => {
    const { responses, crawl } = fixture();
    responses.get(path)!.body = body;
    expect((await crawl()).failures.join("\n")).toContain(error);
  });

  it("rejects noindex headers even when HTML permits indexing", async () => {
    const { headers, crawl } = fixture();
    headers.set("/child", { "x-robots-tag": "googlebot: noindex" });
    expect((await crawl()).failures).toContain("/child: public URL is noindexed");
  });

  it("applies the three-click bound separately from complete reachability", async () => {
    fixture();
    const result = await runSeoCrawl({ baseUrl, keyEntrypoints: ["/last"] });
    expect(result.failures).toEqual(["/last: not reachable from homepage within 3 clicks"]);
  });
});
