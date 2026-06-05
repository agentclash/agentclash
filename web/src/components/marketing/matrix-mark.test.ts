import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { MARK_LABEL, type MarkKind } from "@/lib/comparison-data";
import { MatrixMark } from "./matrix-mark";

// The landing-page capability matrix renders verdicts as decorative glyphs.
// These tests lock in that every verdict ALSO emits real, extractable text
// (the sr-only label) so crawlers / AI answer-engines can read which tool
// supports which capability — the homepage machine-readability fix.
describe("MatrixMark", () => {
  it("emits a real text label for every verdict kind (happy path)", () => {
    const kinds: MarkKind[] = ["yes", "partial", "no"];
    for (const kind of kinds) {
      const html = renderToStaticMarkup(createElement(MatrixMark, { kind }));
      expect(html).toContain(`sr-only">${MARK_LABEL[kind]}<`);
    }
  });

  it("does not invert the mapping: 'no' renders 'No', never 'Yes'", () => {
    const html = renderToStaticMarkup(
      createElement(MatrixMark, { kind: "no" }),
    );
    expect(html).toContain('sr-only">No<');
    expect(html).not.toContain('sr-only">Yes<');
  });

  it("marks the glyph aria-hidden and carries no competing aria-label", () => {
    const html = renderToStaticMarkup(
      createElement(MatrixMark, { kind: "yes", highlight: true }),
    );
    expect(html).toContain('aria-hidden="true"');
    expect(html).not.toContain('aria-label="supported"');
  });
});
