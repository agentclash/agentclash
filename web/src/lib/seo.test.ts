import { describe, expect, it } from "vitest";
import { ogImageUrl } from "./seo";

describe("ogImageUrl", () => {
  it("builds a root-relative /og URL with encoded params", () => {
    const url = ogImageUrl({
      title: "AgentClash vs LangSmith",
      subtitle: "agent eval vs prompt eval",
      kind: "Compare",
    });

    expect(url.startsWith("/og?")).toBe(true);
    const params = new URLSearchParams(url.slice("/og?".length));
    expect(params.get("title")).toBe("AgentClash vs LangSmith");
    expect(params.get("subtitle")).toBe("agent eval vs prompt eval");
    expect(params.get("kind")).toBe("Compare");
  });

  it("omits optional params when not provided", () => {
    const url = ogImageUrl({ title: "AgentClash" });

    const params = new URLSearchParams(url.slice("/og?".length));
    expect(params.get("title")).toBe("AgentClash");
    expect(params.has("subtitle")).toBe(false);
    expect(params.has("kind")).toBe(false);
  });
});
