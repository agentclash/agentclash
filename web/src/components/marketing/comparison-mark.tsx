export type MarkKind = "yes" | "partial" | "no";

export function ComparisonMark({
  kind,
  highlight = false,
}: {
  kind: MarkKind;
  highlight?: boolean;
}) {
  if (kind === "yes") {
    return (
      <span
        className={
          highlight ? "text-white text-lg font-medium" : "text-white/70 text-base"
        }
        aria-label="supported"
      >
        ●
      </span>
    );
  }
  if (kind === "partial") {
    return (
      <span className="text-white/45 text-base" aria-label="partial">
        ◐
      </span>
    );
  }
  return (
    <span className="text-white/25 text-base" aria-label="not a core capability">
      —
    </span>
  );
}
