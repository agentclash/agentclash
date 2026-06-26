import { ArrowRight } from "lucide-react";

import { cn } from "@/lib/utils";

const SERIF = "[font-family:var(--font-race-display)]";
const MICRO = "font-mono text-2xs uppercase tracking-[0.22em]";

/*
 * The payoff. Two runs of the user's agent — before their edit and after —
 * side by side, so the score jump (e.g. Rejected 2.0 → Approved 4.5) is the
 * thing they walk away with: "the eval made my agent measurably better."
 */

const VERDICT_WORDS: Record<string, string> = {
  approved: "Approved",
  needs_edits: "Needs edits",
  rejected: "Rejected",
  not_judged: "Not judged",
};

export type DeltaSide = {
  label: string;
  verdict?: string;
  /** 1–5 grade, or null if not graded. */
  grade?: number | null;
};

function Side({ side, dim }: { side: DeltaSide; dim?: boolean }) {
  return (
    <div className="min-w-0 flex-1">
      <p className={cn(MICRO, "text-white/35")}>{side.label}</p>
      <p
        className={cn(
          SERIF,
          "mt-2 font-light leading-none",
          dim ? "text-white/45" : "text-white/95",
        )}
      >
        {side.grade != null ? (
          <>
            {side.grade.toFixed(1)}
            <span className="text-base text-white/30"> / 5</span>
          </>
        ) : (
          <span className="text-2xl text-white/40">—</span>
        )}
      </p>
      {side.verdict ? (
        <p className="mt-1.5 text-xs text-white/45">{VERDICT_WORDS[side.verdict] ?? side.verdict}</p>
      ) : null}
    </div>
  );
}

export function DeltaCard({
  before,
  after,
  changes,
}: {
  before: DeltaSide;
  after: DeltaSide;
  changes?: string[];
}) {
  const delta =
    before.grade != null && after.grade != null ? after.grade - before.grade : null;
  const improved = delta != null && delta > 0.05;
  const regressed = delta != null && delta < -0.05;
  const flipped = before.verdict === "rejected" && after.verdict === "approved";

  const headline = flipped
    ? "Rejected → Approved. Your edit flipped the verdict."
    : improved
      ? "Your agent got measurably better."
      : regressed
        ? "That edit made it worse — keep tuning."
        : "Same score — try a sharper change.";

  return (
    <div
      className={cn(
        "rounded-lg border p-5 sm:p-6",
        improved || flipped ? "border-white/30 bg-white/[0.03]" : "border-white/12 bg-white/[0.015]",
      )}
    >
      <div className="flex items-baseline justify-between gap-3">
        <p className="text-sm leading-6 text-white/85">{headline}</p>
        {delta != null ? (
          <span
            className={cn(
              "font-mono text-xs tabular-nums",
              improved ? "text-white/80" : regressed ? "text-[#e0a085]" : "text-white/35",
            )}
          >
            {delta > 0 ? "+" : ""}
            {delta.toFixed(1)}
          </span>
        ) : null}
      </div>

      <div className="mt-5 flex items-center gap-4">
        <Side side={before} dim />
        <ArrowRight className="size-5 shrink-0 text-white/30" />
        <Side side={after} />
      </div>

      {changes && changes.length > 0 ? (
        <div className="mt-5 border-t border-white/[0.07] pt-4">
          <p className={cn(MICRO, "mb-2 text-white/30")}>What changed</p>
          <ul className="space-y-1">
            {changes.map((change, index) => (
              <li key={index} className="flex items-baseline gap-2 text-xs text-white/55">
                <span className="size-1 shrink-0 translate-y-1 rounded-full bg-white/30" aria-hidden />
                {change}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
