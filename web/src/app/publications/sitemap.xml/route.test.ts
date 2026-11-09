import { beforeEach, describe, expect, it, vi } from "vitest";

const listAllMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/publication-data", () => ({
  listAllPublicPublications: listAllMock,
}));

import { GET } from "./route";

const ID = "11111111-1111-4111-8111-111111111111";

function publication() {
  return {
    publication: {
      canonical_path: `/publications/${ID}`,
      updated_at: "2026-08-01T00:00:00Z",
    },
  };
}

describe("GET /publications/sitemap.xml", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  it("lists published artifacts with canonical URLs and modification dates", async () => {
    listAllMock.mockResolvedValue([publication()]);
    const response = await GET();
    expect(response.status).toBe(200);
    const body = await response.text();
    expect(body).toContain(`https://www.agentclash.dev/publications/${ID}`);
    expect(body).toContain("2026-08-01T00:00:00Z");
  });

  it("returns an empty urlset when nothing is published", async () => {
    listAllMock.mockResolvedValue([]);
    const response = await GET();
    expect(response.status).toBe(200);
    const body = await response.text();
    expect(body).toContain("<urlset");
    expect(body).not.toContain("<url>");
  });

  it("returns 503 when the upstream catalog is unavailable", async () => {
    listAllMock.mockRejectedValue(new Error("upstream down"));
    const response = await GET();
    expect(response.status).toBe(503);
    expect(response.headers.get("Retry-After")).toBe("60");
  });
});
