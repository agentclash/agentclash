import { describe, expect, it } from "vitest";
import robots from "./robots";

describe("robots", () => {
  it("keeps public crawling open and advertises the canonical sitemap", () => {
    expect(robots()).toEqual({
      rules: [{ userAgent: "*", allow: "/" }],
      sitemap: "https://www.agentclash.dev/sitemap.xml",
    });
  });
});
