import { readFileSync } from "node:fs";
import path from "node:path";
import { expect, it } from "vitest";

const css = readFileSync(path.resolve(__dirname, "project-workspace.css"), "utf8");
function tokens(selector: string) {
  const result: Record<string, string>[] = [];
  for (const block of css.split("}")) {
    if (!block.includes(`${selector} {`)) continue;
    result.push(Object.fromEntries([...block.slice(block.indexOf(`${selector} {`) + selector.length).matchAll(/--vibe-([\w-]+):\s*([^;]+);/g)].map(([, key, value]) => [key, value.trim()])));
  }
  return result;
}
const light = tokens(".vibe-workspace[data-appearance]");
const dark = tokens('.vibe-workspace[data-appearance="dark"]');
const palettes = { light: light[0], dark: { ...light[0], ...dark[0] }, "light more": { ...light[0], ...light[1] }, "dark more": { ...light[0], ...dark[0], ...dark[1] } };
function rgba(value: string): number[] {
  if (value.startsWith("#")) return [1, 3, 5].map(i => parseInt(value.slice(i, i + 2), 16)).concat(1);
  const match = value.match(/^rgba\(([^)]+)\)$/);
  if (!match) throw new Error(`Expected a colour, got ${value}`);
  return match[1].split(",").map(Number);
}
function blend(fg: number[], bg: number[]) { return fg.slice(0, 3).map((v, i) => v * fg[3] + bg[i] * (1 - fg[3])); }
function luminance(c: number[]) { return c.slice(0, 3).map(v => v / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0); }
function contrast(fg: string, bg: string) {
  const back = rgba(bg), a = luminance(blend(rgba(fg), back)), b = luminance(back);
  return (Math.max(a, b) + .05) / (Math.min(a, b) + .05);
}
function boundary(border: string, inside: string, outside: string) {
  const a = luminance(blend(rgba(border), rgba(inside))), b = luminance(rgba(outside));
  return (Math.max(a, b) + .05) / (Math.min(a, b) + .05);
}
for (const [name, palette] of Object.entries(palettes)) {
  it(`${name}: every readable text/status/focus token and control boundary meets contrast on every surface`, () => {
    for (const surface of ["bg", "sidebar", "surface", "surface-2", "surface-3", "elevated"]) {
      for (const token of ["text-1", "text-2", "text-3", "issue", "pass", "caution", "focus", "control-border"]) {
        expect(contrast(palette[token], palette[surface]), `${token} on ${surface}`).toBeGreaterThanOrEqual(token === "control-border" ? 3 : 4.5);
      }
    }
    for (const fill of ["ink", "ink-hover", "ink-pressed"]) expect(contrast(palette["on-ink"], palette[fill]), `on-ink on ${fill}`).toBeGreaterThanOrEqual(4.5);
    expect(contrast(palette["on-issue"], palette.issue)).toBeGreaterThanOrEqual(4.5);
    for (const status of ["issue", "pass"]) expect(contrast(palette[status], palette[`${status}-tint`])).toBeGreaterThanOrEqual(4.5);
  });
  it(`${name}: a control boundary meets 3:1 against its fill and every surrounding surface`, () => {
    for (const inside of ["bg", "sidebar", "surface", "surface-2", "surface-3", "elevated"]) {
      for (const outside of ["bg", "sidebar", "surface", "surface-2", "surface-3", "elevated"]) {
        expect(boundary(palette["control-border"], palette[inside], palette[outside]), `${inside} control on ${outside}`).toBeGreaterThanOrEqual(3);
      }
    }
  });
}
