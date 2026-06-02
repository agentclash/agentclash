import { afterEach, describe, expect, it } from "vitest";
import { ogImageUrl, webmasterVerification } from "./seo";

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

describe("webmasterVerification", () => {
  const origGsc = process.env.NEXT_PUBLIC_GSC_VERIFICATION;
  const origBing = process.env.NEXT_PUBLIC_BING_VERIFICATION;

  afterEach(() => {
    if (origGsc === undefined) delete process.env.NEXT_PUBLIC_GSC_VERIFICATION;
    else process.env.NEXT_PUBLIC_GSC_VERIFICATION = origGsc;
    if (origBing === undefined) delete process.env.NEXT_PUBLIC_BING_VERIFICATION;
    else process.env.NEXT_PUBLIC_BING_VERIFICATION = origBing;
  });

  it("emits google + bing meta when both tokens are set (happy)", () => {
    process.env.NEXT_PUBLIC_GSC_VERIFICATION = "gsc-token";
    process.env.NEXT_PUBLIC_BING_VERIFICATION = "bing-token";

    expect(webmasterVerification()).toEqual({
      google: "gsc-token",
      other: { "msvalidate.01": "bing-token" },
    });
  });

  it("emits only the token that is set", () => {
    process.env.NEXT_PUBLIC_GSC_VERIFICATION = "gsc-only";
    delete process.env.NEXT_PUBLIC_BING_VERIFICATION;

    expect(webmasterVerification()).toEqual({ google: "gsc-only" });
  });

  it("returns undefined when neither token is set (no meta tag, no crash)", () => {
    delete process.env.NEXT_PUBLIC_GSC_VERIFICATION;
    delete process.env.NEXT_PUBLIC_BING_VERIFICATION;

    expect(webmasterVerification()).toBeUndefined();
  });
});
