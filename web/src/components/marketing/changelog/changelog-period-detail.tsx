import Link from "next/link";
import {
  CHANGELOG_CATEGORY_LABELS,
  getChangelogPullRequestUrl,
  type ChangelogCategory,
  type ChangelogPeriod,
  type ChangelogPullRequest,
} from "@/lib/changelog";

const CATEGORY_ORDER: ChangelogCategory[] = [
  "added",
  "improved",
  "fixed",
  "security",
];

const CATEGORY_ACCENT: Record<ChangelogCategory, string> = {
  added: "text-emerald-300/90",
  improved: "text-sky-300/90",
  fixed: "text-amber-300/90",
  security: "text-rose-300/90",
};

function groupEntriesByCategory(period: ChangelogPeriod) {
  return CATEGORY_ORDER.map((category) => ({
    category,
    entries: period.entries.filter((entry) => entry.category === category),
  })).filter((group) => group.entries.length > 0);
}

export function ChangelogPeriodDetail({
  period,
  pullRequests,
}: {
  period: ChangelogPeriod;
  pullRequests: ChangelogPullRequest[];
}) {
  const groups = groupEntriesByCategory(period);

  return (
    <article>
      <nav
        aria-label="Breadcrumb"
        className="mb-8 flex flex-wrap items-center gap-2 text-xs text-white/35"
      >
        <Link href="/changelog" className="transition-colors hover:text-white/55">
          Changelog
        </Link>
        <span aria-hidden>/</span>
        <span className="text-white/50">{period.label}</span>
      </nav>

      <header className="border-b border-white/[0.08] pb-8">
        <time
          dateTime={period.endDate}
          className="font-[family-name:var(--font-mono)] text-2xs text-white/45"
        >
          {period.label}
        </time>
        <h1 className="mt-3 text-xl font-semibold leading-snug text-white sm:text-2xl">
          {period.headline}
        </h1>
        <p className="mt-4 text-sm leading-relaxed text-white/55">
          {period.summary}
        </p>
        {period.themes.length > 0 ? (
          <ul className="mt-5 flex flex-wrap gap-2">
            {period.themes.map((theme) => (
              <li
                key={theme}
                className="rounded-md border border-white/[0.08] bg-white/[0.03] px-2.5 py-1 text-2xs text-white/50"
              >
                {theme}
              </li>
            ))}
          </ul>
        ) : null}
      </header>

      <section className="mt-10">
        <h2 className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.16em] text-white/35">
          What shipped
        </h2>
        <div className="mt-5 space-y-8">
          {groups.map(({ category, entries }) => (
            <div key={category}>
              <h3
                className={`text-xs font-semibold uppercase tracking-wide ${CATEGORY_ACCENT[category]}`}
              >
                {CHANGELOG_CATEGORY_LABELS[category]}
              </h3>
              <ul className="mt-3 space-y-3">
                {entries.map((entry) => (
                  <li
                    key={entry.text}
                    className="border-l border-white/10 pl-4 text-sm leading-relaxed text-white/70"
                  >
                    {entry.href ? (
                      <Link
                        href={entry.href}
                        className="underline decoration-white/20 underline-offset-2 transition-colors hover:text-white hover:decoration-white/40"
                      >
                        {entry.text}
                      </Link>
                    ) : (
                      entry.text
                    )}
                    {entry.detail ? (
                      <p className="mt-1 text-sm leading-relaxed text-white/40">
                        {entry.detail}
                      </p>
                    ) : null}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </section>

      {pullRequests.length > 0 ? (
        <section className="mt-12 border-t border-white/[0.08] pt-10">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.16em] text-white/35">
              Merged pull requests
            </h2>
            <span className="text-2xs text-white/30">{pullRequests.length} PRs</span>
          </div>
          <ol className="mt-5 divide-y divide-white/[0.06] rounded-lg border border-white/[0.08]">
            {pullRequests.map((pullRequest) => (
              <li key={pullRequest.number}>
                <a
                  href={getChangelogPullRequestUrl(pullRequest.number)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-start gap-3 px-4 py-3 text-sm transition-colors hover:bg-white/[0.03]"
                >
                  <span className="shrink-0 font-[family-name:var(--font-mono)] text-2xs text-white/35">
                    #{pullRequest.number}
                  </span>
                  <span className="min-w-0 flex-1 leading-relaxed text-white/70 hover:text-white/90">
                    {pullRequest.title}
                  </span>
                </a>
              </li>
            ))}
          </ol>
        </section>
      ) : null}
    </article>
  );
}
