import { describe, expect, it } from "vitest";
import robots, { AI_CRAWLERS } from "./robots";

describe("robots", () => {
  it("keeps public crawling open and advertises the canonical sitemap", () => {
    const result = robots();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];

    expect(rules[0]).toMatchObject({
      userAgent: "*",
      allow: "/",
      disallow: expect.arrayContaining(["/dashboard", "/workspaces/", "/auth/"]),
    });
    expect(result.sitemap).toBe("https://www.agentclash.dev/sitemap.xml");
  });

  it("explicitly allows every AI crawler (training + answer engines)", () => {
    const result = robots();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];

    for (const userAgent of AI_CRAWLERS) {
      expect(rules).toContainEqual({
        userAgent,
        allow: "/",
        disallow: expect.arrayContaining(["/dashboard"]),
      });
    }
  });

  it("disallows app-shell routes so they do not dilute crawl budget", () => {
    const result = robots();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const star = rules[0];

    expect(star.disallow).toEqual([
      "/dashboard",
      "/workspaces/",
      "/orgs/",
      "/auth/",
      "/invites/",
      "/github/",
      "/share/",
    ]);
  });
});
