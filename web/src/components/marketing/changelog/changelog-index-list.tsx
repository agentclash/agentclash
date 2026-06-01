import Link from "next/link";
import {
  getChangelogPeriodHref,
  getChangelogPeriodPullRequests,
  type ChangelogPeriod,
} from "@/lib/changelog";

export function ChangelogIndexList({ periods }: { periods: ChangelogPeriod[] }) {
  return (
    <ol className="mt-10 divide-y divide-white/[0.08] overflow-hidden rounded-lg border border-white/[0.08] bg-white/[0.02]">
      {periods.map((period, index) => {
        const pullRequestCount = getChangelogPeriodPullRequests(period.id).length;

        return (
          <li key={period.id}>
            <Link
              href={getChangelogPeriodHref(period.id)}
              className="group block px-5 py-5 transition-colors hover:bg-white/[0.03] sm:px-6"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <time
                      dateTime={period.endDate}
                      className="font-[family-name:var(--font-mono)] text-[11px] text-white/45"
                    >
                      {period.label}
                    </time>
                    {index === 0 ? (
                      <span className="rounded border border-white/10 bg-white/[0.06] px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-white/60">
                        Latest
                      </span>
                    ) : null}
                  </div>
                  <h2 className="mt-2 text-[15px] font-semibold leading-snug text-white group-hover:text-white/90">
                    {period.headline}
                  </h2>
                  <p className="mt-2 text-sm leading-relaxed text-white/45">
                    {period.summary}
                  </p>
                  <p className="mt-3 text-[11px] text-white/30">
                    {period.entries.length} updates
                    {pullRequestCount > 0
                      ? ` · ${pullRequestCount} merged PRs`
                      : ""}
                  </p>
                </div>
                <span
                  aria-hidden
                  className="mt-1 shrink-0 text-white/25 transition-transform group-hover:translate-x-0.5 group-hover:text-white/45"
                >
                  →
                </span>
              </div>
            </Link>
          </li>
        );
      })}
    </ol>
  );
}
