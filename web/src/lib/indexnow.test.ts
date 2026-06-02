import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Keep the unit isolated: stub the sitemap + docs modules so we don't load the
// real filesystem-reading source readers.
vi.mock("@/app/sitemap", () => ({
  default: () => [
    { url: "https://www.agentclash.dev/" },
    { url: "https://www.agentclash.dev/compare" },
  ],
}));
vi.mock("@/lib/docs", () => ({
  DOCS_ORIGIN: "https://www.agentclash.dev",
}));

import { HOST, KEY, KEY_LOCATION, buildUrlList, submitIndexNow } from "./indexnow";

describe("buildUrlList", () => {
  it("returns the sitemap's absolute URLs", () => {
    expect(buildUrlList()).toEqual([
      "https://www.agentclash.dev/",
      "https://www.agentclash.dev/compare",
    ]);
  });
});

describe("submitIndexNow", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("POSTs host/key/keyLocation/urlList to the IndexNow endpoint (happy)", async () => {
    fetchMock.mockResolvedValue({ status: 200, text: async () => "" });

    const result = await submitIndexNow(["https://www.agentclash.dev/"]);

    expect(result.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("https://api.indexnow.org/IndexNow");
    expect(init.method).toBe("POST");
    const body = JSON.parse(init.body as string);
    expect(body.host).toBe(HOST);
    expect(body.host).toBe("www.agentclash.dev");
    expect(body.key).toBe(KEY);
    expect(body.keyLocation).toBe(KEY_LOCATION);
    expect(body.keyLocation.endsWith(`/${KEY}.txt`)).toBe(true);
    expect(body.urlList).toEqual(["https://www.agentclash.dev/"]);
  });

  it("returns the upstream status without throwing on a bad key (failure)", async () => {
    fetchMock.mockResolvedValue({ status: 403, text: async () => "Forbidden" });

    const result = await submitIndexNow(["https://www.agentclash.dev/"]);

    expect(result.status).toBe(403);
    expect(result.body).toBe("Forbidden");
  });
});

describe("GET /api/indexnow", () => {
  const origCronSecret = process.env.CRON_SECRET;

  afterEach(() => {
    if (origCronSecret === undefined) delete process.env.CRON_SECRET;
    else process.env.CRON_SECRET = origCronSecret;
    vi.unstubAllGlobals();
  });

  it("rejects unauthorized calls with 401 when CRON_SECRET is set (failure)", async () => {
    process.env.CRON_SECRET = "topsecret";
    const { GET } = await import("@/app/api/indexnow/route");

    const res = await GET(
      new Request("https://www.agentclash.dev/api/indexnow"),
    );

    expect(res.status).toBe(401);
  });
});
