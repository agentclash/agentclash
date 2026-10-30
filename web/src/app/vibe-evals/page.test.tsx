// @vitest-environment node
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToString } from "react-dom/server";
import { afterEach, expect, it, vi } from "vitest";

const route = vi.hoisted(() => ({ suspended: false }));
vi.mock("next/navigation", () => ({ useSearchParams: () => {
  if (route.suspended) throw new Promise(() => {});
  return new URLSearchParams();
} }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: async () => undefined }),
  useAuth: () => ({ loading: false, user: undefined }),
}));
const { default: VibePage, viewport } = await import("./page");
afterEach(() => { route.suspended = false; });

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

it("server-renders the V1 entry with disabled doors before configuration loads", () => {
  const html = renderToString(<VibePage />);
  expect(html).toContain('class="vibe-entry-title">What would you like to evaluate?</h1>');
  expect(html).toContain("Connecting…");
  const doors = html.match(/<button[^>]*disabled=""[^>]*>[\s\S]*?<\/button>/g) || [];
  expect(doors.filter(button => /Build an agent|Improve an existing agent/.test(button))).toHaveLength(2);
  expect(html).toContain('data-appearance="light"');
  expect(html).not.toContain('class="vibe-route-fallback"');
  expect(html).not.toContain("What should your agent do?");
});

it("paints the fallback with the system canvas only when the client suspends", () => {
  route.suspended = true;
  const html = renderToString(<VibePage />);
  expect(html).toContain('class="vibe-route-fallback"');
  expect(html).not.toContain('class="vibe-entry-title"');
  expect(html).not.toContain("bg-background");
  expect(css).toMatch(new RegExp(`\\.vibe-route-fallback \\{[^}]*background: ${light};`));
  expect(css).toMatch(new RegExp(`@media \\(prefers-color-scheme: dark\\) \\{ \\.vibe-route-fallback \\{ background: ${dark}; \\} \\}`));
});
