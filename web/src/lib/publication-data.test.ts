import { beforeEach, describe, expect, it, vi } from "vitest";

const getMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ get: getMock }),
}));

import { listAllPublicPublications } from "./publication-data";

describe("listAllPublicPublications", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("follows cursors across pages until the catalog ends", async () => {
    getMock
      .mockResolvedValueOnce({ items: [{ id: "a" }], next_cursor: "c1" })
      .mockResolvedValueOnce({ items: [{ id: "b" }], next_cursor: null });
    const items = await listAllPublicPublications();
    expect(items).toHaveLength(2);
    expect(getMock).toHaveBeenCalledTimes(2);
  });

  it("stops when a cursor repeats instead of looping forever", async () => {
    getMock.mockResolvedValue({ items: [{ id: "a" }], next_cursor: "c1" });
    const items = await listAllPublicPublications(10);
    expect(items.length).toBeLessThanOrEqual(10);
    expect(getMock.mock.calls.length).toBeLessThanOrEqual(10);
  });

  it("propagates upstream failures to the caller", async () => {
    getMock.mockRejectedValue(new Error("upstream down"));
    await expect(listAllPublicPublications()).rejects.toThrow("upstream down");
  });
});
