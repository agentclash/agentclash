// @vitest-environment node
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToString } from "react-dom/server";
import { expect, it, vi } from "vitest";

// Dev can server-render the client without suspending. Loading must not depend on it suspending.
vi.mock("./vibe-client", () => ({ VibeClient: () => <main data-test-client="ready" /> }));
const { default: VibePage, viewport } = await import("./page");

const css = readFileSync(path.resolve(__dirname, "../../components/vibe/project-workspace.css"), "utf8");
const canvas = (selector: string) => {
  const block = css.slice(css.indexOf(`${selector} {`));
  return block.slice(0, block.indexOf("}")).match(/--vibe-bg:\s*(#[0-9a-f]{6})/i)?.[1];
};
const light = canvas(".vibe-workspace[data-appearance]");
const dark = canvas('.vibe-workspace[data-appearance="dark"]');

it("colours the browser bar with the Vibe canvas of each system appearance", () => {
  expect(light).toBe("#faf8f5");
  expect(dark).toBe("#12110f");
  expect(viewport.themeColor).toEqual([
    { media: "(prefers-color-scheme: light)", color: light },
    { media: "(prefers-color-scheme: dark)", color: dark },
  ]);
});

it("paints the loading fallback with the same canvas, so a light appearance does not flash dark", () => {
  const html = renderToString(<VibePage />);
  expect(html).toContain('class="vibe-route-fallback"');
  expect(html).not.toContain('data-test-client="ready"');
  expect(html).not.toContain("bg-background");
  expect(css).toMatch(new RegExp(`\\.vibe-route-fallback \\{[^}]*background: ${light};`));
  expect(css).toMatch(new RegExp(`@media \\(prefers-color-scheme: dark\\) \\{ \\.vibe-route-fallback \\{ background: ${dark}; \\} \\}`));
});
