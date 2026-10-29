import { describe, expect, it, vi } from "vitest";

vi.mock("fs", () => ({
  default: {
    existsSync: () => true,
    readdirSync: () => ["older.mdx", "newer.mdx"],
    readFileSync: (file: string) => file.endsWith("older.mdx")
      ? `---\ntitle: Older\ndate: "2020-01-01"\ndateModified: "2022-01-01"\ndescription: Older\nauthor: Team\n---\nBody`
      : `---\ntitle: Newer\ndate: "2021-01-01"\ndescription: Newer\nauthor: Team\n---\nBody`,
  },
}));

import { getAllPosts } from "./blog";

describe("blog listing dates", () => {
  it("retains edits while ordering by publication date", () => {
    expect(getAllPosts().map(({ slug, date, dateModified }) => ({ slug, date, dateModified }))).toEqual([
      { slug: "newer", date: "2021-01-01", dateModified: undefined },
      { slug: "older", date: "2020-01-01", dateModified: "2022-01-01" },
    ]);
  });
});
