import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";

const root = mkdtempSync(path.join(tmpdir(), "content-dates-"));
afterEach(() => rmSync(root, { recursive: true, force: true }));

describe("authored date validation", () => {
  it("accepts missing and valid dates and reports invalid, future, and reversed dates by filename", () => {
    mkdirSync(root, { recursive: true });
    writeFileSync(path.join(root, "valid.mdx"), "---\ndate: 2020-02-29\ndateModified: 2020-03-01\n---\nbody");
    writeFileSync(path.join(root, "unknown.mdx"), "---\ntitle: Example\n---\nbody");
    execFileSync("node", ["scripts/check-content-dates.mjs", root]);
    writeFileSync(path.join(root, "invalid.mdx"), "---\ndate: 2020-02-30\n---\nbody");
    writeFileSync(path.join(root, "future.mdx"), "---\ndateModified: 2999-01-01\n---\nbody");
    writeFileSync(path.join(root, "reversed.mdx"), "---\ndatePublished: 2020-03-01\ndateModified: 2020-02-29\n---\nbody");
    const result = spawnSync("node", ["scripts/check-content-dates.mjs", root], { encoding: "utf8" });
    expect(result.status).toBe(1);
    expect(result.stderr).toContain("invalid.mdx");
    expect(result.stderr).toContain("future.mdx");
    expect(result.stderr).toContain("reversed.mdx");
  });
});
