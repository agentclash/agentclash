import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { ClashMark } from "@/components/marketing/clash-mark";

it("keeps the white mark for marketing and gives each current-tone mark its own gradient", () => {
  const white = renderToStaticMarkup(<ClashMark className="size-6" />);
  expect(white).toContain('id="clash-glass"');
  expect(white).toContain('stroke="#ffffff"');
  expect(white).not.toContain("currentColor");

  const pair = renderToStaticMarkup(<><ClashMark tone="current" /><ClashMark tone="current" /></>);
  const ids = [...pair.matchAll(/<linearGradient id="([^"]+)"/g)].map(match => match[1]);
  expect(ids).toHaveLength(2);
  expect(new Set(ids).size).toBe(2);
  for (const id of ids) expect(pair).toContain(`url(#${id})`);
  expect(pair).not.toContain("#ffffff");
});
