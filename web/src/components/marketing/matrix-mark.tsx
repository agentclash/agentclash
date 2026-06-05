import { MARK_LABEL, type MarkKind } from "@/lib/comparison-data";

// Dot/dash glyph for the landing-page capability matrix, paired with a
// visually-hidden text label ("Yes" / "Partial" / "No"). The glyph is purely
// decorative (aria-hidden); the sr-only label is the real text that screen
// readers announce AND that crawlers / LLM answer-engines extract — so the
// homepage matrix's per-cell support verdicts are machine-readable, not
// glyph-only. (The /compare pages render the same MARK_LABEL as visible
// <table> text.)
export function MatrixMark({
  kind,
  highlight,
}: {
  kind: MarkKind;
  highlight?: boolean;
}) {
  const glyph =
    kind === "yes" ? (
      <span
        aria-hidden
        className={`inline-block size-2 rounded-full ${
          highlight
            ? "bg-white shadow-[0_0_12px_rgba(255,255,255,0.55)]"
            : "bg-white/70"
        }`}
      />
    ) : kind === "partial" ? (
      <span
        aria-hidden
        className="inline-block size-2 rounded-full border border-white/50"
      />
    ) : (
      <span aria-hidden className="block h-px w-3 bg-white/20" />
    );

  return (
    <>
      {glyph}
      <span className="sr-only">{MARK_LABEL[kind]}</span>
    </>
  );
}
