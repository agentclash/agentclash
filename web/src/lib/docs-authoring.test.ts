import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, it } from "vitest";

it("rejects relative page links in authored docs while ignoring assets and code", () => {
  const dir = mkdtempSync(join(tmpdir(), "docs-links-"));
  try {
    const file = join(dir, "sample.mdx");
    writeFileSync(file, "[Guide](../guide)\n\n[ref]: other.mdx#part\n\n<a href=\"another\">Read</a>\n\n`[code](relative)`\n\n![image](asset.png)\n");
    const result = spawnSync(process.execPath, ["scripts/check-doc-links.mjs", dir], { cwd: process.cwd(), encoding: "utf8" });
    expect(result.status).toBe(1);
    expect(result.stderr).toContain("../guide");
    expect(result.stderr).toContain("other.mdx#part");
    expect(result.stderr).toContain("another");
    writeFileSync(file, "[Guide](/docs/guide)\n\n`[code](relative)`\n\n![image](asset.png)\n");
    expect(() => execFileSync(process.execPath, ["scripts/check-doc-links.mjs", dir], { cwd: process.cwd() })).not.toThrow();
  } finally { rmSync(dir, { recursive: true, force: true }); }
});
