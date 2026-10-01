import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import { defaultModels, type Session } from "../../src/lib/vibe";

const outputs = {
  json: '```json\n{\n  "invoice_id": 900719925474099312345,\n  "items": [\n' + Array.from({ length: 35 }, (_, i) => `    {"description":"Item ${i}","amount":42.00}`).join(',\n') + '\n  ]\n}\n```',
  table: '| Item | Amount | Source |\n| --- | --- | --- |\n' + Array.from({ length: 35 }, (_, i) => `| Report row ${i} | 42.00 | Supplied figures, page ${i + 1} |`).join('\n'),
  text: '# Meeting notes\n\n' + Array.from({ length: 35 }, (_, i) => `## Decision ${i}\n\nA recorded decision from the supplied transcript. [Source](https://example.com/notes/${i})`).join('\n\n'),
};

for (const size of [{ width: 320, height: 740 }, { width: 844, height: 390 }]) {
  for (const [format, output] of Object.entries(outputs)) {
    test(`full ${format} output expands and exports without extra runs at ${size.width}×${size.height}`, async ({ page }, info) => {
      await page.setViewportSize(size);
      await page.emulateMedia({ reducedMotion: "reduce" });
      const errors: string[] = [], writes: string[] = [];
      page.on("pageerror", error => errors.push(error.message));
      const state: Session = { id: "output-layout", anonymous: true, revision: 1, document: {
        format_version: 1, evaluation: { id: "output-layout", chat_id: "output-layout", door: "build" }, models: defaultModels, requirements: [],
        artifacts: [{ id: "v1", title: "Supplied material output", kind: "test_suite", accepted: true, agent_prompt: "Use supplied material", blueprint: {}, source_message_id: "request", proposal_message_id: "ready" }],
        messages: [{ id: "ready", role: "assistant", content: "Ready", artifact_id: "v1" }, { id: "output", role: "assistant", content: output, origin: "playground", artifact_id: "v1", operation_id: "trial" }],
        active_artifact_id: "v1", build: { cycle_id: "cycle", phase: "results", clarifications_used: 0, artifact_id: "v1", trial_id: "trial" },
      }, operations: [{ id: "trial", kind: "playground", state: "COMPLETED", billing: "SETTLED", models: defaultModels, results: [], max_cost_nano_usd: 0, actual_cost_nano_usd: 0 }] };
      await page.route("**/v1/vibe/**", async route => {
        const path = new URL(route.request().url()).pathname;
        const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
        if (route.request().method() !== "GET") writes.push(path);
        if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, two_door: true, defaults: defaultModels, models: [] } });
        if (path.endsWith("/sessions") || path.endsWith("/evaluations")) return route.fulfill({ headers, json: [state] });
        if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: "event: connected\ndata: {}\n\n" });
        return route.fulfill({ headers, json: state });
      });
      await page.goto("/vibe-evals?session=output-layout");
      const region = page.getByRole("region", { name: "Your agent’s output" });
      const expand = region.getByRole("button", { name: "Show full output" });
      await expect(expand).toBeVisible({ timeout: 15000 });
      const preview = region.locator(".vibe-output-preview");
      expect((await preview.boundingBox())!.height).toBeLessThanOrEqual(385);
      await expand.click();
      expect((await preview.boundingBox())!.height).toBeGreaterThan(384);
      await expect(region.getByRole("button", { name: "Show less output" })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
      expect(await region.evaluate(element => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
      if (format === "table") {
        const table = region.locator("table");
        const cells = table.locator("tbody tr").first().locator("td");
        // Adjacent columns must remain visually separate, including on phones.
        expect(await cells.first().evaluate(cell => parseFloat(getComputedStyle(cell).paddingRight))).toBeGreaterThanOrEqual(12);
        expect((await table.boundingBox())!.width).toBeLessThanOrEqual((await region.boundingBox())!.width + 1);
      }
      await region.locator("summary").filter({ hasText: "Download output" }).click();
      const downloadPromise = page.waitForEvent("download");
      await region.getByRole("button", { name: format === "json" ? "JSON (.json)" : "Markdown (.md)" }).click();
      const download = await downloadPromise;
      expect(await readFile((await download.path())!, "utf8")).toBe(format === "json" ? output.slice(8, -3) : output);
      await region.getByRole("button", { name: "Show less output" }).click();
      await page.screenshot({ path: info.outputPath(`${format}-${size.width}.png`) });
      expect(writes).toEqual([]);
      expect(errors).toEqual([]);
    });
  }
}
