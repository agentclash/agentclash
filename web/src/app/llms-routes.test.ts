import { describe, expect, it, vi } from "vitest";
import { GET as getIndex, HEAD as headIndex } from "@/app/llms.txt/route";
import { GET as getFull, HEAD as headFull } from "@/app/llms-full.txt/route";
import { PUBLIC_MACHINE_CONTRACTS } from "@/lib/public-contracts";

describe("LLM discovery routes", () => {
  it("serves GET and HEAD for /llms.txt", async () => {
    vi.spyOn(console, "log").mockImplementation(() => undefined);
    const request = new Request(`https://www.agentclash.dev/llms.txt`);

    const getResponse = getIndex(request);
    expect(getResponse.status).toBe(200);
    expect(getResponse.headers.get("content-type")).toBe(
      "text/plain; charset=utf-8",
    );
    expect((await getResponse.text()).length).toBeGreaterThan(100);

    const headResponse = headIndex(request);
    expect(headResponse.status).toBe(200);
    expect(headResponse.headers.get("content-type")).toBe(
      "text/plain; charset=utf-8",
    );
    expect(await headResponse.text()).toBe("");
  });

  // The full bundle renders every registered page on a cold registry, which
  // exceeds the default 5s timeout on CI runners as the corpus grows. The
  // longer budget covers that one-time build; assertions are unchanged.
  it("serves GET and HEAD for /llms-full.txt", { timeout: 30000 }, async () => {
    vi.spyOn(console, "log").mockImplementation(() => undefined);
    const request = new Request(`https://www.agentclash.dev/llms-full.txt`);

    const getResponse = getFull(request);
    expect(getResponse.status).toBe(200);
    expect(getResponse.headers.get("content-type")).toBe(
      "text/plain; charset=utf-8",
    );
    expect((await getResponse.text()).length).toBeGreaterThan(100);

    const headResponse = headFull(request);
    expect(headResponse.status).toBe(200);
    expect(headResponse.headers.get("content-type")).toBe(
      "text/plain; charset=utf-8",
    );
    expect(await headResponse.text()).toBe("");
  });

  it("advertises only canonical-host HTTPS links", async () => {
    vi.spyOn(console, "log").mockImplementation(() => undefined);
    const response = getIndex(
      new Request("https://www.agentclash.dev/llms.txt"),
    );
    const body = await response.text();
    const links = [...body.matchAll(/\]\((https:\/\/[^)]+)\)/g)].map(
      (match) => new URL(match[1]),
    );

    expect(links.length).toBeGreaterThan(10);
    expect(links.every((url) => url.host === "www.agentclash.dev")).toBe(true);
    expect(body).not.toContain("https://agentclash.dev");
  });

  it("advertises every registered machine contract", async () => {
    vi.spyOn(console, "log").mockImplementation(() => undefined);
    const response = getIndex(
      new Request("https://www.agentclash.dev/llms.txt"),
    );
    const body = await response.text();

    for (const contract of PUBLIC_MACHINE_CONTRACTS) {
      expect(body).toContain(`https://www.agentclash.dev${contract.path}`);
    }
  });
});
