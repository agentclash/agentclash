import {
  CHANGELOG_CATEGORY_LABELS,
  type ChangelogCategory,
  type ChangelogPeriod,
} from "@/lib/changelog";

const CATEGORY_STYLES: Record<ChangelogCategory, string> = {
  added: "border-emerald-400/25 bg-emerald-400/10 text-emerald-200/90",
  improved: "border-sky-400/25 bg-sky-400/10 text-sky-200/90",
  fixed: "border-amber-400/25 bg-amber-400/10 text-amber-200/90",
  security: "border-rose-400/25 bg-rose-400/10 text-rose-200/90",
};

function CategoryBadge({ category }: { category: ChangelogCategory }) {
  return (
    <span
      className={`inline-flex shrink-0 items-center rounded-full border px-2 py-0.5 text-[10px] font-[family-name:var(--font-mono)] uppercase tracking-[0.14em] ${CATEGORY_STYLES[category]}`}
    >
      {CHANGELOG_CATEGORY_LABELS[category]}
    </span>
  );
}

export function ChangelogTimeline({ periods }: { periods: ChangelogPeriod[] }) {
  return (
    <div className="relative mt-12 w-full max-w-3xl">
      <div
        aria-hidden
        className="absolute bottom-3 left-[7px] top-3 w-px bg-gradient-to-b from-white/20 via-white/10 to-transparent"
      />

      <ol className="space-y-10">
        {periods.map((period, index) => (
          <li key={period.id} className="relative pl-8">
            <span
              aria-hidden
              className="absolute left-0 top-2 flex size-4 items-center justify-center rounded-full border border-white/20 bg-[#060606] shadow-[0_0_0_4px_rgba(6,6,6,1)]"
            >
              <span className="size-1.5 rounded-full bg-white/70" />
            </span>

            <article
              id={period.id}
              className="scroll-mt-24 rounded-xl border border-white/[0.08] bg-white/[0.03] p-5 sm:p-6"
            >
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <time
                  dateTime={period.endDate}
                  className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-white/45"
                >
                  {period.label}
                </time>
                {index === 0 ? (
                  <span className="rounded-full border border-white/15 bg-white/10 px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em] text-white/70">
                    Latest
                  </span>
                ) : null}
              </div>

              <h2 className="mt-3 font-[family-name:var(--font-display)] text-xl leading-snug tracking-[-0.02em] text-white sm:text-2xl">
                {period.headline}
              </h2>

              <ul className="mt-5 space-y-3">
                {period.entries.map((entry) => (
                  <li
                    key={`${period.id}-${entry.text}`}
                    className="flex items-start gap-3 text-sm leading-relaxed text-white/70"
                  >
                    <CategoryBadge category={entry.category} />
                    <span>{entry.text}</span>
                  </li>
                ))}
              </ul>
            </article>
          </li>
        ))}
      </ol>
    </div>
  );
}
