import { describe, expect, it } from "vitest";
import robots, { AI_CRAWLERS } from "./robots";

describe("robots", () => {
  it("keeps public crawling open and advertises the canonical sitemap", () => {
    const result = robots();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];

    expect(rules[0]).toEqual({ userAgent: "*", allow: "/" });
    expect(result.sitemap).toBe("https://www.agentclash.dev/sitemap.xml");
  });

  it("explicitly allows every AI crawler (training + answer engines)", () => {
    const result = robots();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];

    for (const userAgent of AI_CRAWLERS) {
      expect(rules).toContainEqual({ userAgent, allow: "/" });
    }
  });

  it("never disallows any path", () => {
    const result = robots();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];

    for (const rule of rules) {
      expect(rule.disallow).toBeUndefined();
    }
  });
});
